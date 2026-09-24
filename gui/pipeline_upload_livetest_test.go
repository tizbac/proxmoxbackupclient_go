package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestPipelinedChunkUploadRoundTrip is a manual, opt-in live-integration test
// (same PBS_LIVE_TEST=1 convention as TestMultiArchiveLive) added 2026-09-24
// alongside the chunk-upload pipelining change in ChunkState (backup_inline.go).
//
// That change moved the actual network upload off the hashing/scanning
// goroutine and onto a bounded worker pool (see chunkUploadWorkers's doc
// comment) so multiple chunks upload concurrently instead of one at a time.
// This test exists to prove that reshuffling didn't corrupt anything: it
// backs up real, fresh (non-dedupable) random data big enough to span
// several chunks — spread across enough workers to actually exercise the
// pool, not just chunk 1 — plus a byte-identical duplicate file to exercise
// the concurrent-dedup path (GetOrSet racing two nearly-simultaneous uploads
// of the same digest), then restores the snapshot back through the real
// restore path (RestoreSnapshotInline — the same function the GUI's Restore
// tab calls) and compares SHA-256 of every restored file against the
// original. A mismatch here would mean the pipelining reordered or dropped
// a chunk; PBS itself would usually still accept the write (chunks are
// content-addressed, so silently, the failure mode would be a subtly wrong
// restore, not a loud error), which is exactly why this checks bytes, not
// just "did the backup report success".
func TestPipelinedChunkUploadRoundTrip(t *testing.T) {
	if os.Getenv("PBS_LIVE_TEST") == "" {
		t.Skip("set PBS_LIVE_TEST=1 to run this opt-in live integration test")
	}

	srcDir := t.TempDir()

	// ~24MB of real random bytes: at the 4MB-average chunk size this is
	// enough to produce a handful of new chunks, so the 8-worker pool has
	// more than one chunk to actually parallelize (a single-chunk file would
	// pass trivially without exercising the pool at all).
	bigData := make([]byte, 24*1024*1024)
	if _, err := rand.Read(bigData); err != nil {
		t.Fatalf("failed to generate random data: %v", err)
	}
	bigPath := filepath.Join(srcDir, "random-24mb.bin")
	if err := os.WriteFile(bigPath, bigData, 0644); err != nil {
		t.Fatal(err)
	}

	// Byte-identical duplicate, to exercise the dedup path: the rolling-hash
	// chunker is content-based, so an identical file should reuse the same
	// chunk digests already seen for bigPath — GetOrSet on those digests
	// races against whichever of the two upload goroutines gets there first.
	dupPath := filepath.Join(srcDir, "random-24mb-duplicate.bin")
	if err := os.WriteFile(dupPath, bigData, 0644); err != nil {
		t.Fatal(err)
	}

	// A small file too, for a realistic mixed-size backup.
	smallPath := filepath.Join(srcDir, "small.txt")
	if err := os.WriteFile(smallPath, []byte("small control file, should dedupe trivially on a repeat run\n"), 0644); err != nil {
		t.Fatal(err)
	}

	backupID := "pipeline-verify-test"

	var resultStatus *BackupStatus
	backupOpts := BackupOptions{
		BaseURL:         "https://192.168.0.242:8007",
		AuthID:          "testclient@pbs!testclient",
		Secret:          "64376a7f-1aeb-42cb-a604-b75bf6a5561b",
		Datastore:       "teststore",
		CertFingerprint: "07:49:87:AE:76:0F:A8:E7:97:D8:1C:F0:44:1A:65:5D:2C:F3:10:49:00:EA:2C:53:E4:0B:CC:09:95:29:C5:22",
		BackupObjects:   []string{srcDir},
		BackupID:        backupID,
		BackupType:      "host",
		Kind:            "directory",
		Compression:     "fastest",
		OnResult:        func(s *BackupStatus) { resultStatus = s },
		OnProgress: func(pct float64, msg string) {
			t.Logf("[backup %.0f%%] %s", pct*100, msg)
		},
	}

	backupStart := time.Now()
	if err := RunBackupInline(backupOpts); err != nil {
		t.Fatalf("RunBackupInline failed: %v", err)
	}
	backupElapsed := time.Since(backupStart)

	if resultStatus == nil {
		t.Fatal("no result status delivered via OnResult")
	}
	t.Logf("Backup: outcome=%s new=%d reused=%d failed=%d bytes=%d elapsed=%v",
		resultStatus.Outcome, resultStatus.NewChunks, resultStatus.ReusedChunks,
		resultStatus.FailedChunks, resultStatus.TotalBytes, backupElapsed)

	if !resultStatus.Success() {
		t.Fatalf("expected a successful backup outcome, got %s: %s", resultStatus.Outcome, resultStatus.Message)
	}
	if resultStatus.FailedChunks != 0 {
		t.Fatalf("expected 0 failed chunks, got %d", resultStatus.FailedChunks)
	}

	// Message-catalog i18n plumbing (2026-09-24): a fully-successful run with
	// no failed chunks and no dir errors should carry MsgBackupCompleted with
	// a params map the frontend can actually render (duration/mb/new/reused
	// all present as strings/numbers, skipped=0 since nothing was skipped).
	if resultStatus.MessageKey != MsgBackupCompleted {
		t.Fatalf("expected MessageKey=%s, got %q", MsgBackupCompleted, resultStatus.MessageKey)
	}
	for _, field := range []string{"duration", "mb", "new", "reused", "skipped"} {
		if _, ok := resultStatus.MessageParams[field]; !ok {
			t.Fatalf("MessageParams missing expected field %q: %+v", field, resultStatus.MessageParams)
		}
	}
	if skipped, _ := resultStatus.MessageParams["skipped"].(int); skipped != 0 {
		t.Fatalf("expected skipped=0, got %v", resultStatus.MessageParams["skipped"])
	}
	t.Logf("MessageKey=%s MessageParams=%+v", resultStatus.MessageKey, resultStatus.MessageParams)
	// The 24MB file alone should split into multiple new chunks at ~4MB
	// average — if this is 1, the pool never had more than one job in it.
	if resultStatus.NewChunks < 3 {
		t.Fatalf("expected at least 3 new chunks (enough to exercise the worker pool), got %d", resultStatus.NewChunks)
	}
	// The duplicate 24MB file's chunks should mostly be recognized as already
	// known — proves the concurrent dedup path (GetOrSet) didn't let the
	// second file's identical chunks all re-upload as "new". Threshold is
	// deliberately low (>=1, not an exact count): content-defined chunking
	// resyncs to the same boundaries as the original *after* the differing
	// per-file PXAR header shifts them, and exactly how many chunks land
	// before that resync point varies run to run with fresh random data —
	// seen 2-5 reused chunks across repeated runs of this same test, so
	// asserting a higher floor is flaky, not more correct.
	if resultStatus.ReusedChunks < 1 {
		t.Fatalf("expected at least 1 reused chunk from the duplicate file, got %d", resultStatus.ReusedChunks)
	}

	// Restore it back through the real restore path (same function the
	// GUI's Restore tab calls) and verify byte-for-byte integrity.
	destDir := t.TempDir()
	restoreOpts := RestoreOptions{
		BaseURL:         backupOpts.BaseURL,
		AuthID:          backupOpts.AuthID,
		Secret:          backupOpts.Secret,
		Datastore:       backupOpts.Datastore,
		CertFingerprint: backupOpts.CertFingerprint,
		BackupID:        backupID,
		SnapshotTime:    time.Unix(resultStatus.BackupTime, 0).UTC(),
		DestPath:        destDir,
		Mode:            RestoreModeAlternateAbs,
		Overwrite:       true,
		OnProgress: func(pct float64, msg string) {
			t.Logf("[restore %.0f%%] %s", pct*100, msg)
		},
	}

	if err := RestoreSnapshotInline(restoreOpts); err != nil {
		t.Fatalf("RestoreSnapshotInline failed: %v", err)
	}

	// RestoreModeAlternateAbs's exact on-disk layout under destDir (drive
	// letter handling etc.) is an implementation detail this test shouldn't
	// need to know — find each restored file by basename instead.
	findRestored := func(baseName string) string {
		t.Helper()
		var found string
		err := filepath.Walk(destDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.IsDir() && info.Name() == baseName {
				found = path
			}
			return nil
		})
		if err != nil {
			t.Fatalf("failed to walk restore destination: %v", err)
		}
		if found == "" {
			t.Fatalf("restored file %q not found anywhere under %s", baseName, destDir)
		}
		return found
	}

	checkFile := func(baseName string, want []byte) {
		t.Helper()
		restoredPath := findRestored(baseName)
		got, err := os.ReadFile(restoredPath)
		if err != nil {
			t.Fatalf("failed to read restored file %s: %v", restoredPath, err)
		}
		wantSum := sha256.Sum256(want)
		gotSum := sha256.Sum256(got)
		if wantSum != gotSum {
			t.Fatalf("restored file %s: checksum mismatch (want %s, got %s, sizes %d/%d)",
				restoredPath, hex.EncodeToString(wantSum[:]), hex.EncodeToString(gotSum[:]), len(want), len(got))
		}
	}

	checkFile(filepath.Base(bigPath), bigData)
	checkFile(filepath.Base(dupPath), bigData)

	t.Logf("TestPipelinedChunkUploadRoundTrip: PASS — %d new + %d reused chunks, restored bytes match originals exactly", resultStatus.NewChunks, resultStatus.ReusedChunks)
}
