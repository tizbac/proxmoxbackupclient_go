// +build !windows

package main

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
