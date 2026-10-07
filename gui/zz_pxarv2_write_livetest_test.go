package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"pbscommon"
)

// Opt-in (PBS_PXARV2=1): back up the e2e fixture tree in "data"
// change-detection mode (split .mpxar + .ppxar archive) three times (full,
// after mutations, unchanged), then restore every snapshot with our reader
// and compare it with the exact source state at backup time.
//
// For the cross-check with the OFFICIAL client, set PBS_PXARV2_KEEP=<dir>:
// the test then writes <dir>/snapshots.txt (backup id + times + archive name)
// and <dir>/<time>.sha256 (sha256sum-format manifest of the source at that
// backup), so the official client can restore the same snapshots and be
// compared against the same hashes.
func TestPXARv2WriteDataMode(t *testing.T) {
	if os.Getenv("PBS_PXARV2") == "" {
		t.Skip("set PBS_PXARV2=1")
	}
	base := t.TempDir()
	src := filepath.Join(base, "src")
	e2eBuildTree(t, src)
	backupID := fmt.Sprintf("pxarv2-write-%d", time.Now().Unix())
	keep := os.Getenv("PBS_PXARV2_KEEP")
	t.Logf("backup id %s", backupID)

	type snap struct {
		unix  int64
		state map[string]e2eFile
		label string
	}
	var snaps []snap
	doBackup := func(label string) {
		var rs *BackupStatus
		err := RunBackupInline(BackupOptions{
			BaseURL: e2eBaseURL, AuthID: e2eAuthID, Secret: e2eSecret, Datastore: e2eStore, CertFingerprint: e2eFP,
			BackupObjects: []string{src}, BackupID: backupID, BackupType: "host", Kind: "directory",
			Compression: "fastest", ChangeDetectionMode: pbscommon.ChangeDetectionData,
			OnResult: func(s *BackupStatus) { rs = s },
		})
		if err != nil {
			t.Fatalf("%s: backup failed: %v", label, err)
		}
		if rs == nil || !rs.Success() || rs.FailedChunks != 0 || len(rs.SkippedReadError) != 0 {
			t.Fatalf("%s: bad result %+v", label, rs)
		}
		t.Logf("%s: bytes=%d new chunks=%d reused=%d", label, rs.TotalBytes, rs.NewChunks, rs.ReusedChunks)
		snaps = append(snaps, snap{unix: rs.BackupTime, state: e2eSnapshotState(t, src), label: label})
		if keep != "" {
			writeShaManifest(t, src, filepath.Join(keep, fmt.Sprintf("%d.sha256", rs.BackupTime)))
		}
	}

	doBackup("data 1 full")
	time.Sleep(1100 * time.Millisecond)
	e2eMutate(t, src)
	doBackup("data 2 after changes")
	time.Sleep(1100 * time.Millisecond)
	doBackup("data 3 unchanged")

	// The manifest must hold the split pair and no classic archive.
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

	for _, s := range snaps {
		for _, parallel := range []bool{false, true} {
			label := fmt.Sprintf("%s restore parallel=%v", s.label, parallel)
			dest := filepath.Join(base, fmt.Sprintf("restore-%d-%v", s.unix, parallel))
			os.MkdirAll(dest, 0755)
			var verified int
			var fails []string
			o := ropts
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

// writeShaManifest writes `sha256sum`-format lines ("<hex>  ./<rel>") for every
// regular file under root, sorted, so `sha256sum -c` style tools can use it.
func writeShaManifest(t *testing.T, root, out string) {
	t.Helper()
	var lines []string
	filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || !info.Mode().IsRegular() {
			return nil
		}
		f, ferr := os.Open(p)
		if ferr != nil {
			return nil
		}
		h := sha256.New()
		io.Copy(h, f)
		f.Close()
		rel, _ := filepath.Rel(root, p)
		lines = append(lines, hex.EncodeToString(h.Sum(nil))+"  ./"+filepath.ToSlash(rel))
		return nil
	})
	sort.Strings(lines)
	os.MkdirAll(filepath.Dir(out), 0755)
	os.WriteFile(out, []byte(strings.Join(lines, "\n")+"\n"), 0644)
}
