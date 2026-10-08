// Package api implements the local HTTP API shared by the Proxmox Backup
// Client GUI and its privileged service: the routes and token authentication
// (Server) plus the client the GUI uses to reach the service (Client).
package api

import (
	"context"
	"crypto/subtle"
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
	version        string // build version reported by /status
	mux            *http.ServeMux
	backupProgress map[string]*BackupProgress
	progressMutex  sync.RWMutex

	// token is the current shared local-auth secret (H-01). prevToken is the
	// one it replaced during a rotation and keeps authenticating until
	// prevValidUntil, so requests already in flight are not cut off at the
	// exact rotation instant. tokenGrace is how long that honouring lasts.
	tokenMu        sync.RWMutex
	token          string
	prevToken      string
	prevValidUntil time.Time
	tokenGrace     time.Duration
}

// BackupHandler interface that the service must implement
// NOTE: StartBackup will be called in a goroutine (async), so it must be thread-safe
type BackupHandler interface {
	StartBackup(backupType string, backupDirs, driveLetters, excludeList []string, backupID string, useVSS bool, compression string, pbsID string) error
	GetConfigWithHostname() map[string]interface{}
	GetScheduledJobsForAPI() []map[string]interface{}
	SaveScheduledJobFromMap(job map[string]interface{}) error
	UpdateScheduledJobFromMap(job map[string]interface{}) error
	DeleteScheduledJobFromMap(jobID string) error
	// Manual "run now": start a stored scheduled job immediately, ignoring its
	// schedule. The run is asynchronous; the error only reports a job that
	// cannot be started (unknown id, already running).
	RunScheduledJobForAPI(jobID string) error
	PinServerFingerprint(id, fingerprint string) error
	StartMachineBackup(backupType string, backupDevices []string, backupID string, useVSS bool, compression string, pbsID string, backupKind string) error
	// Config round-trip for the GUI in service mode: the service is the single
	// privileged writer of config.json, so the GUI reads and writes the whole
	// (sanitized) document through these two methods.
	GetFullConfigForAPI() map[string]interface{}
	SaveFullConfigFromAPI(doc map[string]interface{}) error
	// Test a stored (or draft) PBS server without touching the GUI's secrets.
	TestPBSServerForAPI(id string, draft map[string]interface{}) error
	// Mint a short-lived PBS ticket for restore/listing operations.
	MintPBSTicketForAPI(id string) (*PBSTicket, error)
	// Job history as stored by the service's scheduler.
	GetJobHistoryForAPI() ([]map[string]interface{}, error)
	// Cancel a running backup job by ID.
	CancelBackup(jobID string) error
}

// NewServer creates a new API server. token is the shared local-auth secret that
// every request must present in the X-Proxmox-Client-Token header (H-01).
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
		tokenGrace:     DefaultTokenGrace,
	}

	s.setupRoutes()
	return s
}

// SetToken replaces the active token and returns the one it replaced, which
// keeps authenticating until tokenGrace has elapsed. Callers that need file
// and memory to stay in sync (TokenRotator.Rotate) use the returned value to
// roll back when the publication step fails.
func (s *Server) SetToken(token string) string {
	s.tokenMu.Lock()
	defer s.tokenMu.Unlock()
	prev := s.token
	if s.token != "" && s.token != token {
		s.prevToken = s.token
		s.prevValidUntil = time.Now().Add(s.tokenGrace)
	}
	s.token = token
	return prev
}

// SetTokenGrace overrides how long a replaced token stays accepted. It exists
// for the rotator (configured grace) and for tests; call it before SetToken.
func (s *Server) SetTokenGrace(grace time.Duration) {
	s.tokenMu.Lock()
	defer s.tokenMu.Unlock()
	s.tokenGrace = grace
}

// tokenAccepted reports whether got authenticates against this server: the
// current token, or — inside the rotation grace window — the token it replaced.
// An empty current token rejects everything (fail closed: a service whose token
// could not be initialised must not expose its privileged API — H-01).
func (s *Server) tokenAccepted(got string) bool {
	if got == "" {
		return false
	}
	s.tokenMu.RLock()
	cur, prev, until := s.token, s.prevToken, s.prevValidUntil
	s.tokenMu.RUnlock()

	if cur == "" {
		return false
	}
	// Compare both even when the primary matched: a short-circuit here would
	// leak which of the two a guess hit.
	current := subtle.ConstantTimeCompare([]byte(got), []byte(cur)) == 1
	previous := prev != "" && subtle.ConstantTimeCompare([]byte(got), []byte(prev)) == 1
	if current {
		return true
	}
	return previous && time.Now().Before(until)
}

func (s *Server) setupRoutes() {
	s.mux.HandleFunc("/status", s.handleStatus)
	s.mux.HandleFunc("/backup", s.handleBackup)
	s.mux.HandleFunc("/backup/machine", s.handleMachineBackup)
	s.mux.HandleFunc("/backup/status/", s.handleBackupStatus)
	s.mux.HandleFunc("/backup/jobs", s.handleListBackupJobs)
	s.mux.HandleFunc("/backup/cancel/", s.handleCancelBackup)
	s.mux.HandleFunc("/jobs", s.handleJobs)
	s.mux.HandleFunc("/jobs/full", s.handleJobsFull)
	s.mux.HandleFunc("/jobs/create", s.handleJobCreate)
	s.mux.HandleFunc("/jobs/update", s.handleJobUpdate)
	s.mux.HandleFunc("/jobs/delete/", s.handleJobDelete)
	s.mux.HandleFunc("/jobs/run/", s.handleJobRun)
	s.mux.HandleFunc("/pbs/fingerprint", s.handlePinFingerprint)
	s.mux.HandleFunc("/config", s.handleConfig)
	s.mux.HandleFunc("/pbs/test", s.handleTestPBS)
	s.mux.HandleFunc("/pbs/ticket", s.handlePBSTicket)
	s.mux.HandleFunc("/history", s.handleHistory)
}

// Start starts the HTTP server
func (s *Server) Start() error {
	log.Printf("Starting API server on %s", s.addr)
	return http.ListenAndServe(s.addr, s.Handler())
}

// Handler returns the routed, token-authenticated handler Start() serves, so
// the server can be mounted on an arbitrary listener (tests, embedders)
// instead of always binding s.addr.
func (s *Server) Handler() http.Handler {
	return s.authMiddleware(s.mux)
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

		// Create a cancellable context for this backup
		ctx, cancel := context.WithCancel(context.Background())

		// Set the backup context on the App so it can be cancelled
		if appWithContext, ok := s.app.(interface {
			SetBackupContext(context.Context, context.CancelFunc)
			ClearBackupContext()
		}); ok {
			appWithContext.SetBackupContext(ctx, cancel)
			defer appWithContext.ClearBackupContext()
		}
		// Also register by job ID for per-job cancellation
		if appWithReg, ok := s.app.(interface {
			RegisterBackupCancel(jobID string, cancel context.CancelFunc)
		}); ok {
			appWithReg.RegisterBackupCancel(jobID, cancel)
		}

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
						// Progress callback receives fraction (0-1), store as percentage (0-100)
						progress.Progress = percent * 100
						progress.Message = message
						log.Printf("[API] Progress update %s: %.1f%% - %s", jid, progress.Progress, message)
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
			req.PBSID,
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

// handleListBackupJobs returns a list of currently running/completed backup jobs
func (s *Server) handleListBackupJobs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.progressMutex.RLock()
	jobs := make([]*BackupProgress, 0, len(s.backupProgress))
	for _, p := range s.backupProgress {
		// Copy to avoid race
		snapshot := *p
		jobs = append(jobs, &snapshot)
	}
	s.progressMutex.RUnlock()

	s.writeJSON(w, jobs, http.StatusOK)
}

// handleCancelBackup cancels a running backup job by ID
func (s *Server) handleCancelBackup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract job ID from URL path: /backup/cancel/{jobID}
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/backup/cancel/"), "/")
	if len(pathParts) == 0 || pathParts[0] == "" {
		s.writeError(w, "Job ID required", http.StatusBadRequest)
		return
	}
	jobID := pathParts[0]

	s.progressMutex.Lock()
	progress, exists := s.backupProgress[jobID]
	if !exists {
		s.progressMutex.Unlock()
		s.writeError(w, "Job not found", http.StatusNotFound)
		return
	}
	if !progress.Running {
		s.progressMutex.Unlock()
		s.writeError(w, "Job is not running", http.StatusBadRequest)
		return
	}
	s.progressMutex.Unlock()

	// Call the app's CancelBackup method
	if err := s.app.CancelBackup(jobID); err != nil {
		s.writeError(w, fmt.Sprintf("Failed to cancel job: %v", err), http.StatusInternalServerError)
		return
	}

	s.writeJSON(w, map[string]string{"status": "cancelled", "job_id": jobID}, http.StatusOK)
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

// handleJobsFull returns the full ScheduledJob data (as []map) for the GUI's
// service-mode delegation. It mirrors GetScheduledJobsForAPI but returns the
// raw maps so the client can unmarshal into its local ScheduledJob type.
func (s *Server) handleJobsFull(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	jobsData := s.app.GetScheduledJobsForAPI()
	if jobsData == nil {
		jobsData = []map[string]interface{}{}
	}
	s.writeJSON(w, jobsData, http.StatusOK)
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

// handleJobRun starts a stored scheduled job immediately (manual "run now"),
// ignoring its schedule. It returns as soon as the job is claimed: the backup,
// its history entry and the lastRun/nextRun update happen in the background.
func (s *Server) handleJobRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract job ID from URL path: /jobs/run/{jobID}
	pathParts := strings.Split(strings.TrimPrefix(r.URL.Path, "/jobs/run/"), "/")
	if len(pathParts) == 0 || pathParts[0] == "" {
		s.writeError(w, "Job ID required", http.StatusBadRequest)
		return
	}
	jobID := pathParts[0]

	if err := s.app.RunScheduledJobForAPI(jobID); err != nil {
		// 409: the caller's view (job list) is stale or the job is already
		// running — not a server-side failure of the request itself.
		s.writeError(w, fmt.Sprintf("Failed to run job: %v", err), http.StatusConflict)
		return
	}

	resp := map[string]interface{}{
		"success": true,
		"message": "Job started",
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

// handleConfig serves the sanitized full configuration document (GET) and
// accepts whole-document writes (POST). The document is sanitized: secrets are
// replaced by *_set boolean markers, and an empty secret/password on write
// means "keep the existing value" on the service side.
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.writeJSON(w, s.app.GetFullConfigForAPI(), http.StatusOK)
	case http.MethodPost:
		var doc map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&doc); err != nil {
			s.writeError(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
			return
		}
		if err := s.app.SaveFullConfigFromAPI(doc); err != nil {
			s.writeError(w, fmt.Sprintf("Failed to save config: %v", err), http.StatusInternalServerError)
			return
		}
		// Echo back the stored (sanitized) document so the GUI re-syncs,
		// including any server-side adjustments (e.g. secret retention).
		s.writeJSON(w, s.app.GetFullConfigForAPI(), http.StatusOK)
	default:
		s.writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleTestPBS tests connectivity to a stored PBS server, optionally merging
// a non-empty draft first (so the GUI can test unsaved form edits).
func (s *Server) handleTestPBS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID    string                 `json:"id"`
		Draft map[string]interface{} `json:"draft"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	if err := s.app.TestPBSServerForAPI(req.ID, req.Draft); err != nil {
		s.writeError(w, err.Error(), http.StatusBadGateway)
		return
	}
	s.writeJSON(w, map[string]interface{}{"success": true}, http.StatusOK)
}

// handlePBSTicket mints a short-lived PBS session ticket for the GUI's
// restore/listing operations. The response carries the ticket plus only the
// non-sensitive connection parameters (the GUI never sees PBS credentials).
func (s *Server) handlePBSTicket(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	ticket, err := s.app.MintPBSTicketForAPI(req.ID)
	if err != nil {
		s.writeError(w, fmt.Sprintf("Failed to mint ticket: %v", err), http.StatusInternalServerError)
		return
	}
	s.writeJSON(w, ticket, http.StatusOK)
}

// handleHistory returns the job history stored by the service's scheduler.
func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	history, err := s.app.GetJobHistoryForAPI()
	if err != nil {
		s.writeError(w, fmt.Sprintf("Failed to read history: %v", err), http.StatusInternalServerError)
		return
	}
	if history == nil {
		history = []map[string]interface{}{}
	}
	s.writeJSON(w, history, http.StatusOK)
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
