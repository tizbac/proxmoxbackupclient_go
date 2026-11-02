//go:build windows && !service
// +build windows,!service

package main

import (
	"fmt"
	"os"
	"path/filepath"
	goruntime "runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/getlantern/systray"
	"github.com/go-toast/toast"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

var (
	trayInitialized = false
	menuShow        *systray.MenuItem
	menuPause       *systray.MenuItem
	menuQuit        *systray.MenuItem
)

// The tray's Pause/Resume item and tooltip. trayMu guards the rest.
var (
	trayMu         sync.Mutex
	trayTooltipMsg = "Scheduled backups active" // last UpdateTrayTooltip message
	trayRunning    bool                         // a backup runs: the item is enabled
	trayPaused     bool                         // it is paused: the item says Resume
	trayServiceOff bool                         // the background service is installed but not running
	trayReady      bool                         // onReady has run: the tooltip can be set
	// trayPublicMsg is what the service's public status says while the app
	// holds no token (token_drop.go); "" otherwise.
	trayPublicMsg string
	// trayViaWindow: the app holds no token, so Pause opens the window and
	// asks for it first.
	trayViaWindow bool
	// trayPauseClickedAt (Unix ns) is when the item was last clicked. A service
	// run list fetched before then may not show that click yet.
	trayPauseClickedAt atomic.Int64
)

// preventCloseToTray reports whether closing the main window should be
// intercepted and turned into a hide-to-tray. On Windows the tray keeps the
// app alive, so we swallow the close.
func (a *App) preventCloseToTray() bool {
	return true
}

// SetupSystemTray initializes the system tray icon and menu
func (a *App) SetupSystemTray() {
	if trayInitialized {
		return
	}

	writeDebugLog("Setting up system tray")

	// Setup tray in goroutine to avoid blocking
	go func() {
		// The tray's hidden window gets its messages only on the OS thread
		// that created it. systray locks the thread in its init, which covers
		// a Run on the main goroutine only; this Run has a goroutine of its
		// own, so without the lock the scheduler can move the message loop to
		// another thread and the tray stops answering (seen on a test machine
		// during a backup, 2026-10-16: right-click did nothing).
		goruntime.LockOSThread()
		systray.Run(onReady(a), onExit)
	}()

	trayInitialized = true
}

func onReady(a *App) func() {
	return func() {
		// Set tray icon from embedded PNG data (icon.go)
		systray.SetIcon(TrayIconForBrand(BrandFromExecutable()))
		name := trayName()
		systray.SetTitle(name)
		trayMu.Lock()
		trayReady = true
		showTrayTooltipLocked() // "Scheduled backups active", or that the service is not running
		trayMu.Unlock()

		// Add menu items
		menuShow = systray.AddMenuItem("🖥️ Show window", "Open the "+name+" interface")
		systray.AddSeparator()

		menuPause = systray.AddMenuItem("⏸️ Pause backup", "Pause the running backup; it carries on from the same place on Resume")
		menuPause.Disable() // until a backup runs (trayPauseLoop)

		menuStatus := systray.AddMenuItem("📊 Backup status", "See scheduled backup status")
		menuStatus.Disable() // For display only

		systray.AddSeparator()
		menuQuit = systray.AddMenuItem("❌ Quit", "Close "+name)

		// Handle menu item clicks
		go func() {
			for {
				select {
				case <-menuShow.ClickedCh:
					writeDebugLog("Tray: Show window clicked")
					a.ShowFromTray()
				case <-menuPause.ClickedCh:
					a.trayPauseClicked()
				case <-menuQuit.ClickedCh:
					writeDebugLog("Tray: Quit clicked")
					// Quit systray first
					systray.Quit()
					// RequestQuit (not a bare runtime.Quit) actually terminates
					// the app now — see forceQuitRequested's doc comment in
					// main.go. The old code here masked the fact that a plain
					// runtime.Quit() was silently swallowed (same beforeClose
					// hook the window's own close button hits) with a hacky
					// 2-second delayed os.Exit(0) fallback; no longer needed.
					a.RequestQuit()
				}
			}
		}()

		writeDebugLog("System tray initialized")

		// Populate the real idle tooltip right away, rather than leaving the
		// generic one up to a minute until the scheduler's own next tick.
		a.RefreshIdleTooltip()

		go a.trayPauseLoop()
	}
}

// trayRunState reports whether a backup runs, in this process or, with Run as
// Service, in the service, and whether it is paused. ok is false while the
// service's run list is older than the last click on the item. viaWindow:
// the app holds no token (token_drop.go), so what runs comes from the
// service's public status, which does not say whether it is paused, and
// Pause has to open the window and ask for the token first.
func (a *App) trayRunState() (running, paused, viaWindow, ok bool) {
	if localBackupRunning() {
		// With several runs, Pause and Resume here act on them all.
		return true, anyRunPaused().Paused, false, true
	}
	if !a.serviceRunsBackups() {
		return false, false, false, true
	}
	if a.holdsNoToken() {
		st := latestPublicStatus.Load()
		return st != nil && st.BackupsRunning > 0, false, true, true
	}
	snap := latestServiceRuns.Load()
	if snap == nil {
		return false, false, false, true
	}
	if snap.at.UnixNano() < trayPauseClickedAt.Load() {
		return false, false, false, false
	}
	for _, r := range snap.runs.Runs {
		if r.Running && !r.Complete {
			return true, r.Paused, false, true
		}
	}
	return false, false, false, true
}

// trayPauseLoop keeps the Pause/Resume item in step with the backup, also
// when Pause is pressed in the window: enabled only while a backup runs, and
// Resume while it is paused.
// Once a run has ended it also puts the idle tooltip back, as soon as the
// run has let go of the operation slot, rather than at the scheduler's next
// once-a-minute refresh.
func (a *App) trayPauseLoop() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	wasRunning, idlePending := false, false
	for range ticker.C {
		running, paused, viaWindow, ok := a.trayRunState()
		if !ok {
			continue
		}
		setTrayPause(running, paused, viaWindow)
		if wasRunning && !running {
			idlePending = true
		}
		wasRunning = running
		if idlePending && !running && currentOperationLabel() == "" {
			idlePending = false
			a.RefreshIdleTooltip()
		}
	}
}

// setTrayPause shows a backup's state on the Pause/Resume item and in the
// tooltip. viaWindow: the app holds no token, so the item opens the window
// to connect first (pauseViaWindow).
func setTrayPause(running, paused, viaWindow bool) {
	paused = running && paused && !viaWindow
	trayMu.Lock()
	defer trayMu.Unlock()
	if menuPause == nil || (running == trayRunning && paused == trayPaused && viaWindow == trayViaWindow) {
		return
	}
	trayRunning, trayPaused, trayViaWindow = running, paused, viaWindow
	switch {
	case viaWindow:
		menuPause.SetTitle("⏸️ Pause backup...")
		menuPause.SetTooltip("Opens the window: the app connects to the background service first, then pauses the backup")
	case paused:
		menuPause.SetTitle("▶️ Resume backup")
		menuPause.SetTooltip("Carry on with the paused backup")
	default:
		menuPause.SetTitle("⏸️ Pause backup")
		menuPause.SetTooltip("Pause the running backup; it carries on from the same place on Resume")
	}
	if running {
		menuPause.Enable()
	} else {
		menuPause.Disable()
	}
	showTrayTooltipLocked()
}

// trayPauseClicked pauses or resumes the running backup, wherever it runs.
func (a *App) trayPauseClicked() {
	trayPauseClickedAt.Store(time.Now().UnixNano())
	trayMu.Lock()
	resume, viaWindow := trayPaused, trayViaWindow
	trayMu.Unlock()
	if viaWindow && a.holdsNoToken() {
		go a.pauseViaWindow()
		return
	}
	var st BackupPauseState
	var err error
	if resume {
		writeDebugLog("Tray: Resume backup clicked")
		st, err = a.ResumeBackup()
	} else {
		writeDebugLog("Tray: Pause backup clicked")
		st, err = a.PauseBackup()
	}
	if err != nil {
		writeDebugLog(fmt.Sprintf("Tray: pause/resume failed: %v", err))
		return
	}
	setTrayPause(true, st.Paused, false)
}

func onExit() {
	writeDebugLog("System tray exiting")
}

// MinimizeToTray hides the window and minimizes to tray (also the close
// prompt's Minimize to Tray). Hidden for a while, the app lets go of the
// service's token (token_drop.go).
func (a *App) MinimizeToTray() {
	writeDebugLog("Minimizing to tray")
	runtime.WindowHide(a.ctx)
	a.windowHiddenToTray()
}

// ShowFromTray shows the window from tray
func (a *App) ShowFromTray() {
	writeDebugLog("Showing from tray")
	a.windowShownAgain()
	runtime.WindowShow(a.ctx)
	runtime.WindowUnminimise(a.ctx)
	// The page reads the Backup Sets again: hidden, it may not have.
	runtime.EventsEmit(a.ctx, "window:shown")
}

// UpdateTrayTooltip updates the tray icon tooltip (e.g., with next backup time)
func (a *App) UpdateTrayTooltip(message string) {
	if !trayInitialized {
		return
	}
	trayMu.Lock()
	defer trayMu.Unlock()
	trayTooltipMsg = message
	showTrayTooltipLocked()
}

// setTrayServiceDown says in the tooltip that the background service, which
// runs this computer's Backup Sets, is not running (service_connect.go), or
// stops saying so.
func setTrayServiceDown(down bool) {
	trayMu.Lock()
	defer trayMu.Unlock()
	if trayServiceOff == down {
		return
	}
	trayServiceOff = down
	if trayReady {
		showTrayTooltipLocked()
	}
}

// setTrayPublicText shows msg, from the service's public status, in the
// tooltip while the app holds no token ("" goes back to the usual text). It
// reports whether the text changed.
func setTrayPublicText(msg string) bool {
	trayMu.Lock()
	defer trayMu.Unlock()
	if trayPublicMsg == msg {
		return false
	}
	trayPublicMsg = msg
	if trayReady {
		showTrayTooltipLocked()
	}
	return true
}

// showTrayTooltipLocked sets the tooltip from the last message, saying so
// while the backup is paused, or while the background service is not
// running; while the app holds no token, from the service's public status.
// trayMu must be held.
func showTrayTooltipLocked() {
	msg := trayTooltipMsg
	if trayServiceOff {
		msg = "Background service not running: Backup Sets do not run"
	} else if trayPublicMsg != "" {
		msg = trayPublicMsg
	} else if trayPaused {
		if label, ok := strings.CutPrefix(msg, "Backup running: "); ok {
			msg = "Backup paused: " + label
		} else {
			msg = "Backup paused"
		}
	}
	systray.SetTooltip(fmt.Sprintf("%s - %s", trayName(), msg))
}

// trayName is the brand's title, so a branded build's tray says its own
// name (the brand comes from the executable's name, see brand.go).
func trayName() string {
	return BrandFromExecutable().Title
}

var (
	trayIconTempOnce sync.Once
	trayIconTempFile string
)

// trayIconTempPath writes the embedded tray .ico out to a temp file once and
// reuses it — go-toast/toast (unlike systray.SetIcon) needs an actual file
// path on disk, not raw bytes, since it hands the path straight to
// PowerShell's WinRT toast call. Returns "" if the write fails; the toast
// still shows fine without a custom icon in that case.
func trayIconTempPath() string {
	trayIconTempOnce.Do(func() {
		brand := BrandFromExecutable()
		path := filepath.Join(os.TempDir(), normalizeBrandKey(brand.Name)+"-tray-icon.ico")
		if err := os.WriteFile(path, TrayIconForBrand(brand), 0644); err != nil {
			writeDebugLog(fmt.Sprintf("trayIconTempPath: write failed: %v", err))
			return
		}
		trayIconTempFile = path
	})
	return trayIconTempFile
}

// ShowToastNotification pops a real Windows toast (Action Center), separate
// from the tray tooltip (which only shows on hover) and from the existing
// email-notification feature (which needs SMTP configured and only reaches
// an inbox, not the desktop). Asked for 2026-09-30: actual desktop
// notifications on backup/restore completion, not just a static tray icon.
// go-toast/toast shells out to PowerShell's WinRT ToastNotificationManager —
// no CGO, same mechanism Windows' own built-in apps use. Never fatal: a
// failed toast (e.g. notifications disabled in Windows Settings) only logs,
// it must never fail the backup/restore it's reporting on.
func (a *App) ShowToastNotification(title, message string, isError bool) {
	n := toast.Notification{
		AppID:   "Proxmox Backup Client",
		Title:   title,
		Message: message,
		Icon:    trayIconTempPath(),
	}
	if isError {
		n.Audio = toast.SMS // pulse tag on the toast for a failure
	}
	if err := n.Push(); err != nil {
		writeDebugLog(fmt.Sprintf("ShowToastNotification failed (non-fatal): %v", err))
	}
}

// RefreshIdleTooltip sets the tray tooltip to the soonest upcoming enabled
// Backup Set's next run, or the generic fallback if none — called once a
// minute from the scheduler's own tick (scheduler.go) and right after any
// backup/restore finishes. Skips entirely while something is actually
// running (currentOperationLabel, operation_queue.go) so it can never
// clobber that more useful "Backup running: X" tooltip.
func (a *App) RefreshIdleTooltip() {
	if !trayInitialized || currentOperationLabel() != "" {
		return
	}
	// Without the token the Backup Sets cannot be read: the tray shows the
	// service's public status instead (watchServiceRuns).
	if a.holdsNoToken() {
		return
	}
	jobs, err := a.GetScheduledJobs()
	if err != nil {
		return
	}
	var soonestName string
	var soonestAt time.Time
	for _, job := range jobs {
		if !job.Enabled || job.NextRun == "" {
			continue
		}
		t, err := time.Parse(time.RFC3339, job.NextRun)
		if err != nil {
			continue
		}
		if soonestAt.IsZero() || t.Before(soonestAt) {
			soonestAt = t
			soonestName = job.Name
		}
	}
	if soonestName == "" {
		a.UpdateTrayTooltip("Scheduled backups active")
		return
	}
	a.UpdateTrayTooltip(fmt.Sprintf("Next: %s at %s", soonestName, soonestAt.Local().Format("02 Jan 15:04")))
}

// pauseViaWindow is the tray's Pause while the app holds no token
// (token_drop.go): Pause needs it, so the window opens and the app asks for
// it (one prompt), then pauses the service's backups. Declined, nothing is
// paused and the window shows Connect.
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
