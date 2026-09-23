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
	fileMenu.AddText("Exit", keys.CmdOrCtrl("q"), func(_ *menu.CallbackData) {
		runtime.Quit(a.ctx)
	})

	viewMenu := appMenu.AddSubmenu("View")
	viewMenu.AddText("Backup", nil, func(_ *menu.CallbackData) {
		runtime.EventsEmit(a.ctx, "nav:goto", "backup")
	})
	viewMenu.AddText("Restore", nil, func(_ *menu.CallbackData) {
		runtime.EventsEmit(a.ctx, "nav:goto", "restore")
	})
	viewMenu.AddText("Reports", nil, func(_ *menu.CallbackData) {
		runtime.EventsEmit(a.ctx, "nav:goto", "reports")
	})
	viewMenu.AddText("Message Log", nil, func(_ *menu.CallbackData) {
		runtime.EventsEmit(a.ctx, "nav:goto", "messagelog")
	})

	toolsMenu := appMenu.AddSubmenu("Tools")
	backupSetsItem := toolsMenu.AddText("Manage Backup Sets…", nil, nil)
	backupSetsItem.Disabled = true // wizard lands in a later phase of the UI overhaul
	toolsMenu.AddText("Preferences…", keys.CmdOrCtrl(","), func(_ *menu.CallbackData) {
		runtime.EventsEmit(a.ctx, "nav:preferences")
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
