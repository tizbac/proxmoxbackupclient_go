package main

import (
	"context"
	"fmt"
)

// This file holds the backup control surface shared by the GUI build (main.go)
// and the privileged service build (app_service_stubs.go). Keeping it
// build-tag-free matters: the API server discovers these methods by interface
// assertion at runtime, so if (say) SetBackupContext only existed in the GUI
// build, the service would silently fail to hand its cancellable context to the
// backup — progress would stay at 0% and Stop would do nothing.

// SetProgressCallbacks registers the API server's live-progress sinks for one
// job. The onProgress callback receives the completion fraction normalized to
// 0.0-1.0 (the server scales it to 0-100 for its progress map).
func (a *App) SetProgressCallbacks(jobID string, onProgress func(string, float64, string), onComplete func(string, bool, string)) {
	writeDebugLog(fmt.Sprintf("[SetProgressCallbacks] Registered callbacks for jobID: %s", jobID))
	a.callbacksMutex.Lock()
	a.callbacksMap[jobID] = &progressCallbacks{
		onProgress: onProgress,
		onComplete: onComplete,
	}
	a.callbacksMutex.Unlock()
}

// SetBackupContext sets the context and cancel function for the currently
// running backup. Called by the API handler so CancelBackup can stop the backup.
func (a *App) SetBackupContext(ctx context.Context, cancel context.CancelFunc) {
	a.backupCtxMu.Lock()
	a.backupCtx = ctx
	a.backupCancel = cancel
	a.backupCtxMu.Unlock()

	// Also register in cancelFuncs map for CancelBackup lookup.
	a.cancelFuncsMu.Lock()
	a.cancelFuncs["current"] = cancel
	a.cancelFuncsMu.Unlock()
}

// ClearBackupContext clears the backup context after the backup completes.
func (a *App) ClearBackupContext() {
	a.backupCtxMu.Lock()
	a.backupCtx = nil
	a.backupCancel = nil
	a.backupCtxMu.Unlock()

	a.cancelFuncsMu.Lock()
	delete(a.cancelFuncs, "current")
	a.cancelFuncsMu.Unlock()
}

// GetBackupContext returns the current backup context, or nil if none is set.
func (a *App) GetBackupContext() context.Context {
	a.backupCtxMu.RLock()
	defer a.backupCtxMu.RUnlock()
	return a.backupCtx
}

// RegisterBackupCancel registers a cancel function for a specific job ID.
func (a *App) RegisterBackupCancel(jobID string, cancel context.CancelFunc) {
	a.cancelFuncsMu.Lock()
	a.cancelFuncs[jobID] = cancel
	a.cancelFuncsMu.Unlock()
}

// dispatchProgress forwards a live-progress update (fraction, 0.0-1.0) to every
// job callback registered via SetProgressCallbacks. It reports whether at least
// one callback consumed the update, so the GUI build can fall back to emitting
// a Wails event in standalone mode.
func (a *App) dispatchProgress(percent float64, message string) bool {
	a.callbacksMutex.RLock()
	defer a.callbacksMutex.RUnlock()

	if len(a.callbacksMap) == 0 {
		return false
	}
	for jobID, callbacks := range a.callbacksMap {
		if callbacks != nil && callbacks.onProgress != nil {
			callbacks.onProgress(jobID, percent, message)
		}
	}
	return true
}

// dispatchComplete forwards the terminal update to every registered callback and
// then removes them, so a finished job can't keep receiving (or leak) updates.
// It reports whether at least one callback consumed the update.
func (a *App) dispatchComplete(success bool, message string) bool {
	a.callbacksMutex.RLock()
	if len(a.callbacksMap) == 0 {
		a.callbacksMutex.RUnlock()
		return false
	}
	jobIDs := make([]string, 0, len(a.callbacksMap))
	for jobID, callbacks := range a.callbacksMap {
		if callbacks != nil && callbacks.onComplete != nil {
			callbacks.onComplete(jobID, success, message)
		}
		jobIDs = append(jobIDs, jobID)
	}
	a.callbacksMutex.RUnlock()

	a.callbacksMutex.Lock()
	for _, jobID := range jobIDs {
		delete(a.callbacksMap, jobID)
	}
	a.callbacksMutex.Unlock()
	return true
}

// setDelegatedJobID remembers the service-side job the GUI is currently polling,
// so a Stop click without an explicit job ID (the GUI's Stop button) can still
// cancel the right run.
func (a *App) setDelegatedJobID(jobID string) {
	a.delegatedJobIDMu.Lock()
	a.delegatedJobID = jobID
	a.delegatedJobIDMu.Unlock()
}

func (a *App) currentDelegatedJobID() string {
	a.delegatedJobIDMu.Lock()
	defer a.delegatedJobIDMu.Unlock()
	return a.delegatedJobID
}

func (a *App) clearDelegatedJobID() {
	a.delegatedJobIDMu.Lock()
	a.delegatedJobID = ""
	a.delegatedJobIDMu.Unlock()
}

// CancelBackup cancels a running backup job by ID. Returns an error if no
// running backup could be found.
func (a *App) CancelBackup(jobID string) error {
	// In service mode the GUI is not the process running the backup: forward the
	// request to the service, otherwise Stop cancels a local run that doesn't
	// exist and the real backup keeps going.
	if a.isDelegatedToService() {
		target := jobID
		if target == "" {
			target = a.currentDelegatedJobID()
		}
		// Last resort: ask the service which job is running.
		if target == "" && a.apiClient != nil {
			if jobs, err := a.apiClient.ListBackupJobs(); err == nil {
				for _, job := range jobs {
					if job != nil && job.Running {
						target = job.JobID
						break
					}
				}
			}
		}
		if target != "" {
			if err := a.apiClient.CancelBackup(target); err != nil {
				return fmt.Errorf("failed to cancel service backup %s: %w", target, err)
			}
			a.clearDelegatedJobID()
			writeDebugLog(fmt.Sprintf("[CancelBackup] Service cancellation requested for jobID: %s", target))
			return nil
		}
		return fmt.Errorf("no running service backup found to cancel")
	}

	// Prefer the per-job registration, then the "current" one the API handler
	// made, then the context attached to the App, and finally the run's own
	// shared token (scheduler / standalone runs register only there).
	a.cancelFuncsMu.Lock()
	cancel := a.cancelFuncs[jobID]
	if cancel != nil {
		delete(a.cancelFuncs, jobID)
	} else {
		cancel = a.cancelFuncs["current"]
		delete(a.cancelFuncs, "current")
	}
	a.cancelFuncsMu.Unlock()

	if cancel == nil {
		a.backupCtxMu.RLock()
		cancel = a.backupCancel
		a.backupCtxMu.RUnlock()
	}
	if cancel == nil {
		currentBackupCancelMutex.Lock()
		cancel = currentBackupCancel
		currentBackupCancelMutex.Unlock()
	}
	if cancel == nil {
		writeDebugLog(fmt.Sprintf("[CancelBackup] No running backup found for jobID: %s", jobID))
		return fmt.Errorf("no running backup found for jobID: %s", jobID)
	}

	writeDebugLog(fmt.Sprintf("[CancelBackup] Cancellation requested for jobID: %s", jobID))
	cancel()

	a.backupCtxMu.Lock()
	a.backupCtx = nil
	a.backupCancel = nil
	a.backupCtxMu.Unlock()
	return nil
}