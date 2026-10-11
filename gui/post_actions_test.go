package main

import "testing"

// The service runs scheduled sets too and has no window: a set that
// minimises to the tray after its backup must not touch the (nil) Wails
// context there.
func TestMinimizeAfterWithoutWindow(t *testing.T) {
	a := &App{}
	a.runPostBackupActions(ScheduledJob{Name: "t", MinimizeAfter: true}, true, "done")
	a.isServiceProcess = true
	a.runPostBackupActions(ScheduledJob{Name: "t", MinimizeAfter: true}, false, "failed")
}
