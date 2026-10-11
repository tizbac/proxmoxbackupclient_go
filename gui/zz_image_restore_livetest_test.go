package main

import (
	"os"
	"path/filepath"
	"pbscommon"
	"strings"
	"testing"
	"time"
)

// Opt-in (PBS_IMG_URL set, same variables as TestLiveImageBrowse): lists a
// machine backup through the restore code (ListSnapshotContentsInline) and
// restores Windows/System32/drivers/etc and Windows/explorer.exe from its
// Windows volume to a temp folder, with verify, in both alternate modes.
func TestLiveImageRestore(t *testing.T) {
	url := os.Getenv("PBS_IMG_URL")
	if url == "" {
		t.Skip("set PBS_IMG_URL and friends")
	}
	base := RestoreOptions{
		BaseURL: url, CertFingerprint: os.Getenv("PBS_IMG_FP"), AuthID: os.Getenv("PBS_IMG_AUTHID"),
		Secret: os.Getenv("PBS_IMG_SECRET"), Datastore: os.Getenv("PBS_IMG_STORE"), BackupID: os.Getenv("PBS_IMG_ID"),
	}
	snaps, err := ListSnapshotsInline(base.BaseURL, base.AuthID, base.Secret, "", "", base.Datastore, "", base.CertFingerprint, base.BackupID)
	if err != nil {
		t.Fatal(err)
	}
	var newest time.Time
	for _, s := range snaps {
		if s.BackupID == base.BackupID && s.BackupTime.After(newest) {
			newest = s.BackupTime
		}
	}
	base.SnapshotTime = newest

	start := time.Now()
	entries, err := ListSnapshotContentsInline(base, "", true)
	if err != nil {
		t.Fatal(err)
	}
	var winVol string
	for _, e := range entries {
		if e.IsDir && !strings.Contains(e.Path, "/") {
			t.Logf("volume %q", e.Path)
		}
		if strings.HasSuffix(e.Path, "/Windows/explorer.exe") {
			winVol = strings.TrimSuffix(e.Path, "/Windows/explorer.exe")
		}
	}
	t.Logf("listed %d entries in %v", len(entries), time.Since(start).Round(time.Millisecond))
	// PBS_IMG_PICK: restore that folder of the first volume instead of the
	// Windows files (a data disk), in both modes, and check the verify.
	if pick := os.Getenv("PBS_IMG_PICK"); pick != "" {
		vol := ""
		for _, e := range entries {
			if e.IsDir && !strings.Contains(e.Path, "/") {
				vol = e.Path
				break
			}
		}
		for _, mode := range []RestoreMode{RestoreModeAlternateAbs, RestoreModeAlternateFlat} {
			opts := base
			opts.DestPath = t.TempDir()
			opts.Mode = mode
			opts.VerifyAfterRestore = true
			opts.RestoreACLs = true
			opts.IncludePaths = []string{vol + "/" + pick}
			var files, verified int
			var bytes int64
			opts.OnSummary = func(f, d int, b int64) { files, bytes = f, b }
			opts.OnVerifySummary = func(v int, fails []string) { verified = v }
			start := time.Now()
			if err := RestoreSnapshotInline(opts); err != nil {
				t.Fatalf("%s: %v", mode, err)
			}
			t.Logf("%s: %s: %d files, %d bytes, %d verified in %v", mode, pick, files, bytes, verified, time.Since(start).Round(time.Millisecond))
			if files == 0 || verified != files {
				t.Errorf("%s: verified %d of %d", mode, verified, files)
			}
		}
		return
	}
	if winVol == "" {
		t.Fatal("no Windows volume")
	}

	for _, mode := range []RestoreMode{RestoreModeAlternateAbs, RestoreModeAlternateFlat} {
		dest := t.TempDir()
		opts := base
		opts.DestPath = dest
		opts.Mode = mode
		opts.VerifyAfterRestore = true
		opts.RestoreACLs = true
		opts.IncludePaths = []string{winVol + "/Windows/System32/drivers/etc", winVol + "/Windows/explorer.exe"}
		var files, dirs int
		var bytes int64
		var verified int
		opts.OnSummary = func(f, d int, b int64) { files, dirs, bytes = f, d, b }
		opts.OnVerifySummary = func(v int, fails []string) { verified = v }
		var extracted []pbscommon.PXARExtractedFile
		opts.OnExtracted = func(x []pbscommon.PXARExtractedFile) { extracted = x }
		start := time.Now()
		if err := RestoreSnapshotInline(opts); err != nil {
			t.Fatalf("%s: %v", mode, err)
		}
		t.Logf("%s: %d files, %d folders, %d bytes, %d verified in %v", mode, files, dirs, bytes, verified, time.Since(start).Round(time.Millisecond))
		if verified != files || files < 3 {
			t.Errorf("%s: verified %d of %d", mode, verified, files)
		}
		want := filepath.Join(dest, winVol, "Windows", "explorer.exe")
		if mode == RestoreModeAlternateFlat {
			want = filepath.Join(dest, "explorer.exe") // both picks share "<volume>/Windows"... minus the volume
		}
		var found bool
		for _, x := range extracted {
			if strings.EqualFold(filepath.Base(x.Path), "explorer.exe") {
				found = true
				st, err := os.Stat(x.Path)
				if err != nil {
					t.Fatal(err)
				}
				t.Logf("  %s: %d bytes, modified %s", x.Path, st.Size(), st.ModTime().UTC().Format(time.RFC3339))
				if st.ModTime().Unix() != x.ModTime {
					t.Errorf("mtime %d, want %d", st.ModTime().Unix(), x.ModTime)
				}
			}
		}
		if !found {
			t.Errorf("%s: explorer.exe not restored (expected near %s)", mode, want)
		}
		hosts := filepath.Join(dest, winVol, "Windows", "System32", "drivers", "etc", "hosts")
		if mode == RestoreModeAlternateFlat {
			hosts = filepath.Join(dest, "System32", "drivers", "etc", "hosts")
		}
		if _, err := os.Stat(hosts); err != nil {
			t.Errorf("%s: hosts file: %v", mode, err)
		}
	}
}
