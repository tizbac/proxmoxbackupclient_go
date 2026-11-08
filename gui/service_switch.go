//go:build !service

package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/salvanza/salvanza_client/gui/api"
)

// Shared by the Windows and Linux service controls (Preferences > Advanced >
// Run as Service).

// switchToServiceMode waits for the service's local API and then lets it run
// the schedules: this app stops its own scheduler so nothing runs twice.
func (a *App) switchToServiceMode() {
	for i := 0; i < 40; i++ {
		if code, _ := a.apiClient.ProbeStatus(); code == http.StatusOK || code == http.StatusUnauthorized {
			a.StopScheduler()
			a.enterServiceMode(code == http.StatusOK)
			a.emitEvent("mode:changed")
			writeDebugLog("[Mode] Service is running: schedules and backups now go to the service")
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	writeDebugLog("[Mode] Service started but its API did not answer within 20s; staying standalone for now")
}

// recheckService switches this app to the service when one has appeared
// since it started (a backup is about to start): the whole switch, so its
// own scheduler stops and its settings become the service's. Not for a user
// who chose their own settings (UseOwnSettings).
func (a *App) recheckService() {
	if a.isServiceProcess || a.mode != api.ModeStandalone || a.ownSettings.Load() {
		return
	}
	if code, _ := a.apiClient.ProbeStatus(); code == http.StatusOK || code == http.StatusUnauthorized {
		writeDebugLog("[Mode Detection] Service now available, switching to Service mode")
		a.switchToServiceMode()
	}
}

// switchToStandalone takes scheduling back after the service has gone.
func (a *App) switchToStandalone() {
	if a.mode == api.ModeStandalone {
		return
	}
	a.leaveServiceMode()
	a.emitEvent("mode:changed")
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
