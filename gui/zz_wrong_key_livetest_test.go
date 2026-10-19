//go:build !service

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"pbscommon"
)

// Opt-in end-to-end test (PBS_E2E=1, the test server of
// zz_e2e_directory_livetest_test.go): an encrypted snapshot restored or
// browsed with another key, or with none, is refused before any data is read,
// naming the snapshot's key; the right key still restores it.
func TestWrongKeyRestoreLive(t *testing.T) {
	if os.Getenv("PBS_E2E") == "" {
		t.Skip("set PBS_E2E=1")
	}
	base := t.TempDir()
	key, err := pbscommon.GenerateEncryptionKey()
	if err != nil {
		t.Fatal(err)
	}
	crypt, err := pbscommon.NewCryptConfig(key)
	if err != nil {
		t.Fatal(err)
	}
	other, err := pbscommon.GenerateEncryptionKey()
	if err != nil {
		t.Fatal(err)
	}
	otherCrypt, err := pbscommon.NewCryptConfig(other)
	if err != nil {
		t.Fatal(err)
	}

	src := filepath.Join(base, "src")
	e2eBuildTree(t, src)
	want := e2eSnapshotState(t, src)
	backupID := fmt.Sprintf("wrongkey-%d", time.Now().Unix())
	var rs *BackupStatus
	err = RunBackupInline(BackupOptions{
		BaseURL: e2eBaseURL, AuthID: e2eAuthID, Secret: e2eSecret, Datastore: e2eStore, CertFingerprint: e2eFP,
		BackupObjects: []string{src}, BackupID: backupID, BackupType: "host", Kind: "directory",
		Compression: "fastest", Crypt: crypt,
		OnResult: func(s *BackupStatus) { rs = s },
	})
	if err != nil || rs == nil || !rs.Success() {
		t.Fatalf("encrypted backup: %v %+v", err, rs)
	}
	t.Logf("encrypted backup host/%s, key %s", backupID, crypt.Fingerprint())

	opts := func(dest string, c *pbscommon.CryptConfig) RestoreOptions {
		return RestoreOptions{
			BaseURL: e2eBaseURL, AuthID: e2eAuthID, Secret: e2eSecret, Datastore: e2eStore, CertFingerprint: e2eFP,
			BackupID: backupID, SnapshotTime: time.Unix(rs.BackupTime, 0).UTC(), DestPath: dest,
			Mode: RestoreModeAlternateAbs, Overwrite: true, VerifyAfterRestore: true, Crypt: c,
		}
	}
	refused := func(label string, c *pbscommon.CryptConfig, err error) {
		t.Helper()
		var ke *pbscommon.SnapshotKeyError
		if !errors.As(err, &ke) {
			t.Fatalf("%s: got %v, want a *pbscommon.SnapshotKeyError", label, err)
		}
		given := ""
		if c != nil {
			given = c.Fingerprint()
		}
		if ke.SnapshotKey != crypt.Fingerprint() || ke.GivenKey != given {
			t.Fatalf("%s: got %+v, want snapshot key %s, given %q", label, ke, crypt.Fingerprint(), given)
		}
		t.Logf("%s: %v", label, err)
	}
	empty := func(label, dir string) {
		t.Helper()
		var files []string
		filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				files = append(files, p)
			}
			return nil
		})
		if len(files) > 0 {
			t.Fatalf("%s: files written: %v", label, files)
		}
	}

	for _, c := range []struct {
		label string
		crypt *pbscommon.CryptConfig
	}{{"restore with another key", otherCrypt}, {"restore with no key", nil}} {
		dest := filepath.Join(base, c.label)
		if err := os.MkdirAll(dest, 0o755); err != nil {
			t.Fatal(err)
		}
		refused(c.label, c.crypt, RestoreSnapshotInline(opts(dest, c.crypt)))
		empty(c.label, dest)
	}

	_, err = ListSnapshotContentsInline(opts("", otherCrypt), "", true)
	refused("browse with another key", otherCrypt, err)

	dest := filepath.Join(base, "restore-right-key")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := RestoreSnapshotInline(opts(dest, crypt)); err != nil {
		t.Fatalf("restore with the right key: %v", err)
	}
	e2eCompare(t, "restore with the right key", want, e2eSnapshotState(t, e2eFindRoot(t, dest)), "")
	t.Logf("restored with the right key: %d entries compared", len(want))
}
