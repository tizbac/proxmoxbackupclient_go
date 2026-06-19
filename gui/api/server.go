package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// jobIDSeq guarantees unique job IDs even when two backups start in the same
// second — time.Now().Unix() alone collides (especially on Windows' coarse
// clock), which would make two jobs share one progress entry.
var jobIDSeq atomic.Uint64

// Server handles HTTP API requests from the GUI
type Server struct {
	addr           string
	app            BackupHandler
	token          string // shared local-auth token required on every route (H-01)
	version        string // build version reported by /status
	mux            *http.ServeMux
	backupProgress map[string]*BackupProgress
	progressMutex  sync.RWMutex
}

// BackupHandler interface that the service must implement
// NOTE: StartBackup will be called in a goroutine (async), so it must be thread-safe
type BackupHandler interface {
	StartBackup(backupType string, backupDirs, driveLetters, excludeList []string, backupID string, useVSS bool, compression string) error
	GetConfigWithHostname() map[string]interface{}
	GetScheduledJobsForAPI() []map[string]interface{}
	SaveScheduledJobFromMap(job map[string]interface{}) error
	UpdateScheduledJobFromMap(job map[string]interface{}) error
	DeleteScheduledJobFromMap(jobID string) error
	PinServerFingerprint(id, fingerprint string) error
}

// NewServer creates a new API server. token is the shared local-auth secret that
// every request must present in the X-Nimbus-Token header (H-01).
func NewServer(addr string, handler BackupHandler, token, version string) *Server {
	if version == "" {
		version = "dev"
	}
	s := &Server{
		addr:           addr,
		app:            handler,
		token:          token,
		version:        version,
		mux:            http.NewServeMux(),
		backupProgress: make(map[string]*BackupProgress),
	}

	s.setupRoutes()
	return s
}

func (s *Server) setupRoutes() {
	s.mux.HandleFunc("/status", s.handleStatus)
	s.mux.HandleFunc("/backup", s.handleBackup)
	s.mux.HandleFunc("/backup/status/", s.handleBackupStatus)
	s.mux.HandleFunc("/jobs", s.handleJobs)
	s.mux.HandleFunc("/jobs/create", s.handleJobCreate)
	s.mux.HandleFunc("/jobs/update", s.handleJobUpdate)
	s.mux.HandleFunc("/jobs/delete/", s.handleJobDelete)
	s.mux.HandleFunc("/pbs/fingerprint", s.handlePinFingerprint)
}

// Start starts the HTTP server
func (s *Server) Start() error {
	log.Printf("Starting API server on %s", s.addr)
	return http.ListenAndServe(s.addr, s.authMiddleware(s.mux))
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	config := s.app.GetConfigWithHostname()

	s.progressMutex.RLock()
	activeJobs := 0
	for _, p := range s.backupProgress {
		if p.Running {
			activeJobs++
		}
	}
	s.progressMutex.RUnlock()

	status := StatusResponse{
		Running:       true,
		Version:       s.version,
		ActiveJobs:    activeJobs,
		Configuration: config,
	}

	s.writeJSON(w, status, http.StatusOK)
}

func (s *Server) handleBackup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req BackupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	// Validate request
	if req.BackupID == "" {
		s.writeError(w, "backup_id is required", http.StatusBadRequest)
		return
	}

	// Reload config before backup (config may have been updated by GUI)
	// This ensures service uses latest config without needing restart
	if reloader, ok := s.app.(interface{ ReloadConfig() }); ok {
		reloader.ReloadConfig()
		log.Printf("[API] Config reloaded before backup")
	}

	// Start backup asynchronously (don't block HTTP request)
	jobID := fmt.Sprintf("backup-%d-%d", time.Now().Unix(), jobIDSeq.Add(1))

	// Initialize progress tracking
	s.progressMutex.Lock()
	s.backupProgress[jobID] = &BackupProgress{
		JobID:     jobID,
		Running:   true,
		Progress:  0,
		Message:   "Starting backup...",
		StartTime: time.Now().Format(time.RFC3339),
	}
	log.Printf("[API] Progress entry created for %s (total entries: %d)", jobID, len(s.backupProgress))
	s.progressMutex.Unlock()

	go func() {
		log.Printf("[API] Starting async backup: %s", jobID)

		// Set up progress callbacks to update the progress map
		handler, ok := s.app.(interface {
			SetProgressCallbacks(jobID string, onProgress func(string, float64, string), onComplete func(string, bool, string))
		})
		if ok {
			log.Printf("[API] SetProgressCallbacks interface found, registering callbacks for %s", jobID)
			handler.SetProgressCallbacks(
				jobID,
				func(jid string, percent float64, message string) {
					s.progressMutex.Lock()
					if progress, exists := s.backupProgress[jid]; exists {
						progress.Progress = percent
						progress.Message = message
						log.Printf("[API] Progress update %s: %.1f%% - %s", jid, percent, message)
					} else {
						log.Printf("[API] WARNING: Progress update for unknown job %s", jid)
					}
					s.progressMutex.Unlock()
				},
				func(jid string, success bool, message string) {
					s.progressMutex.Lock()
					if progress, exists := s.backupProgress[jid]; exists {
						progress.Running = false
						progress.Complete = true
						progress.Success = success
						progress.Message = message
						if !success {
							progress.Error = message
						}
						log.Printf("[API] Backup %s complete: success=%v, %s", jid, success, message)
					} else {
						log.Printf("[API] WARNING: Completion update for unknown job %s", jid)
					}
					s.progressMutex.Unlock()
				},
			)
		} else {
			log.Printf("[API] WARNING: SetProgressCallbacks interface not implemented by handler")
		}

		// Call StartBackup (service App is in standalone mode to execute directly)
		// Default to "fastest" if compression not specified
		compression := req.Compression
		if compression == "" {
			compression = "fastest"
		}

		err := s.app.StartBackup(
			req.BackupType,
			req.BackupDirs,
			req.DriveLetters,
			req.ExcludeList,
			req.BackupID,
			req.UseVSS,
			compression,
		)

		// Update final status if callbacks didn't fire
		s.progressMutex.Lock()
		if progress, exists := s.backupProgress[jobID]; exists && !progress.Complete {
			progress.Running = false
			progress.Complete = true
			if err != nil {
				progress.Success = false
				progress.Error = err.Error()
				progress.Message = fmt.Sprintf("Backup failed: %v", err)
				log.Printf("[API] Backup %s failed: %v", jobID, err)
			} else {
				progress.Success = true
				progress.Progress = 100
				progress.Message = "Backup completed successfully"
				log.Printf("[API] Backup %s completed successfully", jobID)
			}
		}
		s.progressMutex.Unlock()

		// The backup is finished; evict its progress entry after a grace period so
		// the GUI (polling every 3s, stopping on Complete) still reads the final
		// status, while the map can't grow unbounded over the service's lifetime.
		s.scheduleEviction(jobID)
	}()

	// Return immediately with job ID
	resp := BackupResponse{
		Success: true,
		Message: "Backup started successfully (running in background)",
		JobID:   jobID,
	}

	s.writeJSON(w, resp, http.StatusOK)
}

// scheduleEviction removes a finished job's progress entry after a grace period,
// bounding the backupProgress map over the long-lived service process.
func (s *Server) scheduleEviction(jobID string) {
	time.AfterFunc(10*time.Minute, func() {
		s.progressMutex.Lock()
		delete(s.backupProgress, jobID)
		s.progressMutex.Unlock()
	})
}

func (s *Server) handleBackupStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract job ID from URL path: /backup/status/{jobID}
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/backup/status/"), "/")
	if len(pathParts) == 0 || pathParts[0] == "" {
		s.writeError(w, "Job ID required", http.StatusBadRequest)
		return
	}
	jobID := pathParts[0]

	s.progressMutex.RLock()
	progress, exists := s.backupProgress[jobID]
	// Copy the struct under the lock: the backup goroutine mutates the pointee
	// concurrently, so marshaling the pointer outside the lock is a data race.
	var snapshot BackupProgress
	if exists {
		snapshot = *progress
	}
	totalJobs := len(s.backupProgress)
	s.progressMutex.RUnlock()

	log.Printf("[API] Progress query for %s: exists=%v, total_jobs=%d", jobID, exists, totalJobs)

	if !exists {
		log.Printf("[API] Available job IDs: %v", func() []string {
			s.progressMutex.RLock()
			defer s.progressMutex.RUnlock()
			ids := make([]string, 0, len(s.backupProgress))
			for id := range s.backupProgress {
				ids = append(ids, id)
			}
			return ids
		}())
		s.writeError(w, "Job not found", http.StatusNotFound)
		return
	}

	s.writeJSON(w, &snapshot, http.StatusOK)
}

func (s *Server) handleJobs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	jobsData := s.app.GetScheduledJobsForAPI()

	jobs := make([]JobInfo, 0, len(jobsData))
	for _, j := range jobsData {
		job := JobInfo{
			ID:         fmt.Sprintf("%v", j["id"]),
			Name:       fmt.Sprintf("%v", j["name"]),
			BackupType: fmt.Sprintf("%v", j["backup_type"]),
			Schedule:   fmt.Sprintf("%v", j["schedule"]),
			Status:     "idle", // TODO: track actual status
		}
		if lastRun, ok := j["last_run"].(string); ok {
			job.LastRun = lastRun
		}
		if nextRun, ok := j["next_run"].(string); ok {
			job.NextRun = nextRun
		}
		jobs = append(jobs, job)
	}

	resp := JobsResponse{Jobs: jobs}
	s.writeJSON(w, resp, http.StatusOK)
}

func (s *Server) handleJobCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var job map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
		s.writeError(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	if err := s.app.SaveScheduledJobFromMap(job); err != nil {
		s.writeError(w, fmt.Sprintf("Failed to create job: %v", err), http.StatusInternalServerError)
		return
	}

	resp := map[string]interface{}{
		"success": true,
		"message": "Job created successfully",
	}
	s.writeJSON(w, resp, http.StatusOK)
}

func (s *Server) handleJobUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		s.writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var job map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&job); err != nil {
		s.writeError(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	if err := s.app.UpdateScheduledJobFromMap(job); err != nil {
		s.writeError(w, fmt.Sprintf("Failed to update job: %v", err), http.StatusInternalServerError)
		return
	}

	resp := map[string]interface{}{
		"success": true,
		"message": "Job updated successfully",
	}
	s.writeJSON(w, resp, http.StatusOK)
}

func (s *Server) handleJobDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		s.writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract job ID from URL path: /jobs/delete/{jobID}
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/jobs/delete/"), "/")
	if len(pathParts) == 0 || pathParts[0] == "" {
		s.writeError(w, "Job ID required", http.StatusBadRequest)
		return
	}
	jobID := pathParts[0]

	if err := s.app.DeleteScheduledJobFromMap(jobID); err != nil {
		s.writeError(w, fmt.Sprintf("Failed to delete job: %v", err), http.StatusInternalServerError)
		return
	}

	resp := map[string]interface{}{
		"success": true,
		"message": "Job deleted successfully",
	}
	s.writeJSON(w, resp, http.StatusOK)
}

// handlePinFingerprint lets the unprivileged GUI delegate a TOFU certificate pin to
// the privileged service, which is the single writer of config.json.
func (s *Server) handlePinFingerprint(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID          string `json:"id"`
		Fingerprint string `json:"fingerprint"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}
	if req.ID == "" || req.Fingerprint == "" {
		s.writeError(w, "id and fingerprint are required", http.StatusBadRequest)
		return
	}

	if err := s.app.PinServerFingerprint(req.ID, req.Fingerprint); err != nil {
		s.writeError(w, fmt.Sprintf("Failed to pin fingerprint: %v", err), http.StatusInternalServerError)
		return
	}

	resp := map[string]interface{}{
		"success": true,
		"message": "Fingerprint pinned successfully",
	}
	s.writeJSON(w, resp, http.StatusOK)
}

func (s *Server) writeJSON(w http.ResponseWriter, data interface{}, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func (s *Server) writeError(w http.ResponseWriter, message string, status int) {
	errResp := ErrorResponse{
		Error: message,
		Code:  status,
	}
	s.writeJSON(w, errResp, status)
}
