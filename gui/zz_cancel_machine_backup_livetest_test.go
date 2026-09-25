package main

import (
	"os"
	"testing"
	"time"
)

// TestCancelMachineBackupLive is a manual, opt-in live-integration test
// answering a question raised in TODO.md ("No way to cancel a running
// machine backup"): does CancelBackup() (the GUI Stop button's method)
// actually stop a running MACHINE backup, or only a directory backup?
//
// Drives RunBackupInline directly against the isolated test PBS
// (192.168.0.242, datastore teststore) with Kind: "machine" on this
// machine's own root block device, same pattern as TestMultiArchiveLive.
// Cancels a few seconds in (well before a real run would finish) and checks
// the backup actually stops early rather than running to completion.
//
// Run explicitly, as root, on a Linux box with elastio-snap loaded
// (e.g. pbstest-linux-client):
//
//	sudo PBS_LIVE_TEST=1 PBS_CANCEL_TEST_DEVICE=/dev/sda ./cancel_livetest.test -test.v -test.run TestCancelMachineBackupLive
func TestCancelMachineBackupLive(t *testing.T) {
	if os.Getenv("PBS_LIVE_TEST") == "" {
		t.Skip("set PBS_LIVE_TEST=1 to run this opt-in live integration test")
	}

	device := os.Getenv("PBS_CANCEL_TEST_DEVICE")
	if device == "" {
		t.Fatal("set PBS_CANCEL_TEST_DEVICE to the block device to back up (e.g. /dev/sda)")
	}

	app := &App{}

	var resultStatus *BackupStatus
	var runErr error
	done := make(chan struct{})

	start := time.Now()
	opts := BackupOptions{
		// Deliberately NOT set: the real GUI (gui/main.go) never sets Ctx
		// explicitly either, relying on RunBackupInline's own
		// `if opts.Ctx == nil { opts.Ctx = ctx }` to wire in the real
		// cancellable context. Setting it here (as TestMultiArchiveLive
		// does, harmlessly for a test that never cancels) would silently
		// defeat that wiring and this test would validate nothing.
		BaseURL: "https://192.168.0.242:8007",
		AuthID:          "testclient@pbs!testclient",
		Secret:          "64376a7f-1aeb-42cb-a604-b75bf6a5561b",
		Datastore:       "teststore",
		CertFingerprint: "07:49:87:AE:76:0F:A8:E7:97:D8:1C:F0:44:1A:65:5D:2C:F3:10:49:00:EA:2C:53:E4:0B:CC:09:95:29:C5:22",
		BackupObjects:   []string{device},
		BackupID:        "cancel-test",
		BackupType:      "host",
		Kind:            "machine",
		UseVSS:          true,
		OnResult:        func(s *BackupStatus) { resultStatus = s },
		OnProgress: func(pct float64, msg string) {
			t.Logf("[%.1fs] [progress %.1f%%] %s", time.Since(start).Seconds(), pct*100, msg)
		},
	}

	go func() {
		runErr = RunBackupInline(opts)
		close(done)
	}()

	// Give it a few seconds to get well past snapshot setup and into real
	// disk-read/upload, then request a Stop exactly like the GUI's button.
	time.Sleep(4 * time.Second)
	t.Logf("[%.1fs] calling CancelBackup()", time.Since(start).Seconds())
	if err := app.CancelBackup(); err != nil {
		t.Fatalf("CancelBackup returned an error: %v", err)
	}

	select {
	case <-done:
	case <-time.After(60 * time.Second):
		t.Fatalf("backup did not stop within 60s of CancelBackup() being called — Stop has no effect on a running machine backup")
	}

	elapsed := time.Since(start)
	t.Logf("RunBackupInline returned after %.1fs, err=%v", elapsed.Seconds(), runErr)
	if resultStatus != nil {
		t.Logf("Outcome=%s Message=%s", resultStatus.Outcome, resultStatus.Message)
	}

	// A real full-disk backup of this size takes minutes (8-10m for this
	// VM's disk in earlier real runs); stopping in well under a minute is
	// strong evidence the cancel actually interrupted it rather than racing
	// a coincidentally-fast completion.
	if elapsed > 90*time.Second {
		t.Fatalf("backup ran for %.1fs after CancelBackup() — looks like it ran to completion instead of stopping", elapsed.Seconds())
	}
	if runErr == nil {
		t.Fatal("expected RunBackupInline to return an error after cancellation, got nil (looks like it completed successfully instead of being cancelled)")
	}
}
