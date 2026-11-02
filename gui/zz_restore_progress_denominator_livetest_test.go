package main

import (
	"context"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"pbscommon"
)

// TestZZRestoreProgressDenominatorManual is a manual, opt-in live-integration
// test proving the progress-bar denominator fix in restore_inline.go's
// withSnapshotReader: BytesTotal used to always report the WHOLE archive's
// size regardless of IncludePaths, found live 2026-09-30 (Mick restoring one
// subfolder out of a 458GB combined archive, progress looked like it was
// processing the whole snapshot). Reuses the same big-sibling/small-target
// shape as zz_selective_restore_bst_livetest_test.go, but this test checks
// the PROGRESS CALLBACK's own BytesTotal values during a selective restore,
// not just the final extracted content.
//
//	set PBS_LIVE_TEST=1
//	go test -run TestZZRestoreProgressDenominatorManual -v ./...
func TestZZRestoreProgressDenominatorManual(t *testing.T) {
	if os.Getenv("PBS_LIVE_TEST") == "" {
		t.Skip("set PBS_LIVE_TEST=1 to run this opt-in live integration test")
	}

	pbsOpts := struct {
		BaseURL, AuthID, Secret, Datastore, CertFingerprint string
	}{
		BaseURL:         "https://192.168.0.242:8007",
		AuthID:          "testclient@pbs!testclient",
		Secret:          "64376a7f-1aeb-42cb-a604-b75bf6a5561b",
		Datastore:       "teststore",
		CertFingerprint: "07:49:87:AE:76:0F:A8:E7:97:D8:1C:F0:44:1A:65:5D:2C:F3:10:49:00:EA:2C:53:E4:0B:CC:09:95:29:C5:22",
	}

	root := t.TempDir()
	bigDir := filepath.Join(root, "bigsibling")
	targetDir := filepath.Join(root, "target")
	if err := os.MkdirAll(bigDir, 0755); err != nil {
		t.Fatalf("MkdirAll bigsibling: %v", err)
	}
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		t.Fatalf("MkdirAll target: %v", err)
	}

	rng := rand.New(rand.NewSource(20261001))
	writeRandom := func(path string, mb int) {
		data := make([]byte, mb*1024*1024)
		rng.Read(data)
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatalf("WriteFile(%s): %v", path, err)
		}
	}
	writeRandom(filepath.Join(bigDir, "big1.bin"), 20)
	writeRandom(filepath.Join(bigDir, "big2.bin"), 20)
	targetContent := make([]byte, 512*1024)
	rng.Read(targetContent)
	if err := os.WriteFile(filepath.Join(targetDir, "small.bin"), targetContent, 0644); err != nil {
		t.Fatalf("WriteFile target/small.bin: %v", err)
	}

	backupID := "restore-progress-denom-test"
	var resultStatus *BackupStatus
	bOpts := BackupOptions{
		Ctx:             context.Background(),
		BaseURL:         pbsOpts.BaseURL,
		AuthID:          pbsOpts.AuthID,
		Secret:          pbsOpts.Secret,
		Datastore:       pbsOpts.Datastore,
		CertFingerprint: pbsOpts.CertFingerprint,
		BackupObjects:   []string{root},
		BackupID:        backupID,
		BackupType:      "host",
		Kind:            "directory",
		Compression:     "fastest",
		OnResult:        func(s *BackupStatus) { resultStatus = s },
	}
	t.Log("Backing up tree (40MB sibling + 512KB target)...")
	if err := RunBackupInline(bOpts); err != nil {
		t.Fatalf("RunBackupInline: %v", err)
	}
	if resultStatus == nil || !resultStatus.Success() {
		t.Fatalf("backup did not report success: %+v", resultStatus)
	}
	t.Logf("backup: newChunks=%d totalBytes=%d", resultStatus.NewChunks, resultStatus.TotalBytes)

	client := &pbscommon.PBSClient{
		BaseURL:         pbsOpts.BaseURL,
		CertFingerPrint: pbsOpts.CertFingerprint,
		AuthID:          pbsOpts.AuthID,
		Secret:          pbsOpts.Secret,
		Datastore:       pbsOpts.Datastore,
		Insecure:        true,
	}
	client.Connect(true, "host")
	defer client.Close()

	snaps, err := client.ListSnapshots()
	if err != nil {
		t.Fatalf("ListSnapshots: %v", err)
	}
	var snapTime time.Time
	found := false
	for _, m := range snaps {
		if m.BackupID == backupID {
			snapTime = time.Unix(m.BackupTime, 0).UTC()
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("no snapshot found for backupID %s after backup completed", backupID)
	}

	destDir := t.TempDir()
	// lastBytesTotal (not max): what matters is what the user actually sees
	// at any given moment, and in particular what's left on screen once the
	// restore finishes — not whether some transient early reading (e.g.
	// while the fast-path resolver itself was still running, before it knew
	// the narrowed total) was ever larger.
	var lastBytesTotal uint64
	var sawAnyStats bool
	rOpts := RestoreOptions{
		Ctx:             context.Background(),
		BaseURL:         pbsOpts.BaseURL,
		AuthID:          pbsOpts.AuthID,
		Secret:          pbsOpts.Secret,
		Datastore:       pbsOpts.Datastore,
		CertFingerprint: pbsOpts.CertFingerprint,
		BackupID:        backupID,
		SnapshotTime:    snapTime,
		DestPath:        destDir,
		IncludePaths:    []string{filepath.Base(root) + "/target"},
		OnStats: func(s *RestoreProgressStats) {
			sawAnyStats = true
			lastBytesTotal = s.BytesTotal
			t.Logf("OnStats: bytesDone=%d bytesTotal=%d archive=%s", s.BytesDone, s.BytesTotal, s.CurrentArchive)
		},
	}
	t.Log("Restoring ONLY the small target subfolder, watching progress...")
	if err := RestoreSnapshotInline(rOpts); err != nil {
		t.Fatalf("RestoreSnapshotInline: %v", err)
	}

	if !sawAnyStats {
		t.Fatal("never saw a single OnStats callback — can't verify the fix at all")
	}

	const wholeArchiveApprox = 40 * 1024 * 1024 // the ~40MB sibling dominates total archive size
	const targetApprox = 512 * 1024
	t.Logf("final BytesTotal reported for the selective restore: %d (target ~%d, whole archive ~%d)",
		lastBytesTotal, targetApprox, wholeArchiveApprox)

	if lastBytesTotal >= wholeArchiveApprox {
		t.Fatalf("FAIL: final BytesTotal (%d) is as large as the whole archive (~%d) — denominator was NOT narrowed to the selection, this is the original bug",
			lastBytesTotal, wholeArchiveApprox)
	}
	// Generous upper bound (target content + PXAR headers/metadata overhead),
	// still an order of magnitude below the whole archive.
	if lastBytesTotal > targetApprox*3 {
		t.Fatalf("FAIL: final BytesTotal (%d) is much larger than the target's own size (~%d) — denominator looks wrong even though it's not the full archive size",
			lastBytesTotal, targetApprox)
	}
	t.Logf("PASS: selective restore's final progress denominator (%d bytes) reflects the SELECTION, not the whole %d-byte archive",
		lastBytesTotal, wholeArchiveApprox)
}
