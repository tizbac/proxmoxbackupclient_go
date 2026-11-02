package pbscommon

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func otherCryptConfig(t *testing.T) *CryptConfig {
	t.Helper()
	c, err := NewCryptConfig(bytes.Repeat([]byte{5}, 32))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func manifestFor(archive, mode string, fp *string) *BackupManifest {
	return &BackupManifest{
		Files:       []File{{Filename: "catalog.pcat1.didx", CryptMode: CryptModeNone}, {Filename: archive, CryptMode: mode}},
		Unprotected: Unprotected{KeyFingerprint: fp},
	}
}

func TestArchiveKeyError(t *testing.T) {
	key := testCryptConfig(t)
	other := otherCryptConfig(t)
	fp := key.Fingerprint()
	upper := strings.ToUpper(fp)

	cases := []struct {
		name     string
		m        *BackupManifest
		archive  string
		crypt    *CryptConfig
		wantErr  bool
		snapKey  string
		givenKey string
	}{
		{"plain archive, no key", manifestFor("a.pxar.didx", CryptModeNone, nil), "a.pxar.didx", nil, false, "", ""},
		{"plain archive, a key loaded", manifestFor("a.pxar.didx", CryptModeNone, nil), "a.pxar.didx", other, false, "", ""},
		{"crypt-mode missing reads as plain", manifestFor("a.pxar.didx", "", nil), "a.pxar.didx", nil, false, "", ""},
		{"sign-only archive with another key", manifestFor("a.pxar.didx", CryptModeSignOnly, &fp), "a.pxar.didx", other, false, "", ""},
		{"encrypted, right key", manifestFor("a.pxar.didx", CryptModeEncrypt, &fp), "a.pxar.didx", key, false, "", ""},
		{"encrypted, right key in capitals", manifestFor("a.pxar.didx", CryptModeEncrypt, &upper), "a.pxar.didx", key, false, "", ""},
		{"encrypted, another key", manifestFor("a.pxar.didx", CryptModeEncrypt, &fp), "a.pxar.didx", other, true, fp, other.Fingerprint()},
		{"encrypted, no key", manifestFor("a.pxar.didx", CryptModeEncrypt, &fp), "a.pxar.didx", nil, true, fp, ""},
		{"encrypted, no key, manifest names none", manifestFor("a.pxar.didx", CryptModeEncrypt, nil), "a.pxar.didx", nil, true, "", ""},
		{"encrypted, manifest names no key, a key loaded", manifestFor("a.pxar.didx", CryptModeEncrypt, nil), "a.pxar.didx", other, false, "", ""},
		{"archive not in the manifest", manifestFor("a.pxar.didx", CryptModeEncrypt, &fp), "b.pxar.didx", other, false, "", ""},
		{"plain catalog of an encrypted snapshot", manifestFor("a.pxar.didx", CryptModeEncrypt, &fp), "catalog.pcat1.didx", other, false, "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := archiveKeyError(c.m, c.archive, c.crypt)
			if !c.wantErr {
				if err != nil {
					t.Fatalf("got %v, want nil", err)
				}
				return
			}
			var ke *SnapshotKeyError
			if !errors.As(err, &ke) {
				t.Fatalf("got %v, want a *SnapshotKeyError", err)
			}
			if ke.SnapshotKey != c.snapKey || ke.GivenKey != c.givenKey || ke.Archive != c.archive {
				t.Fatalf("got %+v, want snapshot key %q, given %q", ke, c.snapKey, c.givenKey)
			}
		})
	}
}

func TestSnapshotKeyErrorMessages(t *testing.T) {
	cases := []struct {
		e    SnapshotKeyError
		want []string
	}{
		{SnapshotKeyError{SnapshotKey: "aa:bb", GivenKey: "cc:dd"}, []string{"another key", "fingerprint aa:bb", "in use has fingerprint cc:dd"}},
		{SnapshotKeyError{SnapshotKey: "aa:bb"}, []string{"is encrypted", "fingerprint aa:bb", "no key is in use"}},
		{SnapshotKeyError{}, []string{"is encrypted", "no key is in use"}},
	}
	for _, c := range cases {
		msg := c.e.Error()
		for _, w := range c.want {
			if !strings.Contains(msg, w) {
				t.Errorf("%q does not contain %q", msg, w)
			}
		}
	}
}

// The reader refuses an encrypted archive with the wrong key before it
// fetches a single chunk, and names both keys.
func TestDIDXReaderAtWrongKeyRefusedFirst(t *testing.T) {
	key := testCryptConfig(t)
	a := makeDIDX(t, key, payloadsOf(3, 64)...)
	f := newFakePBSWithDIDX(t, "enc.didx", a, key)
	fp := key.Fingerprint()
	f.setManifest(t, *manifestFor("enc.didx", CryptModeEncrypt, &fp))

	f.Crypt = otherCryptConfig(t)
	_, _, err := f.NewDIDXReaderAt("enc.didx", 4, nil)
	var ke *SnapshotKeyError
	if !errors.As(err, &ke) {
		t.Fatalf("got %v, want a *SnapshotKeyError", err)
	}
	if ke.SnapshotKey != fp || ke.GivenKey != f.Crypt.Fingerprint() {
		t.Fatalf("got %+v", ke)
	}
	if n := f.totalFetches(); n != 0 {
		t.Fatalf("%d chunks fetched before the key was refused", n)
	}

	f.Crypt = nil
	if _, _, err := f.NewDIDXReaderAt("enc.didx", 4, nil); !errors.As(err, &ke) || ke.GivenKey != "" || ke.SnapshotKey != fp {
		t.Fatalf("no key: got %v", err)
	}

	// The right key opens it and reads as before.
	f.Crypt = key
	r, _, err := f.NewDIDXReaderAt("enc.didx", 4, nil)
	if err != nil {
		t.Fatalf("right key: %v", err)
	}
	if _, err := r.ReadAt(make([]byte, 8), 0); err != nil {
		t.Fatalf("right key, read: %v", err)
	}
}

// A disk image (fixed index) is checked the same way, before its index is
// even downloaded.
func TestFIDXReaderAtWrongKeyRefusedFirst(t *testing.T) {
	key := testCryptConfig(t)
	a := makeDIDX(t, key, payloadsOf(1, 64)...)
	f := newFakePBSWithDIDX(t, "drive-scsi0.img.fidx", a, key)
	fp := key.Fingerprint()
	f.setManifest(t, *manifestFor("drive-scsi0.img.fidx", CryptModeEncrypt, &fp))
	f.Crypt = otherCryptConfig(t)
	_, _, err := f.NewFIDXReaderAt("drive-scsi0.img.fidx", 4, nil)
	var ke *SnapshotKeyError
	if !errors.As(err, &ke) || ke.SnapshotKey != fp {
		t.Fatalf("got %v, want a *SnapshotKeyError", err)
	}
}

// A manifest that cannot be read does not block the restore: it goes ahead
// and the chunks decide, as before the check existed.
func TestCheckArchiveKeyUnreadableManifest(t *testing.T) {
	key := testCryptConfig(t)
	a := makeDIDX(t, key, payloadsOf(1, 64)...)
	f := newFakePBSWithDIDX(t, "enc.didx", a, key)
	f.Crypt = otherCryptConfig(t)
	if err := f.CheckArchiveKey("enc.didx"); err != nil {
		t.Fatalf("missing manifest: got %v", err)
	}
	f.manifest = mustEncodeBlob(t, []byte("not json"), nil)
	if err := f.CheckArchiveKey("enc.didx"); err != nil {
		t.Fatalf("garbled manifest: got %v", err)
	}
}
