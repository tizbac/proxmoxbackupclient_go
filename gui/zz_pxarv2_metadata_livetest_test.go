package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"pbscommon"
)

// metaSnap is one snapshot taken by the metadata-mode live tests.
type metaSnap struct {
	unix  int64
	state map[string]e2eFile
	label string
	crypt *pbscommon.CryptConfig
	stats *pbscommon.ReuseStats
}

// metaBackup runs one metadata-mode backup of src and returns the snapshot
// with the source state and the reuse statistics (nil when nothing could be
// reused from a previous snapshot).
func metaBackup(t *testing.T, label, src, backupID string, crypt *pbscommon.CryptConfig) metaSnap {
	t.Helper()
	var rs *BackupStatus
	var st *pbscommon.ReuseStats
	reuseStatsHook = func(_ string, s pbscommon.ReuseStats) { st = &s }
	defer func() { reuseStatsHook = nil }()
	start := time.Now()
	err := RunBackupInline(BackupOptions{
		BaseURL: e2eBaseURL, AuthID: e2eAuthID, Secret: e2eSecret, Datastore: e2eStore, CertFingerprint: e2eFP,
		BackupObjects: []string{src}, BackupID: backupID, BackupType: "host", Kind: "directory",
		Compression: "fastest", ChangeDetectionMode: pbscommon.ChangeDetectionMetadata, Crypt: crypt,
		OnResult: func(s *BackupStatus) { rs = s },
	})
	if err != nil {
		t.Fatalf("%s: backup failed: %v", label, err)
	}
	if rs == nil || !rs.Success() || rs.FailedChunks != 0 || len(rs.SkippedReadError) != 0 {
		t.Fatalf("%s: bad result %+v", label, rs)
	}
	if st != nil {
		t.Logf("%s: %s, new chunks %d, reused chunks %d; reused %d files (%d bytes, %d chunks, padding %d, %d over-padding)",
			label, time.Since(start).Round(time.Millisecond), rs.NewChunks, rs.ReusedChunks,
			st.Files, st.Bytes, st.Chunks, st.Padding, st.OverPadding)
	} else {
		t.Logf("%s: %s, new chunks %d, reused chunks %d; no reuse", label, time.Since(start).Round(time.Millisecond), rs.NewChunks, rs.ReusedChunks)
	}
	s := metaSnap{unix: rs.BackupTime, state: e2eSnapshotState(t, src), label: label, crypt: crypt, stats: st}
	time.Sleep(1100 * time.Millisecond) // next snapshot needs a later backup time
	return s
}

// restoreAndCompare restores every snapshot (sequential and parallel) with
// its own key and compares it with the source state at backup time.
func restoreAndCompare(t *testing.T, backupID, base string, snaps []metaSnap) {
	t.Helper()
	ropts := RestoreOptions{BaseURL: e2eBaseURL, AuthID: e2eAuthID, Secret: e2eSecret, Datastore: e2eStore, CertFingerprint: e2eFP, BackupID: backupID}
	for _, s := range snaps {
		for _, parallel := range []bool{false, true} {
			label := fmt.Sprintf("%s restore parallel=%v", s.label, parallel)
			dest := filepath.Join(base, fmt.Sprintf("restore-%d-%v", s.unix, parallel))
			os.MkdirAll(dest, 0755)
			var verified int
			var fails []string
			o := ropts
			o.Crypt = s.crypt
			o.SnapshotTime = time.Unix(s.unix, 0).UTC()
			o.DestPath, o.Mode, o.Overwrite, o.ParallelExtraction = dest, RestoreModeAlternateAbs, true, parallel
			o.VerifyAfterRestore = true
			o.OnVerifySummary = func(v int, f []string) { verified, fails = v, f }
			if err := RestoreSnapshotInline(o); err != nil {
				t.Errorf("%s: %v", label, err)
				continue
			}
			if verified == 0 || len(fails) > 0 {
				t.Errorf("%s: verify-after-restore checked %d files, mismatches %v", label, verified, fails)
			}
			e2eCompare(t, label, s.state, e2eSnapshotState(t, e2eFindRoot(t, dest)), "")
			t.Logf("%s: OK, %d files verified", label, verified)
		}
	}
}

func countRegularFiles(root string) uint64 {
	var n uint64
	filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err == nil && info.Mode().IsRegular() {
			n++
		}
		return nil
	})
	return n
}

// Opt-in (PBS_PXARV2=1): metadata change detection against the test PBS.
// Four backups of the e2e tree in "metadata" mode: the first has no previous
// split snapshot (so every file is read), the second follows mutations, the
// third and fourth follow nothing (the fourth reuses from a snapshot that was
// itself reused, padding and all). Every snapshot is restored by our reader
// and compared with the source state at backup time; the reuse statistics
// must show unchanged files were not read.
//
// PBS_PXARV2_KEEP=<dir> writes sha256 manifests and snapshots.txt for the
// cross-check with the official client, as TestPXARv2WriteDataMode does.
func TestPXARv2MetadataMode(t *testing.T) {
	if os.Getenv("PBS_PXARV2") == "" {
		t.Skip("set PBS_PXARV2=1")
	}
	base := t.TempDir()
	src := filepath.Join(base, "src")
	e2eBuildTree(t, src)
	backupID := fmt.Sprintf("pxarv2-meta-%d", time.Now().Unix())
	keep := os.Getenv("PBS_PXARV2_KEEP")
	t.Logf("backup id %s", backupID)

	var snaps []metaSnap
	add := func(s metaSnap) metaSnap {
		snaps = append(snaps, s)
		if keep != "" {
			writeShaManifest(t, src, filepath.Join(keep, fmt.Sprintf("%d.sha256", s.unix)))
		}
		return s
	}

	s1 := add(metaBackup(t, "meta 1 first", src, backupID, nil))
	if s1.stats != nil {
		t.Errorf("first backup reused files with no previous snapshot: %+v", *s1.stats)
	}
	e2eMutate(t, src)
	s2 := add(metaBackup(t, "meta 2 after changes", src, backupID, nil))
	// e2eMutate changes the big file and f4096.bin, renames one file and
	// adds two: every other file must be reused.
	if s2.stats == nil || s2.stats.Files+s2.stats.OverPadding < countRegularFiles(src)-5 {
		t.Errorf("meta 2: reuse stats %+v, %d files in the tree", s2.stats, countRegularFiles(src))
	}
	s3 := add(metaBackup(t, "meta 3 unchanged", src, backupID, nil))
	s4 := add(metaBackup(t, "meta 4 unchanged again", src, backupID, nil))
	for _, s := range []metaSnap{s3, s4} {
		if s.stats == nil || s.stats.Files+s.stats.OverPadding != countRegularFiles(src) {
			t.Errorf("%s: nothing changed but stats %+v for %d files", s.label, s.stats, countRegularFiles(src))
		}
	}

	ropts := RestoreOptions{BaseURL: e2eBaseURL, AuthID: e2eAuthID, Secret: e2eSecret, Datastore: e2eStore, CertFingerprint: e2eFP, BackupID: backupID}
	var archive string
	for _, s := range snaps {
		o := ropts
		o.SnapshotTime = time.Unix(s.unix, 0).UTC()
		names, err := resolveArchiveNames(o)
		if err != nil || len(names) != 1 || !strings.HasSuffix(names[0], ".mpxar.didx") {
			t.Fatalf("%s: data archives %v (err %v), want one .mpxar.didx", s.label, names, err)
		}
		archive = names[0]
	}
	if keep != "" {
		var lines []string
		for _, s := range snaps {
			lines = append(lines, fmt.Sprintf("%s %d %s", backupID, s.unix, archive))
		}
		os.WriteFile(filepath.Join(keep, "snapshots.txt"), []byte(strings.Join(lines, "\n")+"\n"), 0644)
	}
	restoreAndCompare(t, backupID, base, snaps)
}

// Metadata mode with encryption: reuse only from a snapshot encrypted with
// the same key; a different key, or no key at all, must read every file
// again, since reused chunks are not re-encrypted.
func TestPXARv2MetadataModeEncrypted(t *testing.T) {
	if os.Getenv("PBS_PXARV2") == "" {
		t.Skip("set PBS_PXARV2=1")
	}
	newKey := func() *pbscommon.CryptConfig {
		k, err := pbscommon.GenerateEncryptionKey()
		if err != nil {
			t.Fatal(err)
		}
		c, err := pbscommon.NewCryptConfig(k)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	keyA, keyB := newKey(), newKey()
	base := t.TempDir()
	src := filepath.Join(base, "src")
	e2eWriteBytes(t, filepath.Join(src, "e2e-marker.txt"), []byte("marker"))
	for i := 0; i < 60; i++ {
		e2eWriteRandom(t, filepath.Join(src, fmt.Sprintf("d%d", i%4), fmt.Sprintf("f%02d.bin", i)), int64(1000+i*5000))
	}
	e2eWriteRandom(t, filepath.Join(src, "big.bin"), 12*1024*1024)
	backupID := fmt.Sprintf("pxarv2-meta-crypt-%d", time.Now().Unix())
	t.Logf("backup id %s", backupID)

	var snaps []metaSnap
	for _, c := range []struct {
		label     string
		key       *pbscommon.CryptConfig
		wantReuse bool
	}{
		{"key A first", keyA, false},
		{"key A again", keyA, true},
		{"key B", keyB, false},
		{"key B again", keyB, true},
		{"no key", nil, false},
		{"no key again", nil, true},
		{"key A after no key", keyA, false},
	} {
		s := metaBackup(t, c.label, src, backupID, c.key)
		if got := s.stats != nil && s.stats.Files > 0; got != c.wantReuse {
			t.Errorf("%s: reused files = %v, want %v", c.label, got, c.wantReuse)
		}
		snaps = append(snaps, s)
	}
	restoreAndCompare(t, backupID, base, snaps)
}
