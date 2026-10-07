package pbscommon

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// splitWriteTree creates a small real tree and returns its files' contents
// keyed by archive path.
func splitWriteTree(t *testing.T) (string, map[string][]byte) {
	t.Helper()
	root := t.TempDir()
	big := make([]byte, 5*1024*1024+123)
	rand.Read(big)
	files := map[string][]byte{
		"hello.txt":            []byte("hello\n"),
		"empty.txt":            nil,
		"sub/nested/deep.txt":  []byte("deep\n"),
		"sub/big.bin":          big,
		"spaces & ünicode.txt": []byte("u\n"),
	}
	for p, data := range files {
		full := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	os.MkdirAll(filepath.Join(root, "empty-dir"), 0755)
	mt := time.Date(2021, 3, 4, 5, 6, 7, 123456700, time.UTC)
	if err := os.Chtimes(filepath.Join(root, "hello.txt"), mt, mt); err != nil {
		t.Fatal(err)
	}
	return root, files
}

func writeArchive(t *testing.T, root string, split bool) (meta, payload []byte, fileStarts int) {
	t.Helper()
	var m, p bytes.Buffer
	a := &PXARArchive{ArchiveName: "test.pxar.didx", Split: split}
	a.WriteCB = func(b []byte) error { m.Write(b); return nil }
	if split {
		a.PayloadWriteCB = func(b []byte) error { p.Write(b); return nil }
		a.OnPayloadFileStart = func() error {
			// Every payload byte before the file must already be out.
			if uint64(p.Len()) != a.PayloadPos() {
				t.Errorf("payload not flushed before file start: %d written, pos %d", p.Len(), a.PayloadPos())
			}
			fileStarts++
			return nil
		}
	}
	if _, err := a.WriteDir(root, "", true); err != nil {
		t.Fatalf("WriteDir: %v", err)
	}
	return m.Bytes(), p.Bytes(), fileStarts
}

func TestSplitWriteRoundTrip(t *testing.T) {
	root, files := splitWriteTree(t)
	meta, payload, starts := writeArchive(t, root, true)

	if v := binary.LittleEndian.Uint64(meta[0:8]); v != PXAR_FORMAT_VERSION {
		t.Fatalf("metadata stream does not start with the format version entry")
	}
	if binary.LittleEndian.Uint64(payload[len(payload)-16:]) != PXAR_PAYLOAD_TAIL_MARKER {
		t.Fatalf("payload stream does not end with the tail marker")
	}
	if starts != len(files) {
		t.Errorf("file-start hook ran %d times, want %d", starts, len(files))
	}
	if bytes.Contains(meta, func() []byte { b := make([]byte, 8); binary.LittleEndian.PutUint64(b, PXAR_PAYLOAD); return b }()) {
		t.Errorf("metadata stream contains an inline PXAR_PAYLOAD record")
	}

	r, err := NewSplitPXARReaderAt(bytes.NewReader(meta), int64(len(meta)), bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		t.Fatal(err)
	}
	es, err := r.ListEntries()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, e := range es {
		seen[e.Path] = true
		if e.Path == "hello.txt" && e.ModNanos != 123456700 {
			t.Errorf("hello.txt nanos = %d, want 123456700 (split archives carry the real sub-second mtime)", e.ModNanos)
		}
	}
	for p := range files {
		if !seen[p] {
			t.Errorf("%s missing from listing", p)
		}
	}
	if !seen["empty-dir"] {
		t.Errorf("empty-dir missing from listing")
	}

	dest := t.TempDir()
	if _, err := r.ExtractAll(dest); err != nil {
		t.Fatal(err)
	}
	for p, want := range files {
		got, err := os.ReadFile(filepath.Join(dest, filepath.FromSlash(p)))
		if err != nil || !bytes.Equal(got, want) {
			t.Errorf("%s: restored %d bytes (err %v), want %d", p, len(got), err, len(want))
		}
	}

	// Single-file selection goes through the GOODBYE lookup.
	dest2 := t.TempDir()
	if _, err := r.ExtractFiltered(dest2, []string{"sub/big.bin"}, true); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(dest2, "sub", "big.bin"))
	if !bytes.Equal(got, files["sub/big.bin"]) {
		t.Errorf("selective restore of sub/big.bin returned %d bytes", len(got))
	}
	if _, err := os.Stat(filepath.Join(dest2, "hello.txt")); !os.IsNotExist(err) {
		t.Errorf("selective restore wrote hello.txt too")
	}
	if s, e, _, err := r.ResolveArchivePathBST("sub/nested/deep.txt"); err != nil || e <= s {
		t.Errorf("GOODBYE lookup in our split archive failed: %v", err)
	}
}

// The classic stream must not change: same content as a split archive's
// files, no version entry, and the classic reader reads it.
func TestClassicWriteUnchangedBySplitSupport(t *testing.T) {
	root, files := splitWriteTree(t)
	meta, payload, _ := writeArchive(t, root, false)
	if len(payload) != 0 {
		t.Fatalf("classic archive wrote %d payload-stream bytes", len(payload))
	}
	if binary.LittleEndian.Uint64(meta[0:8]) == PXAR_FORMAT_VERSION {
		t.Fatalf("classic archive starts with a format version entry")
	}
	dest := t.TempDir()
	if _, err := NewPXARReader(meta).ExtractAll(dest); err != nil {
		t.Fatal(err)
	}
	for p, want := range files {
		got, _ := os.ReadFile(filepath.Join(dest, filepath.FromSlash(p)))
		if !bytes.Equal(got, want) {
			t.Errorf("%s differs after classic round trip", p)
		}
	}
}

func TestChunkerReset(t *testing.T) {
	var c Chunker
	c.New(1024 * 1024 * 4)
	data := make([]byte, 3*1024*1024)
	rand.Read(data)
	c.Scan(data[:1000])
	c.Reset()
	var fresh Chunker
	fresh.New(1024 * 1024 * 4)
	if c.Scan(data) != fresh.Scan(data) {
		t.Fatal("a reset chunker does not behave like a fresh one")
	}
	if c.MinSize() != 1024*1024 || c.MaxSize() != 16*1024*1024 {
		t.Fatalf("bounds %d/%d", c.MinSize(), c.MaxSize())
	}
}
