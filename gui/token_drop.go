//go:build !service

package main

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/tizbac/proxmoxbackupclient_go/gui/api"
)

// The app lets go of the service's token while nobody uses its window.
//
// While the app holds the token (service_connect.go), any program running as
// the same user could read it from the app's memory and use the service,
// which runs as SYSTEM or root: restore into any folder, for one. So the app
// holds it only while its window is in use. When the window goes to the tray
// (Minimize to Tray, closing it to the tray, or starting there with
// --minimized), a grace period starts; once it is over with the window still
// hidden, the app drops the token: it clears it from memory and is no longer
// connected, as after the service replaced it. Shown again within the grace
// period, the app keeps it.
//
// Nothing in the background needs the token: the service runs the Backup
// Sets, and while the app holds none, the tray reads the service's public
// status (GET /status/public: running and waiting backups, whether a run
// failed), which needs no token. When the window is shown again, the app asks
// once on its own, as at its first showing (the window's focus or
// window:shown runs ConnectService), and Connect asks at any time. Pause in
// the tray, which needs the token, opens the window and asks first.
//
// An elevated app (or one run as root) reads the token file itself; it drops
// the token too, and stops reading the file until it connects again
// (api.SetFetchedTokenOnly), so it behaves the same. Connecting again needs
// no prompt for it.

// tokenDropGrace is how long the window may stay in the tray before the app
// drops the token (a var for the tests).
var tokenDropGrace = 15 * time.Minute

// tokenDropRetry is when the app looks again if a connect was under way (its
// prompt open) when the grace period ended.
var tokenDropRetry = time.Minute

// mainWindowVisible reports whether the app's window is on screen. Windows
// sets it (window_visible_windows.go): a second launch shows the window
// without this app being told. Elsewhere there is no tray to hide it in.
var mainWindowVisible = func(*App) bool { return false }

// latestPublicStatus is the service's public status as the app last read it
// while it held no token (watchServiceRuns); nil otherwise, or when the
// service gave none (not running, or older than the route).
var latestPublicStatus atomic.Pointer[api.PublicStatus]

// windowHiddenToTray starts the grace period: the window went to the tray. A
// window already hidden keeps the grace period it has.
func (a *App) windowHiddenToTray() {
	h := &a.trayHide
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.hidden && h.timer != nil {
		return
	}
	h.hidden = true
	h.gen++
	gen, grace := h.gen, tokenDropGrace
	h.timer = time.AfterFunc(grace, func() { a.tokenGraceOver(gen, grace) })
}

// windowShownAgain ends the grace period: the window is in front of the user
// again (shown from the tray, or the page asked to connect).
func (a *App) windowShownAgain() {
	h := &a.trayHide
	h.mu.Lock()
	defer h.mu.Unlock()
	h.hidden = false
	h.gen++
	if h.timer != nil {
		h.timer.Stop()
		h.timer = nil
	}
}

// tokenGraceOver runs when the grace period gen, grace long, ends.
func (a *App) tokenGraceOver(gen uint64, grace time.Duration) {
	h := &a.trayHide
	h.mu.Lock()
	current := h.hidden && h.gen == gen
	h.mu.Unlock()
	if !current {
		return
	}
	if mainWindowVisible(a) {
		// Shown by something the app was not told of (a second launch
		// brings the window to the front): it is in use.
		a.windowShownAgain()
		return
	}
	if !a.dropServiceToken(grace) {
		return
	}
	// A connect is under way: its prompt is open. Look again later.
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.hidden && h.gen == gen {
		h.timer = time.AfterFunc(tokenDropRetry, func() { a.tokenGraceOver(gen, grace) })
	}
}

// dropServiceToken lets go of the service's token, the window having been in
// the tray for hidden: the app is in service mode but not connected, as after
// the service replaced the token, and asks once on its own when the window is
// shown again. busy: a connect was under way (tokenFetchMu held), so nothing
// was dropped.
func (a *App) dropServiceToken(hidden time.Duration) (busy bool) {
	if a.isServiceProcess || a.mode != api.ModeService || a.ownSettings.Load() {
		return false
	}
	if !a.tokenFetchMu.TryLock() {
		return true
	}
	defer a.tokenFetchMu.Unlock()
	// An elevated app would read the token file again on its next request.
	api.SetFetchedTokenOnly(true)
	api.SetTokenOverride("")
	if !a.svcConnected.Swap(false) {
		return false
	}
	a.autoConnectTried.Store(false)
	writeDebugLog(fmt.Sprintf("[Service] the window has been in the tray for %s: the app let go of the service's token; it asks again when the window is shown", hidden))
	a.emitEvent("service:disconnected")
	return false
}

// holdsNoToken reports whether the app works on the service's settings but
// holds no token the service takes: not connected yet, declined, dropped
// after a while in the tray, or replaced by the service.
func (a *App) holdsNoToken() bool {
	return !a.isServiceProcess && a.mode == api.ModeService && a.apiClient != nil &&
		!a.ownSettings.Load() && !a.svcConnected.Load()
}

// notePublicStatus keeps st, the service's public status (nil: none), and
// shows it in the tray. It reports whether the tray's text changed.
func (a *App) notePublicStatus(st *api.PublicStatus) bool {
	latestPublicStatus.Store(st)
	return setTrayPublicText(publicTrayText(st))
}

// publicTrayText is what the tray says from the service's public status: how
// many backups run and wait, that one failed, or when the last one ended.
// "" without a status (the tray then shows its usual text).
func publicTrayText(st *api.PublicStatus) string {
	if st == nil {
		return ""
	}
	busy := ""
	switch {
	case st.BackupsRunning > 0:
		busy = countText(st.BackupsRunning, "backup running", "backups running")
		if st.BackupsWaiting > 0 {
			busy += fmt.Sprintf(", %d waiting", st.BackupsWaiting)
		}
	case st.BackupsWaiting > 0:
		busy = countText(st.BackupsWaiting, "backup waiting", "backups waiting")
	}
	if busy != "" {
		if st.LastRunFailed {
			busy += "; last backup failed"
		}
		return busy
	}
	if st.LastRunFailed {
		return "Last backup failed: open the app"
	}
	if t, err := time.Parse(time.RFC3339, st.LastRunEnded); err == nil {
		return "Last backup ended " + t.Local().Format("02 Jan 15:04")
	}
	return "Scheduled backups active"
}

func countText(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return fmt.Sprintf("%d %s", n, many)
}

// pauseViaWindow is the tray's Pause while the app holds no token: Pause
// needs it, so the window opens and the app asks for it (one prompt), then
// pauses the service's backups. Declined, nothing is paused and the window
// shows Connect.
func (a *App) pauseViaWindow() {
	writeDebugLog("Tray: Pause clicked while the app holds no token: opening the window to connect first")
	// The window's own ask when it is shown would be a second prompt.
	a.autoConnectTried.Store(true)
	a.ShowFromTray()
	if err := a.ConnectService(true); err != nil {
		writeDebugLog(fmt.Sprintf("Tray: not connected (%v): the backup was not paused", err))
		return
	}
	if _, err := a.PauseBackup(); err != nil {
		writeDebugLog(fmt.Sprintf("Tray: pause after connecting failed: %v", err))
	}
}
