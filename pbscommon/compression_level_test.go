package pbscommon

import (
	"bytes"
	"strings"
	"testing"
)

// A setting or option may hold the five levels (any case) or nothing; the
// spellings ParseCompressionLevel also takes ("fast", "max", "off", "0") and
// anything else are refused, so a typo is not run as fastest.
func TestValidCompressionLevel(t *testing.T) {
	for _, ok := range []string{"", "none", "fastest", "default", "better", "best", "Best", "DEFAULT", "None"} {
		if !ValidCompressionLevel(ok) {
			t.Errorf("%q refused", ok)
		}
	}
	for _, bad := range []string{"fast", "max", "off", "0", "3", "zstd", "fastest "} {
		if ValidCompressionLevel(bad) {
			t.Errorf("%q accepted", bad)
		}
	}
	if ParseCompressionLevel("") != CompressionDefault {
		t.Error("empty is not default")
	}
	for _, none := range []string{"none", "None", "off", "0"} {
		if ParseCompressionLevel(none) != CompressionNone {
			t.Errorf("%q is not none", none)
		}
	}
}

// Compression "none" sends every chunk and blob as it is: plain ones without
// the zstd frame, encrypted ones in the form version 2.0 of this client also
// reads (ENCR, not ENCR_COMPR), although every backup path asks for a
// compressed chunk. Any other level compresses both.
func TestCompressionNoneSendsUncompressed(t *testing.T) {
	data := []byte(strings.Repeat("compressible data, the same again. ", 4000))
	for _, c := range []struct {
		name      string
		level     CompressionLevel
		encrypted bool
		chunk     []byte // magic the chunk must carry
		blob      []byte // magic the blob must carry
	}{
		{"none, plain", CompressionNone, false, blobUncompressedMagic, blobUncompressedMagic},
		{"none, encrypted", CompressionNone, true, blobEncryptedMagic, blobEncryptedMagic},
		{"default, plain", CompressionDefault, false, blobCompressedMagic, blobUncompressedMagic},
		{"default, encrypted", CompressionDefault, true, blobEncryptedComprMagic, blobEncryptedComprMagic},
	} {
		t.Run(c.name, func(t *testing.T) {
			srv, captured := newFakePBSServer(t)
			var crypt *CryptConfig
			if c.encrypted {
				crypt = testCryptConfig(t)
			}
			client := newTestClient(srv.URL, crypt)
			client.CompressionLevel = c.level

			if err := client.UploadDynamicCompressedChunk(1, client.ChunkDigestHex(data), data); err != nil {
				t.Fatalf("chunk: %v", err)
			}
			if err := client.UploadBlob("test.blob", data); err != nil {
				t.Fatalf("blob: %v", err)
			}
			if len(*captured) != 2 {
				t.Fatalf("%d uploads captured, want 2", len(*captured))
			}
			chunk, blob := (*captured)[0].body, (*captured)[1].body
			if !bytes.Equal(chunk[:8], c.chunk) {
				t.Errorf("chunk magic % x, want % x", chunk[:8], c.chunk)
			}
			if !bytes.Equal(blob[:8], c.blob) {
				t.Errorf("blob magic % x, want % x", blob[:8], c.blob)
			}
			// Whatever the form, both read back as what was sent.
			for what, raw := range map[string][]byte{"chunk": chunk, "blob": blob} {
				got, err := DecodeEncryptedBlob(raw, crypt, nil)
				if err != nil || !bytes.Equal(got, data) {
					t.Errorf("%s does not read back: %v", what, err)
				}
			}
			if c.level == CompressionNone && len(chunk) < len(data) {
				t.Errorf("chunk is %d bytes for %d of data: it was compressed", len(chunk), len(data))
			}
		})
	}
}
