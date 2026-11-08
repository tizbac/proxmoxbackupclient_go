package pbscommon

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

// Upstream's GUI and service directory backup encrypted chunks but indexed
// them under the bare SHA-256 of the plaintext (upstream 1d1d9e8). Such
// snapshots must stay restorable with the key: ChunkMatchesDigest accepts a
// bare SHA-256 whenever a key is loaded, after AES-GCM has already
// authenticated the chunk.
func TestDIDXReaderAtLegacyEncryptedArchive(t *testing.T) {
	crypt := testCryptConfig(t)
	payloads := payloadsOf(3, 128)
	a := makeDIDX(t, nil, payloads...) // bare SHA-256 digests...
	want := bytes.Join(payloads, nil)

	f := newFakePBSWithDIDX(t, "legacy.didx", a, crypt) // ...over encrypted blobs
	r, _, err := f.NewDIDXReaderAt("legacy.didx", 8, nil)
	if err != nil {
		t.Fatalf("NewDIDXReaderAt: %v", err)
	}
	got := make([]byte, len(want))
	if _, err := r.ReadAt(got, 0); err != nil && !errors.Is(err, io.EOF) {
		t.Fatalf("ReadAt: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Error("decrypted stream differs from the original chunks")
	}
}

// Accepting the bare digest must not turn the integrity check off.
func TestDIDXReaderAtLegacyEncryptedArchiveRejectsMismatch(t *testing.T) {
	crypt := testCryptConfig(t)
	payloads := payloadsOf(2, 64)
	a := makeDIDX(t, nil, payloads...)
	f := newFakePBSWithDIDX(t, "legacy.didx", a, crypt)
	f.corrupt[a.order[0]] = bytes.Repeat([]byte{0xAB}, len(payloads[0]))

	r, _, err := f.NewDIDXReaderAt("legacy.didx", 4, nil)
	if err != nil {
		t.Fatalf("NewDIDXReaderAt: %v", err)
	}
	_, err = r.ReadAt(make([]byte, 8), 0)
	if err == nil || !strings.Contains(err.Error(), "content hash mismatch") {
		t.Errorf("error = %v, want a content hash mismatch", err)
	}
}
