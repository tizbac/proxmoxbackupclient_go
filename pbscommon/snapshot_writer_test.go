package pbscommon

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"
)

// storedManifest uploads the client's manifest to a fake PBS and returns what
// was stored, raw and parsed.
func storedManifest(t *testing.T, crypt *CryptConfig) ([]byte, map[string]any) {
	t.Helper()
	srv, captured := newFakePBSServer(t)
	client := newTestClient(srv.URL, crypt)
	client.Manifest.BackupType = "host"
	client.Manifest.BackupID = "writer"
	client.Manifest.BackupTime = 1593179765
	client.Manifest.Files = []File{{CryptMode: CryptModeNone, Filename: "backup.pxar.didx", Size: 1}}
	if err := client.UploadManifest(); err != nil {
		t.Fatalf("UploadManifest: %v", err)
	}
	raw := (*captured)[0].body[DataBlobHeaderSize:]
	var stored map[string]any
	if err := json.Unmarshal(raw, &stored); err != nil {
		t.Fatalf("stored manifest is not valid JSON: %v", err)
	}
	return raw, stored
}

// Every snapshot names the version that wrote it; an encrypted one also the
// format it may use and the first version that reads it. The note is in
// "unprotected", so the signature does not cover it and still verifies.
func TestManifestNamesItsWriter(t *testing.T) {
	SetClientVersion("2.1.0")
	t.Cleanup(func() { SetClientVersion("") })

	_, plain := storedManifest(t, nil)
	w, _ := plain["unprotected"].(map[string]any)["salvanza-client"].(map[string]any)
	if w["version"] != "2.1.0" {
		t.Errorf("unencrypted snapshot: writer %v, want version 2.1.0", w)
	}
	if _, has := w["format"]; has {
		t.Errorf("unencrypted snapshot names a format: %v", w)
	}
	if _, has := w["min-version"]; has {
		t.Errorf("unencrypted snapshot names a minimum version: %v", w)
	}

	crypt := testCryptConfig(t)
	raw, enc := storedManifest(t, crypt)
	w, _ = enc["unprotected"].(map[string]any)["salvanza-client"].(map[string]any)
	if w["version"] != "2.1.0" || w["format"] != float64(SnapshotFormatEncrCompr) || w["min-version"] != "2.1.0" {
		t.Errorf("encrypted snapshot: writer %v, want version 2.1.0, format %d, min-version 2.1.0", w, SnapshotFormatEncrCompr)
	}
	canonical, err := canonicalJSONBytes(raw)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(canonical), "salvanza-client") {
		t.Error("the writer note is inside the signed part of the manifest")
	}
	if sig, _ := enc["signature"].(string); sig != hex.EncodeToString(crypt.ComputeAuthTag(canonical)) {
		t.Error("the signature does not verify with the writer note present")
	}

	// A build with no version says "dev" rather than nothing.
	SetClientVersion("")
	_, dev := storedManifest(t, nil)
	w, _ = dev["unprotected"].(map[string]any)["salvanza-client"].(map[string]any)
	if w["version"] != "dev" {
		t.Errorf("no version set: writer %v, want dev", w)
	}
}

// A manifest with no writer note (version 2.0 and older, the official
// client) and one naming a format this build reads are restored; one naming
// a newer format is refused with the version it needs.
func TestArchiveFormatError(t *testing.T) {
	m := manifestFor("a.pxar.didx", CryptModeEncrypt, nil)
	if err := archiveFormatError(m, "a.pxar.didx"); err != nil {
		t.Errorf("no writer note: %v", err)
	}
	m.Unprotected.Writer = &SnapshotWriter{Version: "2.1.0", Format: ReadableSnapshotFormat, MinVersion: "2.1.0"}
	if err := archiveFormatError(m, "a.pxar.didx"); err != nil {
		t.Errorf("a format this build reads: %v", err)
	}
	m.Unprotected.Writer = &SnapshotWriter{Version: "3.4.0", Format: ReadableSnapshotFormat + 1, MinVersion: "3.0.0"}
	err := archiveFormatError(m, "a.pxar.didx")
	var fe *SnapshotFormatError
	if !errors.As(err, &fe) {
		t.Fatalf("a newer format: got %v, want a *SnapshotFormatError", err)
	}
	for _, want := range []string{"version 3.4.0 of this client", "needs version 3.0.0 or later"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("%q does not contain %q", err.Error(), want)
		}
	}
	if msg := (&SnapshotFormatError{}).Error(); !strings.Contains(msg, "a newer version of this client") || !strings.Contains(msg, "needs a newer version") {
		t.Errorf("nothing named: %q", msg)
	}
}

// The reader refuses a snapshot in a newer format before it fetches a chunk,
// and before it says anything about the key.
func TestDIDXReaderAtNewerFormatRefusedFirst(t *testing.T) {
	key := testCryptConfig(t)
	a := makeDIDX(t, key, payloadsOf(3, 64)...)
	f := newFakePBSWithDIDX(t, "enc.didx", a, key)
	fp := key.Fingerprint()
	m := manifestFor("enc.didx", CryptModeEncrypt, &fp)
	m.Unprotected.Writer = &SnapshotWriter{Version: "9.0.0", Format: ReadableSnapshotFormat + 1, MinVersion: "9.0.0"}
	f.setManifest(t, *m)

	f.Crypt = otherCryptConfig(t) // the wrong key as well: the format is said first
	_, _, err := f.NewDIDXReaderAt("enc.didx", 4, nil)
	var fe *SnapshotFormatError
	if !errors.As(err, &fe) || fe.MinVersion != "9.0.0" {
		t.Fatalf("got %v, want a *SnapshotFormatError naming 9.0.0", err)
	}
	if n := f.totalFetches(); n != 0 {
		t.Fatalf("%d chunks fetched before the format was refused", n)
	}
}

// A chunk whose zstd frame claims to hold more than a blob may is refused
// before memory is found for it.
func TestZstdDecodeAllRefusesOversize(t *testing.T) {
	// A frame header declaring a 1 GiB content size and nothing else: magic,
	// descriptor 0xE0 (single segment, 8 byte content size), the size. The
	// error is the size limit's, not "unexpected end of data".
	frame := []byte{0x28, 0xB5, 0x2F, 0xFD, 0xE0, 0, 0, 0, 0x40, 0, 0, 0, 0}
	if _, err := zstdDecodeAll(frame); !errors.Is(err, zstd.ErrDecoderSizeExceeded) {
		t.Fatalf("a frame declaring 1 GiB: got %v, want the size limit", err)
	}
	// What a client writes still reads.
	crypt := testCryptConfig(t)
	data := []byte(strings.Repeat("ordinary chunk data ", 5000))
	blob, err := crypt.EncodeEncryptedCompressed(data, zstd.SpeedDefault)
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeEncryptedBlob(blob, crypt, nil)
	if err != nil || string(got) != string(data) {
		t.Fatalf("an ordinary compressed chunk no longer reads: %v", err)
	}
}
