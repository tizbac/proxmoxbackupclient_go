package pbscommon

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/klauspost/compress/zstd"
)

// Fuzz tests for the parsers that read what a PBS server, or a file a user
// picks, hands the client: the catalog, the dynamic and fixed indexes, pxar
// archives and data blobs. Whatever the bytes, none may panic, and an
// extraction may write only inside its destination. As plain tests they run
// their seeds; go test -fuzz=FuzzName explores further.

func FuzzParseCatalog(f *testing.F) {
	root := uint64(len(catalog_magic))
	var td []byte
	td = append_u64_7bit(td, 2)
	td = append(td, 'f')
	td = append_u64_7bit(td, 5)
	td = append(td, "a.txt"...)
	td = append_u64_7bit(td, 3)
	td = append_u64_7bit(td, 100)
	td = append(td, 'l')
	td = append_u64_7bit(td, 4)
	td = append(td, "link"...)
	buf := append([]byte{}, catalog_magic...)
	buf = append_u64_7bit(buf, uint64(len(td)))
	buf = append(buf, td...)
	trailer := uint64(len(buf))
	buf = append(buf, buildCatalogTable([]dirEnc{{"backup.pxar", root}}, nil, trailer)...)
	ptr := make([]byte, 8)
	binary.LittleEndian.PutUint64(ptr, trailer)
	f.Add(append(buf, ptr...))
	f.Add(append(append([]byte{}, catalog_magic...), make([]byte, 16)...))
	f.Fuzz(func(t *testing.T, data []byte) {
		entries, err := ParseCatalog(data)
		if err == nil && len(entries) > catalogMaxEntries {
			t.Fatalf("%d entries, over the %d limit", len(entries), catalogMaxEntries)
		}
	})
}

func FuzzParseIndexes(f *testing.F) {
	didx := append(append([]byte{}, didxMagic...), make([]byte, didxHeaderSize-len(didxMagic))...)
	entry := make([]byte, didxEntrySize)
	binary.LittleEndian.PutUint64(entry, 4096)
	f.Add(append(append([]byte{}, didx...), entry...))
	f.Add(makeFIDX(8192, 4096, [][32]byte{{1}, {2}}))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		_ = ParsePreviousDIDXChunkDigests(data)
		if offsets, digests, err := parseDIDX(data); err == nil && len(offsets) != len(digests) {
			t.Fatalf("%d offsets, %d digests", len(offsets), len(digests))
		}
		if idx, err := parseFIDXIndex("fuzz.fidx", data); err == nil && len(idx.offsets) != len(idx.digests) {
			t.Fatalf("%d offsets, %d digests", len(idx.offsets), len(idx.digests))
		}
	})
}

func FuzzPXARListAndExtract(f *testing.F) {
	f.Add(standardPXAR(f))
	f.Add(makePXAR(f, pxarFile("../escape.txt", []byte("x")), pxarDir("d"), pxarFile("d/ok.txt", []byte("ok"))))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		// Lists and extracts must not panic; an extraction must stay inside dest.
		_, _ = NewPXARReader(data).ListEntries()
		parent := t.TempDir()
		dest := filepath.Join(parent, "dest")
		if err := os.Mkdir(dest, 0o755); err != nil {
			t.Fatal(err)
		}
		_, _ = NewPXARReader(data).ExtractAll(dest)
		items, err := os.ReadDir(parent)
		if err != nil {
			t.Fatal(err)
		}
		for _, it := range items {
			if it.Name() != "dest" {
				t.Fatalf("extraction wrote %q outside its destination", it.Name())
			}
		}
	})
}

func FuzzDecodeBlob(f *testing.F) {
	f.Add(append(append([]byte{}, blobUncompressedMagic...), []byte("1234plain data")...))
	f.Add(append(append([]byte{}, blobCompressedMagic...), make([]byte, 12)...))
	f.Add([]byte{})
	crypt, err := NewCryptConfig(bytes.Repeat([]byte{7}, 32))
	if err != nil {
		f.Fatal(err)
	}
	// Both encrypted forms, made with the key the fuzz target holds, so that
	// mutations start from blobs that decrypt: the compressed one leads into
	// the zstd decoder behind the encryption.
	text := bytes.Repeat([]byte("fuzz seed text "), 64)
	if enc, err := crypt.EncodeEncrypted(text); err == nil {
		f.Add(enc)
	}
	if encCompr, err := crypt.EncodeEncryptedCompressed(text, zstd.SpeedDefault); err == nil {
		f.Add(encCompr)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = DecodeEncryptedBlob(data, nil, nil)
		_, _ = DecodeEncryptedBlob(data, crypt, nil)
	})
}
