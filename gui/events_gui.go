//go:build !service

package main

import "github.com/wailsapp/wails/v2/pkg/runtime"

// emitEvent sends an event to the frontend; nothing happens without a
// window (a.ctx nil: tests, command-line runs).
func (a *App) emitEvent(name string, data ...interface{}) {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, name, data...)
}
