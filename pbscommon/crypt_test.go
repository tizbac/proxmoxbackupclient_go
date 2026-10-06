package pbscommon

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"hash/crc32"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"

	"golang.org/x/crypto/scrypt"
)

// TestFingerprintGoldenVector verifies our key derivation against the vector
// hard coded in proxmox-backup `src/backup/crypt_config.rs`:
//
//	key data = (0u8..32u8).collect()
//	=> fingerprint = [14,171,212,70, ...]
func TestFingerprintGoldenVector(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	crypt, err := NewCryptConfig(key)
	if err != nil {
		t.Fatalf("NewCryptConfig: %v", err)
	}

	expected := []byte{
		14, 171, 212, 70, 11, 110, 185, 202,
		52, 80, 35, 222, 226, 183, 120, 199,
		144, 229, 74, 22, 131, 185, 101, 156,
		10, 87, 174, 25, 144, 144, 21, 155,
	}

	got := crypt.FingerprintBytes()
	if !bytes.Equal(got[:], expected) {
		t.Fatalf("fingerprint bytes mismatch:\n got %v\nwant %v", got, expected)
	}

	want := "0e:ab:d4:46:0b:6e:b9:ca:34:50:23:de:e2:b7:78:c7:90:e5:4a:16:83:b9:65:9c:0a:57:ae:19:90:90:15:9b"
	if got := crypt.Fingerprint(); got != want {
		t.Fatalf("fingerprint string mismatch: got %q want %q", got, want)
	}
}

// TestFingerprintInputMatchesConstant guards the fingerprint constant, which is
// sha256(b"Proxmox Backup Encryption Key Fingerprint").
func TestFingerprintInputMatchesConstant(t *testing.T) {
	want, err := hex.DecodeString("6ed0ef77471fff4d55c7a8fe4a9db62161407f134c725ddf30992d25ec45ed26")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(fingerprintInput, want) {
		t.Fatalf("fingerprint input mismatch:\n got %v\nwant %v", fingerprintInput, want)
	}
}

func TestParseFingerprint(t *testing.T) {
	crypt, err := NewCryptConfig(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := ParseFingerprint(crypt.Fingerprint())
	if err != nil {
		t.Fatalf("ParseFingerprint: %v", err)
	}
	want := crypt.FingerprintBytes()
	if !bytes.Equal(raw, want[:]) {
		t.Fatalf("round trip mismatch: %v", raw)
	}

	for _, bad := range []string{"", "aa:bb", "zz:" + crypt.Fingerprint()[3:]} {
		if _, err := ParseFingerprint(bad); err == nil {
			t.Fatalf("expected error for %q", bad)
		}
	}
}

func TestNewCryptConfigRejectsBadKeySize(t *testing.T) {
	if _, err := NewCryptConfig(make([]byte, 16)); err == nil {
		t.Fatal("expected an error for a 16 byte key")
	}
}

func sha256Sum(data []byte) []byte {
	sum := sha256.Sum256(data)
	return sum[:]
}

func itoa(v uint64) string { return strconv.FormatUint(v, 10) }

func TestComputeDigestAppendsIDKeyLast(t *testing.T) {
	crypt, err := NewCryptConfig(bytes.Repeat([]byte{1}, 32))
	if err != nil {
		t.Fatal(err)
	}
	data := []byte("hello world")
	digest := crypt.ComputeDigest(data)
	if hex.EncodeToString(digest[:]) != crypt.ComputeDigestHex(data) {
		t.Fatal("ComputeDigestHex disagrees with ComputeDigest")
	}
	if bytes.Equal(digest[:], sha256Sum(data)) {
		t.Fatal("encrypted digest must not be a plain sha256")
	}
}

// TestEncodeEncryptedBlobRoundTrip checks the framing and decryption path.
func TestEncodeEncryptedBlobRoundTrip(t *testing.T) {
	crypt, err := NewCryptConfig(bytes.Repeat([]byte{0x42}, 32))
	if err != nil {
		t.Fatal(err)
	}

	for _, size := range []int{0, 1, 15, 16, 17, 4096, 1 << 20} {
		plaintext := make([]byte, size)
		for i := range plaintext {
			plaintext[i] = byte(i * 7 % 251)
		}

		blob, err := crypt.EncodeEncrypted(plaintext)
		if err != nil {
			t.Fatalf("size %d: EncodeEncrypted: %v", size, err)
		}

		if !bytes.Equal(blob[:8], blobEncryptedMagic) {
			t.Fatalf("size %d: wrong magic %v", size, blob[:8])
		}
		if want := EncryptedDataBlobHeaderSize + size; len(blob) != want {
			t.Fatalf("size %d: encoded length %d, want %d", size, len(blob), want)
		}

		stored := uint32(blob[8]) | uint32(blob[9])<<8 | uint32(blob[10])<<16 | uint32(blob[11])<<24
		if got := crc32.ChecksumIEEE(blob[EncryptedDataBlobHeaderSize:]); got != stored {
			t.Fatalf("size %d: CRC32 %d, stored %d", size, got, stored)
		}

		digest := crypt.ComputeDigest(plaintext)
		got, err := DecodeEncryptedBlob(blob, crypt, digest[:])
		if err != nil {
			t.Fatalf("size %d: DecodeEncryptedBlob: %v", size, err)
		}
		if !bytes.Equal(got, plaintext) {
			t.Fatalf("size %d: plaintext mismatch", size)
		}

		if !IsEncryptedBlob(blob) {
			t.Fatalf("size %d: IsEncryptedBlob = false", size)
		}

		// No key must fail cleanly instead of returning garbage.
		if _, err := DecodeEncryptedBlob(blob, nil, nil); err == nil {
			t.Fatalf("size %d: expected error without a key", size)
		}
	}
}

func TestDecodeEncryptedBlobDetectsTampering(t *testing.T) {
	crypt, err := NewCryptConfig(bytes.Repeat([]byte{9}, 32))
	if err != nil {
		t.Fatal(err)
	}
	blob, err := crypt.EncodeEncrypted([]byte("some highly compressible payload"))
	if err != nil {
		t.Fatal(err)
	}

	t.Run("flipped ciphertext bit", func(t *testing.T) {
		tampered := bytes.Clone(blob)
		tampered[len(tampered)-1] ^= 0x01
		if _, err := DecodeEncryptedBlob(tampered, crypt, nil); err == nil {
			t.Fatal("expected an auth failure")
		}
	})

	t.Run("flipped tag bit", func(t *testing.T) {
		tampered := bytes.Clone(blob)
		tampered[30] ^= 0x80
		if _, err := DecodeEncryptedBlob(tampered, crypt, nil); err == nil {
			t.Fatal("expected an auth failure")
		}
	})

	t.Run("wrong crc", func(t *testing.T) {
		tampered := bytes.Clone(blob)
		tampered[8] ^= 0xff
		if _, err := DecodeEncryptedBlob(tampered, crypt, nil); err == nil {
			t.Fatal("expected a CRC failure")
		}
	})

	t.Run("wrong digest", func(t *testing.T) {
		other, err := NewCryptConfig(bytes.Repeat([]byte{1}, 32))
		if err != nil {
			t.Fatal(err)
		}
		digest := other.ComputeDigest([]byte("some highly compressible payload"))
		if _, err := DecodeEncryptedBlob(blob, crypt, digest[:]); err == nil {
			t.Fatal("expected a digest mismatch")
		}
	})
}

func TestDecodeEncryptedBlobRejectsUnknownMagic(t *testing.T) {
	blob := make([]byte, 32)
	blob[0] = 0xde
	if _, err := DecodeEncryptedBlob(blob, nil, nil); err == nil {
		t.Fatal("expected an error for an unknown magic")
	}
	if _, err := DecodeEncryptedBlob([]byte{1, 2, 3}, nil, nil); err == nil {
		t.Fatal("expected an error for a truncated blob")
	}
}

// TestDecodeEncryptedBlobUnencryptedVariants makes sure the pre-existing
// unencrypted blob formats still decode, both with and without a key.
func TestDecodeEncryptedBlobUnencryptedVariants(t *testing.T) {
	payload := bytes.Repeat([]byte("compress me "), 64)
	csum := sha256Sum(payload)

	raw := make([]byte, 0, len(payload)+DataBlobHeaderSize)
	raw = append(raw, blobUncompressedMagic...)
	raw = append(raw, 0, 0, 0, 0)
	raw = append(raw, payload...)
	crc := crc32.ChecksumIEEE(raw[DataBlobHeaderSize:])
	raw[8], raw[9], raw[10], raw[11] = byte(crc), byte(crc>>8), byte(crc>>16), byte(crc>>24)

	if IsEncryptedBlob(raw) {
		t.Fatal("uncompressed blob must not be reported as encrypted")
	}
	got, err := DecodeEncryptedBlob(raw, nil, csum)
	if err != nil {
		t.Fatalf("decode uncompressed: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatal("uncompressed payload mismatch")
	}
	if _, err := DecodeEncryptedBlob(raw, nil, sha256Sum([]byte("other"))); err == nil {
		t.Fatal("expected a digest mismatch")
	}
}

// TestManifestSignatureGoldenVector reproduces
// proxmox-backup `src/backup/manifest.rs::test_manifest_signature`.
func TestManifestSignatureGoldenVector(t *testing.T) {
	testkey, err := scrypt.Key([]byte("test"), nil, 1<<16, 8, 1, 32)
	if err != nil {
		t.Fatal(err)
	}
	crypt, err := NewCryptConfig(testkey)
	if err != nil {
		t.Fatal(err)
	}

	manifest := map[string]any{
		"backup-type": "host",
		"backup-id":   "elsa",
		"backup-time": json.Number("1593179765"),
		"files": []any{
			fileInfoJSON("test1.img.fidx", 200, bytes.Repeat([]byte{1}, 32), CryptModeEncrypt),
			fileInfoJSON("abc.blob", 200, bytes.Repeat([]byte{2}, 32), CryptModeNone),
		},
		"unprotected": map[string]any{
			"note": "This is not protected by the signature.",
		},
	}
	delete(manifest, "signature")
	delete(manifest, "unprotected")

	canonical, err := canonicalJSON(manifest)
	if err != nil {
		t.Fatal(err)
	}
	// serde_json sorts object keys byte-wise, so "crypt-mode" < "csum".
	expectedCanonical := `{"backup-id":"elsa","backup-time":1593179765,"backup-type":"host",` +
		`"files":[{"crypt-mode":"encrypt",` +
		`"csum":"0101010101010101010101010101010101010101010101010101010101010101",` +
		`"filename":"test1.img.fidx","size":200},` +
		`{"crypt-mode":"none",` +
		`"csum":"0202020202020202020202020202020202020202020202020202020202020202",` +
		`"filename":"abc.blob","size":200}]}`
	if string(canonical) != expectedCanonical {
		t.Fatalf("canonical json mismatch:\n got %s\nwant %s", canonical, expectedCanonical)
	}

	signature := hex.EncodeToString(crypt.ComputeAuthTag(canonical))
	const want = "d7b446fb7db081662081d4b40fedd858a1d6307a5aff4ecff7d5bf4fd35679e9"
	if signature != want {
		t.Fatalf("signature mismatch: got %s want %s", signature, want)
	}
}

func fileInfoJSON(name string, size uint64, csum []byte, mode string) map[string]any {
	return map[string]any{
		"filename":   name,
		"crypt-mode": mode,
		"size":       json.Number(itoa(size)),
		"csum":       hex.EncodeToString(csum),
	}
}

func TestCanonicalJSONEscaping(t *testing.T) {
	value := map[string]any{
		"z":        json.Number("1"),
		"a":        "quote:\" backslash:\\ slash:/ lt:< amp:& del:\x7f",
		"controls": "\x00\x08\x09\x0a\x0c\x0d\x1f",
		"utf8":     "é 中",
		"nested":   map[string]any{"b": true},
	}

	got, err := canonicalJSON(value)
	if err != nil {
		t.Fatal(err)
	}
	// null is rejected by proxmox, everything else must be serde_json compatible.
	want := `{"a":"quote:\" backslash:\\ slash:/ lt:< amp:& del:` + "\x7f" + `",` +
		`"controls":"\u0000\b\t\n\f\r\u001f",` +
		`"nested":{"b":true},` +
		`"utf8":"é 中","z":1}`
	if string(got) != want {
		t.Fatalf("escaping mismatch:\n got %q\nwant %q", got, want)
	}

	if _, err := canonicalJSONBytes([]byte(`[1,2]`)); err == nil {
		t.Fatal("expected an error for a non object document")
	}
}

// TestCanonicalJSONBytesDropsSignedFields mirrors what proxmox-backup removes
// before signing.
func TestCanonicalJSONBytesDropsSignedFields(t *testing.T) {
	raw := []byte(`{"signature":"dead","unprotected":{"a":1},"files":[],"backup-id":"x"}`)
	got, err := canonicalJSONBytes(raw)
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"backup-id":"x","files":[]}`; string(got) != want {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestKeyFileRoundTrip(t *testing.T) {
	crypt, err := NewCryptConfig(bytes.Repeat([]byte{0xab}, 32))
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "enc.key")
	if err := SaveKeyFile(path, crypt); err != nil {
		t.Fatalf("SaveKeyFile: %v", err)
	}

	// Windows has no Unix permission bits: Go reports 0666 whatever mode was asked for.
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if perm := info.Mode().Perm(); perm != 0600 {
			t.Fatalf("key file permissions %o, want 600", perm)
		}
	}

	loaded, err := LoadKeyFile(path, nil)
	if err != nil {
		t.Fatalf("LoadKeyFile: %v", err)
	}
	if loaded.Fingerprint() != crypt.Fingerprint() {
		t.Fatalf("fingerprint %s, want %s", loaded.Fingerprint(), crypt.Fingerprint())
	}
}

func TestLoadKeyFileWithKDF(t *testing.T) {
	rawKey := bytes.Repeat([]byte{0x5c}, 32)
	crypt, err := NewCryptConfig(rawKey)
	if err != nil {
		t.Fatal(err)
	}

	salt := bytes.Repeat([]byte{0x11}, 32)
	passphrase := []byte("correct horse")
	derived, err := scrypt.Key(passphrase, salt, 1<<15, 8, 1, 32)
	if err != nil {
		t.Fatal(err)
	}
	derivedCrypt, err := NewCryptConfig(derived)
	if err != nil {
		t.Fatal(err)
	}
	iv, sealed, tag, err := derivedCrypt.Seal(rawKey)
	if err != nil {
		t.Fatal(err)
	}
	blob := append(append(append([]byte{}, iv...), tag...), sealed...)

	cfg := KeyConfig{
		KDF: &KeyDerivationConfig{
			KDF:  "scrypt",
			N:    1 << 15,
			R:    8,
			P:    1,
			Salt: base64.StdEncoding.EncodeToString(salt),
		},
		Data:        base64.StdEncoding.EncodeToString(blob),
		Fingerprint: crypt.Fingerprint(),
	}
	path := filepath.Join(t.TempDir(), "pw.key")
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}

	got, err := LoadKeyFile(path, passphrase)
	if err != nil {
		t.Fatalf("LoadKeyFile: %v", err)
	}
	if got.Fingerprint() != crypt.Fingerprint() {
		t.Fatal("fingerprint mismatch after decrypting a protected key file")
	}

	if _, err := LoadKeyFile(path, []byte("wrong")); err == nil {
		t.Fatal("expected a decryption failure")
	}
	if _, err := LoadKeyFile(path, []byte("abc")); err == nil {
		t.Fatal("expected a short passphrase error")
	}
}

func TestLoadKeyFileRejectsBadInput(t *testing.T) {
	dir := t.TempDir()

	if _, err := LoadKeyFile(filepath.Join(dir, "missing.key"), nil); err == nil {
		t.Fatal("expected an error for a missing file")
	}

	broken := filepath.Join(dir, "broken.key")
	if err := os.WriteFile(broken, []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadKeyFile(broken, nil); err == nil {
		t.Fatal("expected a parse error")
	}

	empty := filepath.Join(dir, "empty.key")
	if err := os.WriteFile(empty, []byte(`{"data":""}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadKeyFile(empty, nil); err == nil {
		t.Fatal("expected an error for an empty key")
	}

	// A fingerprint that does not match the key must be rejected.
	short := filepath.Join(dir, "short.key")
	if err := os.WriteFile(short, []byte(`{"data":"AAAA"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadKeyFile(short, nil); err == nil {
		t.Fatal("expected a key size error")
	}
}

func TestGenerateEncryptionKey(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 8; i++ {
		key, err := GenerateEncryptionKey()
		if err != nil {
			t.Fatal(err)
		}
		if len(key) != BlobEncryptionKeySize {
			t.Fatalf("key length %d", len(key))
		}
		if seen[string(key)] {
			t.Fatal("GenerateEncryptionKey returned a duplicate")
		}
		seen[string(key)] = true
		if _, err := NewCryptConfig(key); err != nil {
			t.Fatal(err)
		}
	}
}
