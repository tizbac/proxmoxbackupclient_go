package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// handleMachineBackup handles machine (whole-disk) backup requests via the service API.
func (s *Server) handleMachineBackup(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req BackupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.writeError(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	if req.BackupID == "" {
		s.writeError(w, "backup_id is required", http.StatusBadRequest)
		return
	}
	if len(req.DriveLetters) == 0 {
		s.writeError(w, "at least one drive letter/device is required for machine backup", http.StatusBadRequest)
		return
	}

	if reloader, ok := s.app.(interface{ ReloadConfig() }); ok {
		reloader.ReloadConfig()
		log.Printf("[API] Config reloaded before machine backup")
	}

	jobID := fmt.Sprintf("backup-machine-%d-%d", time.Now().Unix(), jobIDSeq.Add(1))

	s.progressMutex.Lock()
	s.backupProgress[jobID] = &BackupProgress{
		JobID:     jobID,
		Running:   true,
		Progress:  0,
		Message:   "Starting machine backup...",
		StartTime: time.Now().Format(time.RFC3339),
	}
	s.progressMutex.Unlock()

	go func() {
		log.Printf("[API] Starting async machine backup: %s", jobID)

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

		handler, ok := s.app.(interface {
			SetProgressCallbacks(jobID string, onProgress func(string, float64, string), onComplete func(string, bool, string))
		})
		if ok {
			handler.SetProgressCallbacks(
				jobID,
				func(jid string, percent float64, message string) {
					s.progressMutex.Lock()
					if progress, exists := s.backupProgress[jid]; exists {
						// Progress callback receives fraction (0-1), store as percentage (0-100)
						progress.Progress = percent * 100
						progress.Message = message
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
					}
					s.progressMutex.Unlock()
				},
			)
		}

		compression := req.Compression
		if compression == "" {
			compression = "fastest"
		}

		err := s.app.StartMachineBackup(req.BackupType, req.DriveLetters, req.BackupID, req.UseVSS, compression, req.PBSID, req.BackupKind)
		s.progressMutex.Lock()
		if progress, exists := s.backupProgress[jobID]; exists && !progress.Complete {
			progress.Running = false
			progress.Complete = true
			if err != nil {
				progress.Success = false
				progress.Error = err.Error()
				if isCancelled(err) {
					progress.Message = "Machine backup cancelled by user"
					log.Printf("[API] Machine backup %s cancelled by user", jobID)
				} else {
					progress.Message = fmt.Sprintf("Machine backup failed: %v", err)
					log.Printf("[API] Machine backup %s failed: %v", jobID, err)
				}
			} else {
				progress.Success = true
				progress.Progress = 100
				progress.Message = "Machine backup completed successfully"
				log.Printf("[API] Machine backup %s completed successfully", jobID)
			}
		}
		s.progressMutex.Unlock()
		s.scheduleEviction(jobID)
	}()

	s.writeJSON(w, map[string]string{"job_id": jobID}, http.StatusAccepted)
}
