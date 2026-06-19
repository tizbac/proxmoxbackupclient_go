package pbscommon

import (
	"encoding/binary"
	"errors"
	"testing"
)

// dirEnc / fileEnc describe a directory table to be serialized exactly the way
// the PXAR writer (pxar.go) encodes the pcat1 catalog.
type dirEnc struct {
	name       string
	childStart uint64 // catalog offset of the child directory's table
}

type fileEnc struct {
	name  string
	size  uint64
	mtime uint64
}

// buildCatalogTable serializes one length-prefixed directory table at offset
// tableStart, mirroring the writer: a varint length, then varint entry count,
// then 'd' entries (backward offset to the child table) followed by 'f' entries
// (size, mtime).
func buildCatalogTable(dirs []dirEnc, files []fileEnc, tableStart uint64) []byte {
	var td []byte
	td = append_u64_7bit(td, uint64(len(dirs)+len(files)))
	for _, d := range dirs {
		td = append(td, 'd')
		td = append_u64_7bit(td, uint64(len(d.name)))
		td = append(td, d.name...)
		td = append_u64_7bit(td, tableStart-d.childStart)
	}
	for _, f := range files {
		td = append(td, 'f')
		td = append_u64_7bit(td, uint64(len(f.name)))
		td = append(td, f.name...)
		td = append_u64_7bit(td, f.size)
		td = append_u64_7bit(td, f.mtime)
	}
	var out []byte
	out = append_u64_7bit(out, uint64(len(td)))
	return append(out, td...)
}

// buildCatalog assembles a complete pcat1 stream for the fixed tree:
//
//	root/ (A.txt, empty/, S/ -> B.txt)
//
// written leaves-first like the real writer, terminated by the trailer table
// and the little-endian root pointer.
func buildCatalog(t *testing.T) []byte {
	t.Helper()
	buf := append([]byte{}, catalog_magic...) // magic occupies [0,8)

	// Leaf directories first so their tables precede references to them.
	sStart := uint64(len(buf))
	buf = append(buf, buildCatalogTable(nil, []fileEnc{{"B.txt", 20, 222}}, sStart)...)

	emptyStart := uint64(len(buf))
	buf = append(buf, buildCatalogTable(nil, nil, emptyStart)...)

	rootStart := uint64(len(buf))
	buf = append(buf, buildCatalogTable(
		[]dirEnc{{"S", sStart}, {"empty", emptyStart}},
		[]fileEnc{{"A.txt", 10, 111}},
		rootStart,
	)...)

	trailerStart := uint64(len(buf))
	buf = append(buf, buildCatalogTable([]dirEnc{{"backup.pxar", rootStart}}, nil, trailerStart)...)

	ptr := make([]byte, 8)
	binary.LittleEndian.PutUint64(ptr, trailerStart)
	return append(buf, ptr...)
}

func TestParseCatalogRoundTrip(t *testing.T) {
	entries, err := ParseCatalog(buildCatalog(t))
	if err != nil {
		t.Fatalf("ParseCatalog: %v", err)
	}

	got := make(map[string]CatalogEntry, len(entries))
	for _, e := range entries {
		if _, dup := got[e.Path]; dup {
			t.Errorf("duplicate path %q", e.Path)
		}
		got[e.Path] = e
	}

	want := []CatalogEntry{
		{Path: "A.txt", IsDir: false, Size: 10, ModTime: 111},
		{Path: "S", IsDir: true},
		{Path: "S/B.txt", IsDir: false, Size: 20, ModTime: 222},
		{Path: "empty", IsDir: true},
	}
	if len(entries) != len(want) {
		t.Fatalf("got %d entries, want %d: %+v", len(entries), len(want), entries)
	}
	for _, w := range want {
		g, ok := got[w.Path]
		if !ok {
			t.Errorf("missing entry %q", w.Path)
			continue
		}
		if g.IsDir != w.IsDir || g.Size != w.Size || g.ModTime != w.ModTime {
			t.Errorf("entry %q = %+v, want %+v", w.Path, g, w)
		}
	}
}

func TestParseCatalogRejectsBadMagic(t *testing.T) {
	buf := make([]byte, 32)
	copy(buf, "NOTPCAT1")
	if _, err := ParseCatalog(buf); !errors.Is(err, ErrNotCatalog) {
		t.Fatalf("expected ErrNotCatalog, got %v", err)
	}
}

func TestParseCatalogRejectsTruncated(t *testing.T) {
	if _, err := ParseCatalog(catalog_magic); err == nil {
		t.Fatal("expected error for too-short catalog")
	}
}

// A table whose declared entry count is enormous must yield a parse error, not
// an out-of-memory make panic (the slices are no longer preallocated by count).
func TestParseCatalogRejectsHugeCount(t *testing.T) {
	buf := append([]byte{}, catalog_magic...)

	tableStart := uint64(len(buf))
	var td []byte
	td = append_u64_7bit(td, 1<<60) // absurd entry count, no entries follow
	var table []byte
	table = append_u64_7bit(table, uint64(len(td)))
	table = append(table, td...)
	buf = append(buf, table...)

	ptr := make([]byte, 8)
	binary.LittleEndian.PutUint64(ptr, tableStart)
	buf = append(buf, ptr...)

	if _, err := ParseCatalog(buf); err == nil {
		t.Fatal("expected error for absurd entry count")
	}
}

func TestParseCatalogRejectsOutOfRangePointer(t *testing.T) {
	buf := append([]byte{}, catalog_magic...)
	buf = append(buf, make([]byte, 8)...) // some body
	ptr := make([]byte, 8)
	binary.LittleEndian.PutUint64(ptr, 0xffffffff) // pointer past the buffer
	buf = append(buf, ptr...)
	if _, err := ParseCatalog(buf); err == nil {
		t.Fatal("expected out-of-range root pointer to be rejected")
	}
}
