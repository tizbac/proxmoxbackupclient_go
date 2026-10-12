package api

import (
	"fmt"
	"log"
	"time"
)

// This file publishes backups that did NOT start through an HTTP handler into
// the same progress map the handlers fill. The GUI's Running Jobs tab lists
// GET /backup/jobs, so before this existed a backup executed by the service
// scheduler (scheduled run or "Run now" — see main.runScheduledBackup) ran
// invisibly: it was executing, but never showed up anywhere in the GUI.

// RegisterInlineJob creates the progress entry for such a run and returns its
// job ID, which is the key ReportInlineJobProgress/ReportInlineJobComplete use
// afterwards. title is what the GUI shows before the first progress update.
func (s *Server) RegisterInlineJob(title string) string {
	jobID := fmt.Sprintf("backup-scheduled-%d-%d", time.Now().Unix(), jobIDSeq.Add(1))

	s.progressMutex.Lock()
	s.backupProgress[jobID] = &BackupProgress{
		JobID:     jobID,
		Running:   true,
		Progress:  0,
		Message:   title,
		StartTime: time.Now().Format(time.RFC3339),
	}
	s.progressMutex.Unlock()

	log.Printf("[API] Scheduled/inline job registered: %s (%s)", jobID, title)
	return jobID
}

// ReportInlineJobProgress feeds a live update (completion fraction 0.0-1.0)
// into the entry RegisterInlineJob created. Updates for an entry that is no
// longer running are dropped, exactly like the handler-side callbacks do.
func (s *Server) ReportInlineJobProgress(jobID string, frac float64, message string) {
	s.progressMutex.Lock()
	if p, exists := s.backupProgress[jobID]; exists && p.Running {
		p.Progress = frac * 100
		p.Message = message
	}
	s.progressMutex.Unlock()
}

// ReportInlineJobComplete closes the entry once the run has finished; it is
// the single closing path for inline entries (App.dispatchComplete only
// reaches the callbacks registered by the HTTP handlers). The guard keeps it
// idempotent — a repeat call, or one arriving after the entry was evicted, is
// a no-op — so runScheduledBackup's deferred close can always fire without
// racing the backup's own OnComplete. Without this the entry would sit at
// "running" forever when the backup failed before OnComplete (PBS resolve
// errors, missing config, …). Every entry it closes gets the same eviction
// grace period as handler-started jobs.
func (s *Server) ReportInlineJobComplete(jobID string, success bool, message string) {
	s.progressMutex.Lock()
	p, exists := s.backupProgress[jobID]
	if !exists || p.Complete {
		s.progressMutex.Unlock()
		return
	}
	p.Running = false
	p.Complete = true
	p.Success = success
	p.Message = message
	if success {
		p.Progress = 100
	} else {
		p.Error = message
	}
	s.progressMutex.Unlock()

	log.Printf("[API] Scheduled/inline job %s finished (success=%v): %s", jobID, success, message)
	s.scheduleEviction(jobID)
}
