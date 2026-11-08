//go:build service

package main

// emitEvent: the service has no window to send events to.
func (a *App) emitEvent(name string, data ...interface{}) {}
