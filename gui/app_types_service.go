//go:build service

package main

// The helpers below report a service run's progress to its API entry. Only
// the service's backup and restore entry points (app_service_stubs.go) call
// them, so they are built with the service only; in app_types.go they were
// dead code in the GUI build.

// runIDFor returns the API entry of the service run under key ("" if none).
func (a *App) runIDFor(key string) string {
	if key == "" {
		return ""
	}
	if v, ok := a.pendingPostActions.Load(key); ok {
		return v.(*pendingPostActionsEntry).runID
	}
	return ""
}

// reportRun sends a service run's progress (fraction 0-1) to its API entry.
func (a *App) reportRun(runID string, fraction float64, message string) {
	if runID != "" && a.runReporter != nil {
		a.runReporter.ReportProgress(runID, fraction*100, message)
	}
}

// reportRunStats sends a service run's live statistics to its API entry.
func (a *App) reportRunStats(runID string, stats *BackupProgressStats) {
	if runID != "" && a.runReporter != nil && stats != nil {
		a.runReporter.ReportStats(runID, stats.event())
	}
}

// reportRunFile sends the file a service run is reading to its API entry.
func (a *App) reportRunFile(runID string, path string) {
	if runID != "" && a.runReporter != nil {
		a.runReporter.ReportFile(runID, path)
	}
}
