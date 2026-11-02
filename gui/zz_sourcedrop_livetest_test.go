package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// Opt-in live test of a real source disk dropping out mid-backup and coming
// back (source_wait.go in pbscommon):
//
//	PBS_SOURCEDROP_DIR    folder to back up, on a disk that can be detached
//	PBS_SOURCEDROP_TOGGLE script taking "detach" or "attach" for that disk
//	PBS_SOURCEDROP_FILE   base name of the file to drop the disk during
//
// The disk is detached shortly after the file starts, reattached 10 seconds
// later, and the backup must finish with no read errors and restore
// byte-identical.
func TestSourceDropLive(t *testing.T) {
	src, toggle, dropFile := os.Getenv("PBS_SOURCEDROP_DIR"), os.Getenv("PBS_SOURCEDROP_TOGGLE"), os.Getenv("PBS_SOURCEDROP_FILE")
	if src == "" || toggle == "" || dropFile == "" {
		t.Skip("set PBS_SOURCEDROP_DIR, PBS_SOURCEDROP_TOGGLE and PBS_SOURCEDROP_FILE")
	}
	want := e2eSnapshotState(t, src)
	backupID := fmt.Sprintf("sourcedrop-%d", time.Now().Unix())

	run := func(action string) {
		out, err := exec.Command("powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", toggle, action).CombinedOutput()
		t.Logf("%s: %s (err %v)", action, out, err)
	}
	var once sync.Once
	dropDone := make(chan struct{})
	var dropAt, backAt time.Time
	onFile := func(p string) {
		if filepath.Base(p) != dropFile {
			return
		}
		once.Do(func() {
			go func() {
				defer close(dropDone)
				time.Sleep(1500 * time.Millisecond)
				dropAt = time.Now()
				run("detach")
				time.Sleep(10 * time.Second)
				run("attach")
				backAt = time.Now()
			}()
		})
	}

	var rs *BackupStatus
	start := time.Now()
	err := RunBackupInline(BackupOptions{
		BaseURL: e2eBaseURL, AuthID: e2eAuthID, Secret: e2eSecret, Datastore: e2eStore, CertFingerprint: e2eFP,
		BackupObjects: []string{src}, BackupID: backupID, BackupType: "host", Kind: "directory",
		Compression: "fastest", OnFile: onFile,
		OnResult: func(s *BackupStatus) { rs = s },
	})
	select {
	case <-dropDone:
	case <-time.After(30 * time.Second):
		t.Fatal("the disk was never dropped: the drop file was not reached")
	}
	t.Logf("backup took %s; disk dropped at +%s, back at +%s", time.Since(start).Round(time.Millisecond),
		dropAt.Sub(start).Round(time.Millisecond), backAt.Sub(start).Round(time.Millisecond))
	if err != nil {
		t.Fatalf("backup failed: %v", err)
	}
	if rs == nil || !rs.Success() || len(rs.SkippedReadError) != 0 {
		t.Fatalf("bad result %+v", rs)
	}

	dest := t.TempDir()
	var verified int
	var fails []string
	o := RestoreOptions{BaseURL: e2eBaseURL, AuthID: e2eAuthID, Secret: e2eSecret, Datastore: e2eStore, CertFingerprint: e2eFP,
		BackupID: backupID, SnapshotTime: time.Unix(rs.BackupTime, 0).UTC(),
		DestPath: dest, Mode: RestoreModeAlternateAbs, Overwrite: true, VerifyAfterRestore: true,
		OnVerifySummary: func(v int, f []string) { verified, fails = v, f }}
	if err := RestoreSnapshotInline(o); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if verified == 0 || len(fails) > 0 {
		t.Errorf("verify-after-restore checked %d files, mismatches %v", verified, fails)
	}
	e2eCompare(t, "after drop", want, e2eSnapshotState(t, e2eFindRoot(t, dest)), "")
	t.Logf("restored and verified %d files", verified)
}
