// +build windows

package main

import (
	"fmt"
	"os"
	"time"

	"github.com/getlantern/systray"
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
		brand := BrandFromExecutable()

		// Set the tray icon and labels from the runtime-resolved brand, so a
		// branded exe (NimbusBackup.exe, ...) shows its own identity in the
		// tray instead of the default one.
		systray.SetIcon(TrayIconForBrand(brand))
		systray.SetTitle(brand.Title)
		systray.SetTooltip(brand.Title + " - Scheduled backups active")

		// Add menu items
		menuShow = systray.AddMenuItem("🖥️ Show window", "Open the "+brand.Title+" interface")
		systray.AddSeparator()

		menuStatus := systray.AddMenuItem("📊 Backup status", "See scheduled backup status")
		menuStatus.Disable() // For display only

		systray.AddSeparator()
		menuQuit = systray.AddMenuItem("❌ Quit", "Close "+brand.Title)

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
					// Request Wails shutdown
					runtime.Quit(a.ctx)
					// Force exit after short delay if graceful shutdown doesn't work
					go func() {
						time.Sleep(2 * time.Second)
						writeDebugLog("Force exit after timeout")
						os.Exit(0)
					}()
				}
			}
		}()

		writeDebugLog("System tray initialized")
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
	systray.SetTooltip(fmt.Sprintf("%s - %s", BrandFromExecutable().Title, message))
}
