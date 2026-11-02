//go:build !service

package main

import (
	"fmt"
	"time"

	"github.com/tizbac/proxmoxbackupclient_go/gui/api"
)

// Shared by the Windows and Linux service controls (Preferences > Advanced >
// Run as Service).

// setMode changes where backups run and tells the window, so the mode badge
// and the notices that depend on it update when the service appears or goes
// after the app started (upstream's mode:changed).
func (a *App) setMode(m api.ExecutionMode) {
	if a.mode == m {
		return
	}
	a.mode = m
	a.emitEvent("mode:changed")
}

// switchToServiceMode waits for the service's local API and then lets it run
// the schedules: this app stops its own scheduler so nothing runs twice.
func (a *App) switchToServiceMode() {
	detector := api.NewModeDetector(getAPITokenPath())
	for i := 0; i < 40; i++ {
		if detector.DetectMode() == api.ModeService {
			a.setMode(api.ModeService)
			a.StopScheduler()
			writeDebugLog("[Mode] Service is running: schedules and backups now go to the service")
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	writeDebugLog("[Mode] Service started but its API did not answer within 20s; staying standalone for now")
}

// switchToStandalone takes scheduling back after the service has gone.
func (a *App) switchToStandalone() {
	if a.mode == api.ModeStandalone {
		return
	}
	a.setMode(api.ModeStandalone)
	a.RecalculateNextRuns()
	a.StartScheduler()
	writeDebugLog("[Mode] Service stopped or removed: this app runs the schedules again")
}

// serviceBusy asks the service whether it is running a backup.
func (a *App) serviceBusy() (bool, string) {
	st, err := a.apiClient.GetStatus()
	if err != nil || st == nil {
		return false, ""
	}
	if st.ActiveJobs > 0 {
		return true, fmt.Sprintf("%d backup(s)", st.ActiveJobs)
	}
	return false, ""
}
