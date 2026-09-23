//go:build !windows && !service
// +build !windows,!service

package main

// preventCloseToTray reports whether closing the main window should be
// intercepted and turned into a hide-to-tray. There is no tray off-Windows, so
// closing the window must let the app actually quit.
func (a *App) preventCloseToTray() bool {
	return false
}

// SetupSystemTray is not supported on non-Windows platforms yet
func (a *App) SetupSystemTray() {
	writeDebugLog("System tray is only supported on Windows")
}

// MinimizeToTray is not supported on non-Windows platforms yet
func (a *App) MinimizeToTray() {
	writeDebugLog("MinimizeToTray is only supported on Windows")
}

// ShowFromTray is not supported on non-Windows platforms yet
func (a *App) ShowFromTray() {
	writeDebugLog("ShowFromTray is only supported on Windows")
}

// UpdateTrayTooltip is not supported on non-Windows platforms yet
func (a *App) UpdateTrayTooltip(message string) {
	writeDebugLog("UpdateTrayTooltip is only supported on Windows")
}
