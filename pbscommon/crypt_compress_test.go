package pbscommon

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"
)

// vecEncrComprBlob is a compressed + encrypted DataBlob built the way
// proxmox-backup's DataBlob::encode(compress = true) does, by
// testdata/gen_encr_compr_vector.py: libzstd level 1 (the library the Rust
// zstd crate binds) and OpenSSL AES-256-GCM, key 00 01 .. 1f, IV 64 .. 73.
const vecEncrComprBlob = "e6591bbf0bbfd80b533581f56465666768696a6b6c6d6e6f707172733a618e343ee7017d8cb5b485b1ad5327f0066cac84a42725b3f9c98a17a5831d371e20e0de8e8360b0f0340df69a77ba1eb2fda904eb86af627d36756314a812bb312b95819521905434a87ad33bec5e5b39a68e0f73894789"

var vecEncrComprPlain = strings.Repeat("Proxmox Backup compressed encrypted blob test vector. ", 8)

// What the official client writes by default must restore.
func TestDecodeEncrComprGoldenVector(t *testing.T) {
	crypt := testCryptConfig(t)
	raw, err := hex.DecodeString(vecEncrComprBlob)
	if err != nil {
		t.Fatal(err)
	}
	digest := crypt.ComputeDigest([]byte(vecEncrComprPlain))
	got, err := DecodeEncryptedBlob(raw, crypt, digest[:])
	if err != nil {
		t.Fatalf("DecodeEncryptedBlob: %v", err)
	}
	if string(got) != vecEncrComprPlain {
		t.Errorf("plaintext = %q", got)
	}
	if _, err := DecodeEncryptedBlob(raw, nil, nil); err == nil {
		t.Error("decoded without a key")
	}
	raw[len(raw)-1] ^= 1
	if _, err := DecodeEncryptedBlob(raw, crypt, nil); err == nil {
		t.Error("tampered blob accepted")
	}
}

func TestEncodeEncryptedCompressedRoundTrip(t *testing.T) {
	crypt := testCryptConfig(t)
	plain := bytes.Repeat([]byte("compressible chunk data "), 4096)
	blob, err := crypt.EncodeEncryptedCompressed(plain, zstd.SpeedFastest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(blob[:8], blobEncryptedComprMagic) {
		t.Fatalf("magic = % x, want ENCR_COMPR", blob[:8])
	}
	if len(blob) >= len(plain) {
		t.Errorf("not compressed: %d bytes for %d", len(blob), len(plain))
	}
	digest := crypt.ComputeDigest(plain)
	got, err := DecodeEncryptedBlob(blob, crypt, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, plain) {
		t.Error("round trip mismatch")
	}
}

// Incompressible data stays in the plain encrypted form, as in PBS.
func TestEncodeEncryptedCompressedFallsBack(t *testing.T) {
	crypt := testCryptConfig(t)
	plain := make([]byte, 64<<10)
	if _, err := rand.Read(plain); err != nil {
		t.Fatal(err)
	}
	blob, err := crypt.EncodeEncryptedCompressed(plain, zstd.SpeedDefault)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(blob[:8], blobEncryptedMagic) {
		t.Fatalf("magic = % x, want ENCR (uncompressed)", blob[:8])
	}
	got, err := DecodeEncryptedBlob(blob, crypt, nil)
	if err != nil || !bytes.Equal(got, plain) {
		t.Fatalf("round trip: %v", err)
	}
}
