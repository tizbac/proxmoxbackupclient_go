//go:build windows && !service
// +build windows,!service

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/getlantern/systray"
	"github.com/go-toast/toast"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

var (
	trayInitialized = false
	menuShow        *systray.MenuItem
	menuQuit        *systray.MenuItem
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
		systray.Run(onReady(a), onExit)
	}()

	trayInitialized = true
}

func onReady(a *App) func() {
	return func() {
		// Set tray icon from embedded PNG data (icon.go)
		systray.SetIcon(TrayIconData)
		systray.SetTitle("Proxmox Backup Client")
		systray.SetTooltip("Proxmox Backup Client - Scheduled backups active")

		// Add menu items
		menuShow = systray.AddMenuItem("🖥️ Show window", "Open the Proxmox Backup Client interface")
		systray.AddSeparator()

		menuStatus := systray.AddMenuItem("📊 Backup status", "See scheduled backup status")
		menuStatus.Disable() // For display only

		systray.AddSeparator()
		menuQuit = systray.AddMenuItem("❌ Quit", "Close Proxmox Backup Client")

		// Handle menu item clicks
		go func() {
			for {
				select {
				case <-menuShow.ClickedCh:
					writeDebugLog("Tray: Show window clicked")
					// Show the main window
					runtime.WindowShow(a.ctx)
					runtime.WindowUnminimise(a.ctx)
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
	}
}

func onExit() {
	writeDebugLog("System tray exiting")
}

// MinimizeToTray hides the window and minimizes to tray
func (a *App) MinimizeToTray() {
	writeDebugLog("Minimizing to tray")
	runtime.WindowHide(a.ctx)
}

// ShowFromTray shows the window from tray
func (a *App) ShowFromTray() {
	writeDebugLog("Showing from tray")
	runtime.WindowShow(a.ctx)
	runtime.WindowUnminimise(a.ctx)
}

// UpdateTrayTooltip updates the tray icon tooltip (e.g., with next backup time)
func (a *App) UpdateTrayTooltip(message string) {
	if !trayInitialized {
		return
	}
	systray.SetTooltip(fmt.Sprintf("Proxmox Backup Client - %s", message))
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
		path := filepath.Join(os.TempDir(), "pbsgo-tray-icon.ico")
		if err := os.WriteFile(path, TrayIconData, 0644); err != nil {
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
// an inbox, not the desktop). Mick, 2026-09-30: wanted actual desktop
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
