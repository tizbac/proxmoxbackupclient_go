//go:build !service
// +build !service

package main

import (
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// buildAppMenu constructs the native OS menu bar. Every action is wired
// through a "nav:*" Wails event rather than calling App methods directly,
// so the frontend's existing activeTab state machine stays the single
// source of truth for what's on screen — the menu only ever asks it to
// switch tabs or open an overlay, the same way a sidebar click would.
func buildAppMenu(a *App) *menu.Menu {
	appMenu := menu.NewMenu()

	fileMenu := appMenu.AddSubmenu("File")
	fileMenu.AddText("Backup Now", keys.CmdOrCtrl("b"), func(_ *menu.CallbackData) {
		runtime.EventsEmit(a.ctx, "nav:goto", "backup")
	})
	fileMenu.AddSeparator()
	fileMenu.AddText("Minimize to Tray", nil, func(_ *menu.CallbackData) {
		a.MinimizeToTray()
	})
	fileMenu.AddSeparator()
	fileMenu.AddText("Exit", keys.CmdOrCtrl("q"), func(_ *menu.CallbackData) {
		// RequestQuit (not a bare runtime.Quit) — closing the window normally
		// minimizes to tray by design; Exit here means a real, unconditional
		// quit. See forceQuitRequested's doc comment in main.go for why a
		// plain runtime.Quit() alone doesn't actually exit.
		a.RequestQuit()
	})

	viewMenu := appMenu.AddSubmenu("View")
	viewMenu.AddText("Reports", nil, func(_ *menu.CallbackData) {
		runtime.EventsEmit(a.ctx, "nav:goto", "reports")
	})
	viewMenu.AddText("Message Log", nil, func(_ *menu.CallbackData) {
		runtime.EventsEmit(a.ctx, "nav:goto", "messagelog")
	})

	// Backup/Restore live here rather than View — the Backup tab IS the
	// backup-sets manager now, which reads more like a tool you open than a
	// view you switch to. This also retires the old disabled "Manage Backup
	// Sets…" placeholder that predated the real Backup Sets UI.
	toolsMenu := appMenu.AddSubmenu("Tools")
	toolsMenu.AddText("Backup", nil, func(_ *menu.CallbackData) {
		runtime.EventsEmit(a.ctx, "nav:goto", "backup")
	})
	toolsMenu.AddText("Restore", nil, func(_ *menu.CallbackData) {
		runtime.EventsEmit(a.ctx, "nav:goto", "restore")
	})
	toolsMenu.AddSeparator()
	toolsMenu.AddText("Preferences…", nil, func(_ *menu.CallbackData) {
		runtime.EventsEmit(a.ctx, "nav:preferences")
	})
	toolsMenu.AddSeparator()
	// Points at /releases/latest rather than a specific version's asset URL,
	// so this never goes stale across future releases even though the ISO's
	// filename carries a version number.
	toolsMenu.AddText("Download Bare Metal Restore ISO…", nil, func(_ *menu.CallbackData) {
		runtime.BrowserOpenURL(a.ctx, "https://github.com/mjb-is/proxmoxbackupclient_go/releases/latest")
	})

	helpMenu := appMenu.AddSubmenu("Help")
	helpMenu.AddText("Known Limitations", nil, func(_ *menu.CallbackData) {
		runtime.EventsEmit(a.ctx, "nav:limitations")
	})
	helpMenu.AddSeparator()
	helpMenu.AddText("About", nil, func(_ *menu.CallbackData) {
		runtime.EventsEmit(a.ctx, "nav:goto", "about")
	})

	return appMenu
}
