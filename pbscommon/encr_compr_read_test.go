package pbscommon

import (
	"bytes"
	"crypto/rand"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/klauspost/compress/zstd"
)

// An encrypted archive holds chunks in both stored forms, in any order:
// compressed then encrypted where that made the chunk smaller, encrypted as
// it is where it did not. That is every encrypted snapshot of the official
// client and of this one from version 2.1. The reader that every restore goes
// through (folders, disk images, the file browser) reads such an archive end
// to end, and at offsets inside and across chunks.
func TestDIDXReaderAtMixedEncryptedForms(t *testing.T) {
	key := testCryptConfig(t)
	noise := make([]byte, 64<<10)
	if _, err := rand.Read(noise); err != nil {
		t.Fatal(err)
	}
	payloads := [][]byte{
		[]byte(strings.Repeat("a line of text that compresses well\n", 3000)),
		noise,
		bytes.Repeat([]byte{0}, 100_000),
		noise[:1000],
		[]byte(strings.Repeat("another run of text, after the noise. ", 500)),
	}
	want := bytes.Join(payloads, nil)

	a := makeDIDX(t, key, payloads...)
	f := newFakePBSWithDIDX(t, "mixed.didx", a, key)
	var mu sync.Mutex
	forms := map[string]int{}
	f.encode = func(p []byte) []byte {
		blob, err := key.EncodeEncryptedCompressed(p, zstd.SpeedDefault)
		if err != nil {
			t.Errorf("encode: %v", err)
			return nil
		}
		mu.Lock()
		forms[string(blob[:8])]++
		mu.Unlock()
		return blob
	}

	r, total, err := f.NewDIDXReaderAt("mixed.didx", 2, nil)
	if err != nil {
		t.Fatalf("NewDIDXReaderAt: %v", err)
	}
	if total != int64(len(want)) {
		t.Fatalf("total = %d, want %d", total, len(want))
	}
	got, err := io.ReadAll(io.NewSectionReader(r, 0, total))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("the archive read back differs from what was stored")
	}
	// Across the join of a compressed chunk and an uncompressed one, both ways.
	for _, off := range []int{len(payloads[0]) - 10, len(payloads[0]) + len(noise) - 10} {
		buf := make([]byte, 20)
		if _, err := r.ReadAt(buf, int64(off)); err != nil && !errors.Is(err, io.EOF) {
			t.Fatalf("ReadAt(%d): %v", off, err)
		}
		if !bytes.Equal(buf, want[off:off+20]) {
			t.Errorf("ReadAt(%d) differs", off)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if forms[string(blobEncryptedComprMagic)] < 3 || forms[string(blobEncryptedMagic)] < 2 {
		t.Errorf("chunks served: %d compressed and encrypted, %d encrypted only; the test needs both forms",
			forms[string(blobEncryptedComprMagic)], forms[string(blobEncryptedMagic)])
	}
}

// What a compressed encrypted chunk is refused for, each with an error and
// none with a panic or with wrong data handed back.
func TestDecodeEncrComprRefusals(t *testing.T) {
	key := testCryptConfig(t)
	plain := []byte(strings.Repeat("chunk data that compresses. ", 2000))
	blob, err := key.EncodeEncryptedCompressed(plain, zstd.SpeedDefault)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(blob[:8], blobEncryptedComprMagic) {
		t.Fatalf("magic % x: the test needs a compressed chunk", blob[:8])
	}
	digest := key.ComputeDigest(plain)

	// The right key and digest: it reads.
	if got, err := DecodeEncryptedBlob(blob, key, digest[:]); err != nil || !bytes.Equal(got, plain) {
		t.Fatalf("the chunk does not read: %v", err)
	}
	// Another key.
	if _, err := DecodeEncryptedBlob(blob, otherCryptConfig(t), nil); err == nil {
		t.Error("read with another key")
	}
	// The digest of other data: the chunk decrypts and decompresses, and is
	// then refused for what it holds.
	other := key.ComputeDigest([]byte("other data"))
	if _, err := DecodeEncryptedBlob(blob, key, other[:]); err == nil || !strings.Contains(err.Error(), "digest mismatch") {
		t.Errorf("the digest of other data: got %v, want a digest mismatch", err)
	}
	// The magic is not under the authentication tag: a compressed chunk
	// relabelled as an uncompressed one still decrypts. The digest of the
	// chunk, which every index gives, is what refuses the compressed bytes.
	relabelled := append([]byte{}, blob...)
	copy(relabelled, blobEncryptedMagic)
	if _, err := DecodeEncryptedBlob(relabelled, key, digest[:]); err == nil {
		t.Error("a compressed chunk relabelled as uncompressed was accepted")
	}
	// And the other way: an uncompressed chunk relabelled as compressed is
	// not a zstd frame.
	enc, err := key.EncodeEncrypted(plain)
	if err != nil {
		t.Fatal(err)
	}
	copy(enc, blobEncryptedComprMagic)
	if _, err := DecodeEncryptedBlob(enc, key, digest[:]); err == nil || !strings.Contains(err.Error(), "unable to decompress") {
		t.Errorf("an uncompressed chunk relabelled as compressed: got %v, want a decompression error", err)
	}
	// A frame cut short inside good encryption.
	frame, err := zstdEncodeForTest(plain)
	if err != nil {
		t.Fatal(err)
	}
	short, err := key.encodeEncrypted(frame[:len(frame)/2], blobEncryptedComprMagic)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeEncryptedBlob(short, key, nil); err == nil || !strings.Contains(err.Error(), "unable to decompress") {
		t.Errorf("a zstd frame cut short: got %v, want a decompression error", err)
	}
	// The stored bytes cut short: the checksum says so before the key is used.
	if _, err := DecodeEncryptedBlob(blob[:len(blob)-5], key, nil); err == nil {
		t.Error("a blob cut short was accepted")
	}
}

func zstdEncodeForTest(data []byte) ([]byte, error) {
	enc, err := zstd.NewWriter(nil, zstd.WithEncoderConcurrency(1))
	if err != nil {
		return nil, err
	}
	defer enc.Close()
	return enc.EncodeAll(data, nil), nil
}

// Data that compression cannot shrink, nothing included, is stored encrypted
// as it is and reads back, at every level.
func TestEncodeEncryptedCompressedEdges(t *testing.T) {
	key := testCryptConfig(t)
	noise := make([]byte, 4096)
	if _, err := rand.Read(noise); err != nil {
		t.Fatal(err)
	}
	for _, level := range []zstd.EncoderLevel{zstd.SpeedFastest, zstd.SpeedDefault, zstd.SpeedBetterCompression, zstd.SpeedBestCompression} {
		for name, data := range map[string][]byte{"empty": {}, "one byte": {42}, "noise": noise} {
			blob, err := key.EncodeEncryptedCompressed(data, level)
			if err != nil {
				t.Fatalf("%s, level %v: %v", name, level, err)
			}
			if !bytes.Equal(blob[:8], blobEncryptedMagic) {
				t.Errorf("%s, level %v: magic % x, want the uncompressed encrypted form", name, level, blob[:8])
			}
			digest := key.ComputeDigest(data)
			got, err := DecodeEncryptedBlob(blob, key, digest[:])
			if err != nil || !bytes.Equal(got, data) {
				t.Errorf("%s, level %v: does not read back: %v", name, level, err)
			}
		}
		// And data that does shrink reads back at this level.
		text := []byte(strings.Repeat("text ", 10000))
		blob, err := key.EncodeEncryptedCompressed(text, level)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(blob[:8], blobEncryptedComprMagic) || len(blob) >= len(text) {
			t.Errorf("level %v: text was not stored compressed (%d bytes for %d)", level, len(blob), len(text))
		}
		if got, err := DecodeEncryptedBlob(blob, key, nil); err != nil || !bytes.Equal(got, text) {
			t.Errorf("level %v: text does not read back: %v", level, err)
		}
	}
}
