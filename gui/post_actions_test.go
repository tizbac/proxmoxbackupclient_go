package main

import (
	"testing"

	"clientcommon"
)

type fakeNoter struct{ notes []string }

func (f *fakeNoter) AddNote(note string) error {
	f.notes = append(f.notes, note)
	return nil
}

// A notification e-mail that cannot be sent is noted in the snapshot's PBS
// comment, as the command-line tools do; with no e-mail due, nothing is noted.
func TestMailFailureNotedOnSnapshot(t *testing.T) {
	a := &App{config: &Config{SMTPHost: "127.0.0.1", SMTPPort: "1"}} // nothing listens on port 1
	job := ScheduledJob{Name: "t", EmailOnSuccess: true, EmailOnSuccessTo: "someone@example.com"}

	n := &fakeNoter{}
	a.runPostBackupActions(job, true, "done", n)
	if len(n.notes) != 1 || n.notes[0] != clientcommon.MailFailedNote {
		t.Fatalf("notes %q, want the mail warning once", n.notes)
	}

	quiet := &fakeNoter{}
	a.runPostBackupActions(ScheduledJob{Name: "t"}, true, "done", quiet)
	if len(quiet.notes) != 0 {
		t.Fatalf("noted %q with no e-mail due", quiet.notes)
	}
	a.runPostBackupActions(job, true, "done", nil) // no snapshot: nothing to note, no panic
}

// The service runs scheduled sets too and has no window: a set that
// minimises to the tray after its backup must not touch the (nil) Wails
// context there.
func TestMinimizeAfterWithoutWindow(t *testing.T) {
	a := &App{}
	a.runPostBackupActions(ScheduledJob{Name: "t", MinimizeAfter: true}, true, "done", nil)
	a.isServiceProcess = true
	a.runPostBackupActions(ScheduledJob{Name: "t", MinimizeAfter: true}, false, "failed", nil)
}

// A set that closes the app when it finishes must not stop the service.
func TestExitAfterIgnoredInService(t *testing.T) {
	exits := make(chan struct{}, 2)
	old := exitAppAfterBackup
	exitAppAfterBackup = func() { exits <- struct{}{} }
	defer func() { exitAppAfterBackup = old }()

	svc := &App{isServiceProcess: true}
	svc.runPostBackupActions(ScheduledJob{Name: "t", ExitAppAfter: true}, true, "done", nil)

	app := &App{}
	app.runPostBackupActions(ScheduledJob{Name: "t", ExitAppAfter: true}, true, "done", nil)

	<-exits // the app's exit (it runs in a goroutine)
	select {
	case <-exits:
		t.Fatal("the service exited after a set that closes the app")
	default:
	}
}
