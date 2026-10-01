package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	"pbscommon"
)

// TestZZBSTRealisticReproManual is a manual, opt-in live-integration test
// built to reproduce, against a REALISTIC tree shape (many small files across
// many nested subdirectories, several real sibling folders), the discrepancy
// Mick hit live on 2026-10-01 restoring "Beeby Property" out of the real
// production "Deepthought - Data" snapshot: the client reported a 5.8GB/1698
// -chunk target, fetched past that (2453 chunks, 107%) before being stopped,
// and the actually-restored folder (stopped mid-way) measured 4.69GB —
// nowhere near either number. The small, flat trees in
// zz_selective_restore_bst_livetest_test.go and
// zz_restore_progress_denominator_livetest_test.go (one or two big files per
// folder) never exercised "thousands of small files, hundreds of
// directories", which is the actual shape that broke.
//
//	set PBS_LIVE_TEST=1
//	go test -run TestZZBSTRealisticReproManual -v -timeout 600s ./...
func TestZZBSTRealisticReproManual(t *testing.T) {
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
	rng := rand.New(rand.NewSource(20261001))

	// Several real sibling top-level folders, each with their OWN nested
	// structure — mimics F:\Data having many unrelated folders besides the
	// one being selectively restored.
	var targetFileCount int
	var targetTotalBytes int64
	makeTree := func(base string, numDirs, filesPerDir, fileSizeMin, fileSizeMax int, track bool) {
		for d := 0; d < numDirs; d++ {
			dir := filepath.Join(base, fmt.Sprintf("sub%03d", d))
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatalf("MkdirAll(%s): %v", dir, err)
			}
			for f := 0; f < filesPerDir; f++ {
				size := fileSizeMin
				if fileSizeMax > fileSizeMin {
					size += rng.Intn(fileSizeMax - fileSizeMin)
				}
				data := make([]byte, size)
				rng.Read(data)
				path := filepath.Join(dir, fmt.Sprintf("file%03d.bin", f))
				if err := os.WriteFile(path, data, 0644); err != nil {
					t.Fatalf("WriteFile(%s): %v", path, err)
				}
				if track {
					targetFileCount++
					targetTotalBytes += int64(size)
				}
			}
		}
	}

	// Siblings: 3 unrelated folders, smallish, BEFORE the target alphabetically.
	makeTree(filepath.Join(root, "aaa_sibling1"), 10, 20, 1024, 8192, false)
	makeTree(filepath.Join(root, "bbb_sibling2"), 15, 15, 2048, 16384, false)
	// Target: realistically shaped like "Beeby Property" — many small files,
	// many subdirectories (scaled down from 3737 files/495 dirs to keep this
	// test's runtime reasonable, same PROPORTIONS: ~7-8 files per dir).
	makeTree(filepath.Join(root, "target_folder"), 60, 8, 512, 65536, true)
	// One more sibling AFTER the target alphabetically.
	makeTree(filepath.Join(root, "zzz_sibling3"), 10, 20, 1024, 8192, false)
	// A large incompressible file AFTER the target: ~40 chunks of "whatever
	// comes next in the archive" that read-ahead must not fetch for a
	// selective restore of target_folder.
	bigAfter := make([]byte, 160*1024*1024)
	rng.Read(bigAfter)
	if err := os.WriteFile(filepath.Join(root, "zzz_sibling3", "big_after.bin"), bigAfter, 0644); err != nil {
		t.Fatalf("WriteFile big_after: %v", err)
	}

	t.Logf("Target folder: %d files, %d bytes (%.2f MB)", targetFileCount, targetTotalBytes, float64(targetTotalBytes)/1024/1024)

	backupID := "bst-realistic-repro-test"
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
	t.Log("Backing up realistic tree (4 top-level folders, hundreds of files)...")
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

	// --- Step 1: direct ResolveArchivePathBST check against the real archive ---
	archiveName := archiveBaseName(root, map[string]int{}) + ".pxar.didx"
	client.Manifest.BackupID = backupID
	client.Manifest.BackupTime = snapTime.Unix()
	ra, size, err := client.NewDIDXReaderAt(archiveName, 64, nil)
	if err != nil {
		t.Fatalf("NewDIDXReaderAt: %v", err)
	}
	reader := pbscommon.NewPXARReaderAt(ra, size)
	targetStart, targetEnd, parentPath, berr := reader.ResolveArchivePathBST("target_folder")
	if berr != nil {
		t.Fatalf("FAIL: ResolveArchivePathBST(\"target_folder\") failed on a realistic tree: %v", berr)
	}
	spanSize := targetEnd - targetStart
	t.Logf("ResolveArchivePathBST: start=%d end=%d span=%d parentPath=%q (archive size=%d)",
		targetStart, targetEnd, spanSize, parentPath, size)
	t.Logf("Real target content: %d bytes across %d files", targetTotalBytes, targetFileCount)

	// --- Step 2: ground truth via a WHOLE-archive restore (no selection),
	// which never touches any BST code. ---
	var groundTruthBytes int64
	var groundTruthFiles int
	destFull := t.TempDir()
	fullOpts := RestoreOptions{
		Ctx:             context.Background(),
		BaseURL:         pbsOpts.BaseURL,
		AuthID:          pbsOpts.AuthID,
		Secret:          pbsOpts.Secret,
		Datastore:       pbsOpts.Datastore,
		CertFingerprint: pbsOpts.CertFingerprint,
		BackupID:        backupID,
		SnapshotTime:    snapTime,
		DestPath:        destFull,
		// No IncludePaths at all: restores everything via the plain,
		// unconditional linear walk — ResolveArchivePathBST is never even
		// attempted when len(includes) != 1 at ExtractWithRewriter's own
		// level (includes is empty here), so this path is untouched by any
		// of tonight's BST code.
	}
	t.Log("Restoring the WHOLE archive (no selection) as ground truth...")
	if err := RestoreSnapshotInline(fullOpts); err != nil {
		t.Fatalf("RestoreSnapshotInline (full, ground truth): %v", err)
	}
	groundTruthRoot := filepath.Join(destFull, filepath.Base(root), "target_folder")
	err = filepath.Walk(groundTruthRoot, func(path string, info os.FileInfo, werr error) error {
		if werr != nil || info.IsDir() {
			return werr
		}
		groundTruthBytes += info.Size()
		groundTruthFiles++
		return nil
	})
	if err != nil {
		t.Fatalf("walk ground truth restore: %v", err)
	}
	t.Logf("Ground truth (full restore, filtered to target_folder): %d files, %d bytes", groundTruthFiles, groundTruthBytes)

	if groundTruthFiles != targetFileCount {
		t.Errorf("ground truth file count (%d) doesn't match what we wrote (%d) — test setup issue, not a BST bug",
			groundTruthFiles, targetFileCount)
	}
	if groundTruthBytes != targetTotalBytes {
		t.Errorf("ground truth byte count (%d) doesn't match what we wrote (%d) — test setup issue, not a BST bug",
			groundTruthBytes, targetTotalBytes)
	}

	// --- Step 3: selective restore via the REAL RestoreSnapshotInline path,
	// exactly as the GUI does it (wrapper-prefixed IncludePaths) ---
	destSelective := t.TempDir()
	var lastBytesTotal uint64
	selOpts := RestoreOptions{
		Ctx:             context.Background(),
		BaseURL:         pbsOpts.BaseURL,
		AuthID:          pbsOpts.AuthID,
		Secret:          pbsOpts.Secret,
		Datastore:       pbsOpts.Datastore,
		CertFingerprint: pbsOpts.CertFingerprint,
		BackupID:        backupID,
		SnapshotTime:    snapTime,
		DestPath:        destSelective,
		IncludePaths:    []string{filepath.Base(root) + "/target_folder"},
		OnStats: func(s *RestoreProgressStats) {
			lastBytesTotal = s.BytesTotal
			t.Logf("OnStats: bytesDone=%d bytesTotal=%d", s.BytesDone, s.BytesTotal)
		},
	}
	t.Log("Restoring ONLY target_folder via the real RestoreSnapshotInline path...")
	if err := RestoreSnapshotInline(selOpts); err != nil {
		t.Fatalf("RestoreSnapshotInline (selective): %v", err)
	}

	var selectiveBytes int64
	var selectiveFiles int
	var leakedSiblingFiles int
	err = filepath.Walk(destSelective, func(path string, info os.FileInfo, werr error) error {
		if werr != nil || info.IsDir() {
			return werr
		}
		if filepath.Base(path) == BackupMetaFilename {
			return nil
		}
		if contains2(path, "sibling") {
			leakedSiblingFiles++
		}
		selectiveBytes += info.Size()
		selectiveFiles++
		return nil
	})
	if err != nil {
		t.Fatalf("walk selective restore: %v", err)
	}

	t.Logf("=== SUMMARY ===")
	t.Logf("Real target content:        %d files, %d bytes", targetFileCount, targetTotalBytes)
	t.Logf("Ground truth (full restore): %d files, %d bytes", groundTruthFiles, groundTruthBytes)
	t.Logf("BST span estimate:          %d bytes (vs real content %d — overhead ratio %.2f%%)",
		spanSize, targetTotalBytes, 100*float64(spanSize-targetTotalBytes)/float64(targetTotalBytes))
	t.Logf("Selective restore actual:   %d files, %d bytes, %d leaked sibling files", selectiveFiles, selectiveBytes, leakedSiblingFiles)
	t.Logf("Progress bar final total:   %d bytes", lastBytesTotal)

	if leakedSiblingFiles > 0 {
		t.Fatalf("FAIL: %d sibling files leaked into the selective restore output — real correctness bug, not just a progress display issue", leakedSiblingFiles)
	}
	if selectiveFiles != targetFileCount {
		t.Errorf("FAIL: selective restore wrote %d files, expected exactly %d (the real target content)", selectiveFiles, targetFileCount)
	}
	if selectiveBytes != targetTotalBytes {
		t.Errorf("FAIL: selective restore wrote %d bytes, expected exactly %d (the real target content)", selectiveBytes, targetTotalBytes)
	}
	if uint64(spanSize) != uint64(lastBytesTotal) && lastBytesTotal != 0 {
		t.Logf("NOTE: BST span (%d) and reported progress total (%d) differ — expected if extraction fell back to the linear walk for this archive", spanSize, lastBytesTotal)
	}

	// Checksum spot-check: every selectively-restored file's content must
	// match a freshly-computed hash of a few real sample files written
	// earlier — simplest robust check here is re-verifying against the
	// ground-truth full restore's own copies, file by file.
	mismatches := 0
	err = filepath.Walk(groundTruthRoot, func(path string, info os.FileInfo, werr error) error {
		if werr != nil || info.IsDir() {
			return werr
		}
		rel, _ := filepath.Rel(groundTruthRoot, path)
		// selective restore's own layout mirrors the same archive-relative
		// structure under destSelective/<wrapper>/target_folder/...
		selPath := filepath.Join(destSelective, filepath.Base(root), "target_folder", rel)
		a, aerr := sha256File2(path)
		b, berr2 := sha256File2(selPath)
		if aerr != nil || berr2 != nil {
			mismatches++
			t.Logf("MISMATCH (read error) %s: groundTruthErr=%v selectiveErr=%v", rel, aerr, berr2)
			return nil
		}
		if a != b {
			mismatches++
			t.Logf("MISMATCH (content) %s", rel)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk for checksum comparison: %v", err)
	}
	if mismatches > 0 {
		t.Fatalf("FAIL: %d file(s) differ (or missing) between ground-truth full restore and selective restore", mismatches)
	}

	// --- Step 4: read-ahead must stay inside the resolved span. Walk the
	// same span twice on fresh readers: once unbounded (the pre-fix
	// behaviour), once bounded with LimitPrefetchTo, and compare how many
	// chunks each actually fetched against the span's own chunk count.
	spanChunks := ra.ChunkCountInRange(targetStart, targetEnd)
	fetchedFor := func(bounded bool) int {
		r2, sz2, err := client.NewDIDXReaderAt(archiveName, 64, nil)
		if err != nil {
			t.Fatalf("NewDIDXReaderAt: %v", err)
		}
		pr := pbscommon.NewPXARReaderAt(r2, sz2)
		r2.SetPrefetchEnabled(false)
		s2, e2, _, rerr := pr.ResolveArchivePathBST("target_folder")
		r2.SetPrefetchEnabled(true)
		if rerr != nil {
			t.Fatalf("resolve: %v", rerr)
		}
		if bounded {
			r2.LimitPrefetchTo(e2)
		}
		_ = s2
		if _, err := pr.ExtractWithRewriter(func(p string) string {
			return filepath.Join(t.TempDir(), p)
		}, []string{"target_folder"}, true); err != nil {
			t.Fatalf("ExtractWithRewriter: %v", err)
		}
		time.Sleep(2 * time.Second) // let any in-flight background prefetch land
		return r2.Stats().ChunksFetched
	}
	unbounded := fetchedFor(false)
	bounded := fetchedFor(true)
	t.Logf("span chunks=%d, fetched unbounded=%d, fetched bounded=%d", spanChunks, unbounded, bounded)
	if bounded > spanChunks+2 {
		t.Errorf("FAIL: bounded read-ahead still fetched %d chunks for a %d-chunk span", bounded, spanChunks)
	}
	if unbounded <= bounded {
		t.Logf("NOTE: unbounded run did not over-fetch (%d vs %d); test data may be too small to show the overshoot", unbounded, bounded)
	}
	t.Log("PASS: selective restore content is byte-for-byte correct and complete, no sibling leakage")
}

func contains2(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return len(sub) == 0
}

func sha256File2(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}
