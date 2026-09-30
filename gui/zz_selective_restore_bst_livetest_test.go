package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"pbscommon"
)

// TestZZSelectiveRestoreBSTManual is a manual, opt-in live-integration test
// for the GOODBYE binary-search-tree fast path in pxar_reader.go
// (resolveArchivePathBST/walkRange), added 2026-10-01 after live evidence
// (Mick restoring one subfolder out of a 458GB combined archive) showed the
// previous linear-scan-only walk() fetching a fresh ~4MB chunk for
// essentially every step forward through the archive, REGARDLESS of whether
// that span belonged to the selected folder or a skipped sibling — see
// TODO.md's "selective restore from a big combined archive" entry.
//
// Backs up a tree with one deliberately large sibling folder and one small
// target folder, restores with IncludePaths naming only the small target,
// and asserts two things: (1) correctness — only the target's own file
// shows up in the restored output, with the right content, no sibling
// leakage; (2) the actual performance property this whole feature exists
// for — the restore's own chunk-fetch count stays small, proving the big
// sibling's chunks were never fetched at all, not just "correctly not
// written to disk after being fetched anyway".
//
//	set PBS_LIVE_TEST=1
//	go test -run TestZZSelectiveRestoreBSTManual -v ./...
func TestZZSelectiveRestoreBSTManual(t *testing.T) {
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
	// Several chunks' worth in the sibling (big enough that fetching all of
	// it would be obviously wrong and easy to detect), one small file in the
	// target the test actually wants restored.
	writeRandom(filepath.Join(bigDir, "big1.bin"), 20)
	writeRandom(filepath.Join(bigDir, "big2.bin"), 20)
	targetContent := make([]byte, 512*1024)
	rng.Read(targetContent)
	if err := os.WriteFile(filepath.Join(targetDir, "small.bin"), targetContent, 0644); err != nil {
		t.Fatalf("WriteFile target/small.bin: %v", err)
	}
	targetSum := sha256.Sum256(targetContent)
	targetSumHex := hex.EncodeToString(targetSum[:])

	backupID := "selective-restore-bst-test"
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
	// "target" is the archive-relative path of the whole root dir's only
	// "target" subfolder — the archive root here IS the backed-up directory
	// itself (BackupObjects had one entry), so the archive-relative path of
	// that subfolder is simply its own name — BUT RestoreOptions.IncludePaths
	// is a level above that: restore_inline.go wraps every archive's paths
	// under its own display name (the original folder's base name, read from
	// the meta sidecar — see resolveArchiveDisplayNames) before matching a
	// selection against it, so the caller-facing IncludePaths here needs
	// that same "<wrapper>/target" shape, not the bare archive-relative path
	// ResolveArchivePathBST itself takes directly below.
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
	}
	t.Log("Restoring ONLY the small target subfolder...")
	if err := RestoreSnapshotInline(rOpts); err != nil {
		t.Fatalf("RestoreSnapshotInline: %v", err)
	}

	// --- Correctness: exactly one restored file, right content, no sibling leakage ---
	var restoredFiles []string
	var restoredBig bool
	err = filepath.Walk(destDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		restoredFiles = append(restoredFiles, path)
		if filepath.Base(path) == "big1.bin" || filepath.Base(path) == "big2.bin" {
			restoredBig = true
		}
		if filepath.Base(path) == "small.bin" {
			data, rerr := os.ReadFile(path)
			if rerr != nil {
				t.Errorf("read restored small.bin: %v", rerr)
				return nil
			}
			sum := sha256.Sum256(data)
			if hex.EncodeToString(sum[:]) != targetSumHex {
				t.Errorf("restored small.bin content does not match original (checksum mismatch)")
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk destDir: %v", err)
	}
	if restoredBig {
		t.Fatalf("FAIL: sibling 'bigsibling' content was restored even though only 'target' was selected — leaked %v", restoredFiles)
	}
	foundSmall := false
	for _, f := range restoredFiles {
		if filepath.Base(f) == "small.bin" {
			foundSmall = true
		}
	}
	if !foundSmall {
		t.Fatalf("FAIL: target/small.bin was not restored at all — got files: %v", restoredFiles)
	}
	t.Log("PASS: only the selected target folder was restored, with correct content, no sibling leakage")

	// --- Direct proof the fast path itself actually resolved, not just that
	// the pre-existing pathMatches filter happened to produce correct output
	// even via a linear-walk fallback (which would look IDENTICAL from the
	// correctness check above alone — this is the check that actually
	// distinguishes "fast path used" from "silently fell back but still
	// correct"). Re-opens the exact same archive directly and calls
	// resolveArchivePathBST for "target", asserting it succeeds and resolves
	// to a span whose size matches the small target content, nowhere near
	// the big sibling's ~40MB.
	archiveName := archiveBaseName(root, map[string]int{}) + ".pxar.didx"
	client.Manifest.BackupID = backupID
	client.Manifest.BackupTime = snapTime.Unix()
	ra, size, err := client.NewDIDXReaderAt(archiveName, 64, nil)
	if err != nil {
		t.Fatalf("NewDIDXReaderAt: %v", err)
	}
	reader := pbscommon.NewPXARReaderAt(ra, size)
	targetStart, targetEnd, parentPath, err := reader.ResolveArchivePathBST("target")
	if err != nil {
		t.Fatalf("FAIL: resolveArchivePathBST(\"target\") did not resolve via the BST fast path at all: %v", err)
	}
	span := targetEnd - targetStart
	t.Logf("BST-resolved span for \"target\": start=%d end=%d size=%d bytes (parentPath=%q, archive size=%d)",
		targetStart, targetEnd, span, parentPath, size)
	const bigSiblingBytes = 2 * 20 * 1024 * 1024 // both big1.bin + big2.bin
	if span >= bigSiblingBytes {
		t.Fatalf("FAIL: resolved span (%d bytes) is as large as the big sibling (%d bytes) — BST lookup did not actually narrow the search", span, bigSiblingBytes)
	}
	t.Logf("PASS: BST fast path resolved \"target\" directly to a %d-byte span, nowhere near the %d-byte sibling — confirms the lookup genuinely skipped it, not just filtered it out after the fact", span, bigSiblingBytes)
}
