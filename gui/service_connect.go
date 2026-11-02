//go:build !service

package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/tizbac/proxmoxbackupclient_go/gui/api"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// The app and the background service (upstream's strict model).
//
// The service keeps the computer's settings, which only it (SYSTEM or root)
// and elevated administrators may read. The app reaches them through the
// service's API with the service's token, which it gets from an elevated
// copy of itself (UAC on Windows, pkexec on Linux; fetchServiceToken) and
// keeps in memory. An elevated app, or one run as root, reads the token file
// directly and connects without a prompt.
//
// Unlike upstream, the app does not ask at start: it starts in the tray at
// every sign-in (the machine-wide autostart), and a prompt nobody asked for
// would greet every user. It asks once on its own when its window first comes
// to the front (focus, or opened from the tray), and whenever the user clicks
// Connect; never for a request made in the background. Until connected, or
// after a decline, it is in service mode but not connected: calls to the
// service fail with api.ErrUnauthorized and the window shows Connect, and
// "Use my own settings" for a user who cannot answer the prompt
// (UseOwnSettings).

// errNoPromptNow is the hook's answer: the request keeps its 401 and nobody
// is asked.
var errNoPromptNow = errors.New("not connected to the background service yet")

// A connection the app refuses because something other than the background
// service answers on its port (gui/api/peer.go) goes to the debug log.
func init() { api.SetPeerLog(writeDebugLog) }

// resolveMode decides how this app runs: with the service (it answers,
// 200 or 401) or on its own. A 200 means this app could read the token
// itself (elevated, or root): connected at once.
func (a *App) resolveMode() {
	code, _ := a.apiClient.ProbeStatus()
	switch {
	case code == http.StatusUnauthorized && ownSettingsChosen():
		// This user works on their own settings (UseOwnSettings); an
		// elevated app (200) always takes the computer's.
		a.mode = api.ModeStandalone
		a.ownSettings.Store(true)
	case code == http.StatusOK || code == http.StatusUnauthorized:
		a.enterServiceMode(code == http.StatusOK)
	case serviceInstalled():
		// Installed but not answering yet (the app started first at
		// sign-in, or the service is restarting): still the service's
		// settings, not connected; the app must not run the computer's
		// Backup Sets itself. A user who chose their own keeps them.
		if ownSettingsChosen() {
			a.mode = api.ModeStandalone
			a.ownSettings.Store(true)
		} else {
			a.enterServiceMode(false)
		}
	default:
		a.mode = api.ModeStandalone
	}
	writeDebugLog(fmt.Sprintf("Execution mode: %s (status %d, connected %v)", a.mode, code, a.svcConnected.Load()))
}

// enterServiceMode makes this app work on the service's settings: at start,
// or when Run as Service has started the service. connected: the app could
// read the token itself (elevated, or root). The app's Message Log entries
// go to the service's log from now on.
func (a *App) enterServiceMode(connected bool) {
	api.SetUnauthorizedHook(a.serviceTokenHook)
	messageSink.Store(func(e MessageLogEntry) error { return a.apiClient.AppendMessage(e) })
	if a.apiClient != nil {
		a.apiClient.SetAnswerHook(a.onServiceAnswer)
		if answers, known := a.apiClient.Answering(); known {
			setTrayServiceDown(!answers)
		}
	}
	a.svcConnected.Store(connected)
	a.setModeQuiet(api.ModeService)
	a.serviceSettings.Store(true)
	// Until connected the app shows no settings: those in the user's own
	// folder (from use without the service) are not the computer's.
	a.config = &Config{PBSServers: map[string]*PBSServer{}}
	applySpeedLimits(a.config)
	if connected {
		if err := a.hydrateFromService(); err != nil {
			writeDebugLog(fmt.Sprintf("[Service] reading the service's settings: %v", err))
		}
	}
}

// leaveServiceMode is enterServiceMode undone, when the service has gone:
// the app works on its own settings again.
func (a *App) leaveServiceMode() {
	api.SetUnauthorizedHook(nil)
	api.SetTokenOverride("")
	if a.apiClient != nil {
		a.apiClient.SetAnswerHook(nil)
	}
	setTrayServiceDown(false)
	messageSink.Store((func(MessageLogEntry) error)(nil))
	a.svcConnected.Store(false)
	// The user's own settings first, then the flag: a setter running in
	// between must never save the service's copy (its stand-ins for the
	// secrets) as the user's settings.
	a.config = LoadConfig()
	applySpeedLimits(a.config)
	a.serviceSettings.Store(false)
	a.setModeQuiet(api.ModeStandalone)
}

// setModeQuiet sets the mode. Its callers tell the window themselves
// (mode:changed) once the switch is complete.
func (a *App) setModeQuiet(m api.ExecutionMode) { a.mode = m }

// serviceTokenHook runs when the service refuses a request: the app has no
// token yet, or the service replaced it (at its start, and every 24 hours).
// It never asks for elevation: a request nobody is waiting on (the run list,
// the tray) must not raise a prompt, and one the user made is answered "not
// connected", with the Connect banner. Refused after being connected: back to
// "not connected", and the window asks again when it next comes to the front.
func (a *App) serviceTokenHook(sent string) (string, error) {
	if now := api.GetTokenOverride(); now != "" && now != sent {
		return now, nil // ConnectService fetched one while this one was on its way
	}
	if a.svcConnected.Swap(false) {
		go a.onServiceDisconnected()
	}
	return "", errNoPromptNow
}

// ConnectService connects this app to the background service, asking for
// elevation if it must. userAsked is the Connect button; without it (the
// window came to the front) the app asks only once on its own until the
// service replaces the token. Returns nil when connected, or why not.
func (a *App) ConnectService(userAsked bool) error {
	return a.connect(userAsked, nil)
}

// connect is ConnectService; once connected, and before the window is told,
// it runs then (nil for nothing), whose error it returns.
func (a *App) connect(userAsked bool, then func() error) error {
	if a.ownSettings.Load() {
		// Working on the user's own settings: only Connect leaves them.
		if !userAsked {
			return nil
		}
		a.StopScheduler()
		_ = os.Remove(ownSettingsMarker())
		a.ownSettings.Store(false)
		a.enterServiceMode(false)
		a.emitEvent("mode:changed")
	}
	if a.mode != api.ModeService || a.svcConnected.Load() {
		return nil
	}
	if !userAsked && a.autoConnectTried.Swap(true) {
		return api.ErrUnauthorized
	}
	// One fetch at a time. The token is stored before the lock is released,
	// so a second attempt waiting here finds it instead of asking again.
	a.tokenFetchMu.Lock()
	defer a.tokenFetchMu.Unlock()
	if a.svcConnected.Load() {
		return nil
	}
	// A stopped service has no token to give: say so rather than ask for
	// elevation in vain (the window shows Start the service instead).
	if code, _ := a.apiClient.ProbeStatus(); code == 0 {
		return errServiceNotRunning
	}
	tok, err := fetchServiceToken()
	if err != nil {
		writeDebugLog(fmt.Sprintf("[Service] token not fetched: %v", err))
		return err
	}
	return a.useServiceToken(tok, then)
}

// errServiceNotRunning: the background service is installed but nothing
// answers on its port (or something other than the service does).
var errServiceNotRunning = errors.New("the background service is not running")

// useServiceToken connects this app with tok, the service's token (fetched,
// or handed back by the copy that started the service), then runs then (nil
// for nothing) and tells the window. tokenFetchMu must be held.
func (a *App) useServiceToken(tok string, then func() error) error {
	api.SetTokenOverride(tok)
	if code, _ := a.apiClient.ProbeStatus(); code != http.StatusOK {
		api.SetTokenOverride("")
		if code == 0 {
			return errServiceNotRunning
		}
		return fmt.Errorf("the background service did not take the token (status %d)", code)
	}
	a.svcConnected.Store(true)
	var thenErr error
	if then != nil {
		thenErr = then()
	}
	go a.onServiceConnected()
	return thenErr
}

// UseOwnSettings is the banner's "Use my own settings": for a user who cannot
// (or does not want to) answer the administrator prompt, the app works on
// their own settings in ~/.proxmox-backup-guardian instead, with their own
// PBS servers and Backup Sets, which run while the app runs (as upstream's
// app does after a declined prompt). The computer's Backup Sets go on in the
// service. The choice is kept for this user; Connect goes back.
func (a *App) UseOwnSettings() error {
	if a.mode != api.ModeService || a.svcConnected.Load() {
		return fmt.Errorf("the app is not waiting to connect to the background service")
	}
	if err := os.WriteFile(ownSettingsMarker(), []byte("own settings\n"), 0o600); err != nil {
		writeDebugLog(fmt.Sprintf("[Service] could not remember the own-settings choice: %v", err))
	}
	a.leaveServiceMode()
	a.ownSettings.Store(true)
	a.RecalculateNextRuns()
	a.StartScheduler()
	writeDebugLog("[Service] working on the user's own settings")
	if a.ctx != nil && !a.headless {
		runtime.EventsEmit(a.ctx, "settings:switched")
	}
	return nil
}

// ownSettingsMarker is the file in the user's folder that keeps the choice.
func ownSettingsMarker() string {
	dir, err := getConfigDir() // the user's own folder in the app
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "use-own-settings")
}

func ownSettingsChosen() bool {
	p := ownSettingsMarker()
	if p == "" {
		return false
	}
	_, err := os.Stat(p)
	return err == nil
}

// IsServiceConnected reports whether the app works on the service's settings
// now (always true without a service: it works on its own).
func (a *App) IsServiceConnected() bool {
	return a.mode != api.ModeService || a.svcConnected.Load()
}

// onServiceDisconnected: the service no longer takes this app's token (it
// replaced it). The window shows the Connect banner again and asks once more
// on its own when it next comes to the front.
func (a *App) onServiceDisconnected() {
	a.autoConnectTried.Store(false)
	writeDebugLog("[Service] the service replaced its token: not connected until the window asks again")
	if a.ctx != nil && !a.headless {
		runtime.EventsEmit(a.ctx, "service:disconnected")
	}
}

// serviceRunning reports whether the background service this app works with
// answers (its /status answers 200 or 401; gui/api/answer.go): false without
// a service, while it is stopped, and while something other than the service
// holds its port (the peer check refuses that; it is logged). It uses what
// the app's last request to the service found, which the app renews every 3
// seconds while it works with the service (watchServiceRuns), and probes
// only when nothing has been sent yet.
func (a *App) serviceRunning() bool {
	if a.mode != api.ModeService || a.ownSettings.Load() || a.apiClient == nil {
		return false
	}
	if answers, known := a.apiClient.Answering(); known {
		return answers
	}
	code, _ := a.apiClient.ProbeStatus()
	return code != 0
}

// onServiceAnswer runs when the service stops or starts answering (the
// client's answer hook, installed by enterServiceMode). Stopped: the window
// shows that the service is not running, so this computer's Backup Sets do
// not run, with Start the service. Answering again: after a restart the
// service has a new token, so the app is not connected any more (the next
// request is refused and the window shows Connect); it asks once on its own
// when the window next comes to the front.
func (a *App) onServiceAnswer(answers bool, err error) {
	if a.isServiceProcess || a.mode != api.ModeService || a.ownSettings.Load() {
		return
	}
	setTrayServiceDown(!answers)
	if answers {
		a.autoConnectTried.Store(false)
		writeDebugLog("[Service] the background service answers again")
		a.emitEvent("service:started")
		return
	}
	var pe *api.ServicePeerError
	if errors.As(err, &pe) {
		writeDebugLog(fmt.Sprintf("[Service] %v; treated as the background service not running: this computer's Backup Sets do not run", pe))
	} else {
		writeDebugLog(fmt.Sprintf("[Service] the background service is not running (%v): this computer's Backup Sets do not run until it is started", err))
	}
	a.emitEvent("service:stopped")
}

// onServiceConnected loads the service's settings into the app and tells
// the window, which reloads its pages.
func (a *App) onServiceConnected() {
	if err := a.hydrateFromService(); err != nil {
		writeDebugLog(fmt.Sprintf("[Service] reading the service's settings: %v", err))
	}
	if a.ctx != nil && !a.headless {
		runtime.EventsEmit(a.ctx, "service:connected")
	}
}
