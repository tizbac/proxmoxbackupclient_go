//go:build !service

package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"pbscommon"
)

// Opt-in end-to-end test of getting a key back from paper (PBS_E2E=1, the
// test server of zz_e2e_directory_livetest_test.go): a new key, its paper
// key, a photo of that page read back with Import, an encrypted backup made
// with the original key, the original lost, and the snapshot restored with
// the imported key and compared with the source. A different key must not
// restore it.
func TestKeyImportRestoreLive(t *testing.T) {
	if os.Getenv("PBS_E2E") == "" {
		t.Skip("set PBS_E2E=1")
	}
	a := &App{}
	base := t.TempDir()
	orig := filepath.Join(base, "orig", "encryption-key.json")
	if _, err := a.GenerateEncryptionKeyFile(orig); err != nil {
		t.Fatal(err)
	}
	origText, _ := os.ReadFile(orig)

	// The paper key's QR code, photographed.
	pk, err := a.EncryptionPaperKey(orig)
	if err != nil {
		t.Fatal(err)
	}
	qrPNG, err := base64.StdEncoding.DecodeString(pk.QRPNG)
	if err != nil {
		t.Fatal(err)
	}
	qr, err := png.Decode(bytes.NewReader(qrPNG))
	if err != nil {
		t.Fatal(err)
	}
	ki, err := keyImportFromImageBytes(encodeJPEG(t, photo(qr, 3000, 4000, 1300, 7), 80))
	if err != nil || ki.Problem != "" || !ki.Checked || ki.Fingerprint != pk.Fingerprint {
		t.Fatalf("reading the photo: err %v %+v (want fingerprint %s)", err, ki, pk.Fingerprint)
	}
	imported, existed, err := saveImportedKey(filepath.Join(base, "keys"), ki.KeyText, "")
	if err != nil || existed {
		t.Fatalf("saving: %v (existed %v)", err, existed)
	}
	if got, _ := os.ReadFile(imported); !bytes.Equal(got, origText) {
		t.Fatalf("the imported key file differs from the original:\n%s\n%s", got, origText)
	}
	t.Logf("key %s read back from a photo of its paper key into %s, byte for byte", pk.Fingerprint, imported)

	cryptOrig, err := pbscommon.LoadKeyFile(orig, nil)
	if err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(base, "src")
	e2eBuildTree(t, src)
	want := e2eSnapshotState(t, src)
	backupID := fmt.Sprintf("keyimport-%d", time.Now().Unix())
	var rs *BackupStatus
	err = RunBackupInline(BackupOptions{
		BaseURL: e2eBaseURL, AuthID: e2eAuthID, Secret: e2eSecret, Datastore: e2eStore, CertFingerprint: e2eFP,
		BackupObjects: []string{src}, BackupID: backupID, BackupType: "host", Kind: "directory",
		Compression: "fastest", Crypt: cryptOrig,
		OnResult: func(s *BackupStatus) { rs = s },
	})
	if err != nil || rs == nil || !rs.Success() {
		t.Fatalf("encrypted backup: %v %+v", err, rs)
	}
	t.Logf("encrypted backup host/%s made with the original key (%d bytes)", backupID, rs.TotalBytes)
	if err := os.Remove(orig); err != nil { // lost with the computer
		t.Fatal(err)
	}

	restore := func(label string, crypt *pbscommon.CryptConfig) (string, error) {
		dest := filepath.Join(base, label)
		if err := os.MkdirAll(dest, 0o755); err != nil {
			t.Fatal(err)
		}
		var verified int
		var fails []string
		err := RestoreSnapshotInline(RestoreOptions{
			BaseURL: e2eBaseURL, AuthID: e2eAuthID, Secret: e2eSecret, Datastore: e2eStore, CertFingerprint: e2eFP,
			BackupID: backupID, SnapshotTime: time.Unix(rs.BackupTime, 0).UTC(), DestPath: dest,
			Mode: RestoreModeAlternateAbs, Overwrite: true, VerifyAfterRestore: true, Crypt: crypt,
			OnVerifySummary: func(v int, f []string) { verified, fails = v, f },
		})
		if err == nil && (verified == 0 || len(fails) > 0) {
			err = fmt.Errorf("verify after restore: %d files, mismatches %v", verified, fails)
		}
		return dest, err
	}

	other, err := pbscommon.GenerateEncryptionKey()
	if err != nil {
		t.Fatal(err)
	}
	otherCrypt, err := pbscommon.NewCryptConfig(other)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := restore("restore-wrong-key", otherCrypt); err == nil {
		t.Fatal("a different key restored the snapshot")
	} else {
		t.Logf("a different key is refused: %v", err)
	}

	cryptImported, err := pbscommon.LoadKeyFile(imported, nil)
	if err != nil {
		t.Fatal(err)
	}
	dest, err := restore("restore-imported-key", cryptImported)
	if err != nil {
		t.Fatalf("restore with the imported key: %v", err)
	}
	e2eCompare(t, "restore with the imported key", want, e2eSnapshotState(t, e2eFindRoot(t, dest)), "")
	t.Logf("restored with the imported key: %d entries compared", len(want))
}
