package main

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// Opt-in (PBS_CLI_ID = the backup ID of a directory CLI snapshot on the e2e
// server, made from a folder holding note1.txt created 2019-03-04 05:06:07
// local time and a read-only readonly.txt): restores the newest snapshot with
// permissions and checks that the CLI recorded the creation time and the
// attributes (they come from the NTFS metadata collector the CLI now shares
// with the GUI).
func TestLiveCLIMetaRestore(t *testing.T) {
	id := os.Getenv("PBS_CLI_ID")
	if id == "" {
		t.Skip("set PBS_CLI_ID")
	}
	snaps, err := ListSnapshotsInline(e2eBaseURL, e2eAuthID, e2eSecret, "", "", e2eStore, "", e2eFP, id)
	if err != nil {
		t.Fatal(err)
	}
	var newest time.Time
	for _, s := range snaps {
		if s.BackupID == id && s.BackupTime.After(newest) {
			newest = s.BackupTime
		}
	}
	if newest.IsZero() {
		t.Fatalf("no snapshot for %s", id)
	}
	dest := t.TempDir()
	opts := RestoreOptions{
		BaseURL: e2eBaseURL, CertFingerprint: e2eFP, AuthID: e2eAuthID, Secret: e2eSecret, Datastore: e2eStore,
		BackupID: id, SnapshotTime: newest, DestPath: dest, Mode: RestoreModeAlternateFlat,
		RestoreACLs: true, VerifyAfterRestore: true,
	}
	var files, verified int
	opts.OnSummary = func(f, d int, b int64) { files = f }
	opts.OnVerifySummary = func(v int, fails []string) { verified = v }
	if err := RestoreSnapshotInline(opts); err != nil {
		t.Fatal(err)
	}
	t.Logf("restored %d files, %d verified", files, verified)
	find := func(name string) string {
		var hit string
		filepath.Walk(dest, func(p string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() && info.Name() == name {
				hit = p
			}
			return nil
		})
		if hit == "" {
			t.Fatalf("%s not restored", name)
		}
		return hit
	}
	note := find("note1.txt")
	st, err := os.Stat(note)
	if err != nil {
		t.Fatal(err)
	}
	created := time.Unix(0, st.Sys().(*syscall.Win32FileAttributeData).CreationTime.Nanoseconds())
	t.Logf("note1.txt created %s", created.Format(time.RFC3339))
	if created.Year() != 2019 {
		t.Errorf("note1.txt creation time %s, want 2019 (not recorded by the CLI?)", created)
	}
	ro := find("readonly.txt")
	a, err := windows.GetFileAttributes(windows.StringToUTF16Ptr(ro))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("readonly.txt attributes %#x", a)
	if a&windows.FILE_ATTRIBUTE_READONLY == 0 {
		t.Error("readonly.txt lost its read-only attribute")
	}
	if verified != files || files == 0 {
		t.Errorf("verified %d of %d", verified, files)
	}
}
