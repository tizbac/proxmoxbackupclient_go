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

// A set that closes the app when it finishes must not stop the service.
func TestExitAfterIgnoredInService(t *testing.T) {
	exits := make(chan struct{}, 2)
	old := exitAppAfterBackup
	exitAppAfterBackup = func() { exits <- struct{}{} }
	defer func() { exitAppAfterBackup = old }()

	svc := &App{isServiceProcess: true}
	svc.runPostBackupActions(ScheduledJob{Name: "t", ExitAppAfter: true}, true, "done")

	app := &App{}
	app.runPostBackupActions(ScheduledJob{Name: "t", ExitAppAfter: true}, true, "done")

	<-exits // the app's exit (it runs in a goroutine)
	select {
	case <-exits:
		t.Fatal("the service exited after a set that closes the app")
	default:
	}
}
