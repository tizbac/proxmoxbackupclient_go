package pbscommon

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// splitOpts controls how makeSplitPXAR lays out the payload stream.
type splitOpts struct {
	// padding is written before each file's PAYLOAD record, the way reused
	// chunks from a previous snapshot leave unreferenced bytes in the stream.
	padding []byte
}

// makeSplitPXAR serialises entries into a split (format version 2) archive:
// the metadata stream carries PXAR_FORMAT_VERSION, an optional PRELUDE, then
// the usual tree with a PXAR_PAYLOAD_REF per file; the payload stream carries
// a start marker, a PXAR_PAYLOAD record per file (with optional padding
// before each) and a tail marker.
func makeSplitPXAR(t *testing.T, opts splitOpts, entries ...pxarEntry) (meta, payload []byte) {
	t.Helper()

	payload = pxarSection(t, PXAR_PAYLOAD_START_MARKER, nil)

	var emit func(nodes []*pxarNode) []byte
	emit = func(nodes []*pxarNode) []byte {
		var out []byte
		for _, n := range nodes {
			out = append(out, pxarSection(t, PXAR_FILENAME, []byte(n.entry.name))...)
			out = append(out, pxarSection(t, PXAR_ENTRY, pxarEntryPayload(t, n.entry.mode, n.entry.mtime))...)
			if n.entry.isDir {
				out = append(out, emit(n.children)...)
				out = append(out, pxarSection(t, PXAR_GOODBYE, nil)...)
				continue
			}
			payload = append(payload, opts.padding...)
			ref := make([]byte, 16)
			binary.LittleEndian.PutUint64(ref[0:8], uint64(len(payload)))
			binary.LittleEndian.PutUint64(ref[8:16], uint64(len(n.entry.data)))
			payload = append(payload, pxarSection(t, PXAR_PAYLOAD, n.entry.data)...)
			out = append(out, pxarSection(t, PXAR_PAYLOAD_REF, ref)...)
		}
		return out
	}

	version := make([]byte, 8)
	binary.LittleEndian.PutUint64(version, 2)
	meta = pxarSection(t, PXAR_FORMAT_VERSION, version)
	meta = append(meta, pxarSection(t, PXAR_PRELUDE, []byte(`{"exclude-patterns":""}`))...)
	meta = append(meta, pxarSection(t, PXAR_ENTRY, pxarEntryPayload(t, IFDIR|0755, 1600000000))...)
	meta = append(meta, emit(pxarTree(entries))...)
	meta = append(meta, pxarSection(t, PXAR_GOODBYE, nil)...)
	payload = append(payload, pxarSection(t, PXAR_PAYLOAD_TAIL_MARKER, nil)...)
	return meta, payload
}

func standardEntries() []pxarEntry {
	return []pxarEntry{
		pxarFile("hello.txt", []byte("hello world\n")),
		pxarDir("empty-dir"),
		pxarFile("sub/other.txt", []byte("other\n")),
		pxarFile("sub/nested/deep.txt", []byte("deep\n")),
		pxarFile("notes/file with spaces & ünicode.txt", []byte("unicode\n")),
		pxarFile("zero.bin", nil),
	}
}

func splitReader(t *testing.T, meta, payload []byte) *PXARReader {
	t.Helper()
	r, err := NewSplitPXARReaderAt(bytes.NewReader(meta), int64(len(meta)), bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		t.Fatalf("NewSplitPXARReaderAt: %v", err)
	}
	if !r.IsSplit() {
		t.Fatal("IsSplit() = false for a split reader")
	}
	return r
}

type listedEntry struct {
	path  string
	isDir bool
	size  uint64
}

func listOf(t *testing.T, r *PXARReader) []listedEntry {
	t.Helper()
	es, err := r.ListEntries()
	if err != nil {
		t.Fatalf("ListEntries: %v", err)
	}
	out := make([]listedEntry, 0, len(es))
	for _, e := range es {
		out = append(out, listedEntry{e.Path, e.IsDir, e.Size})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].path < out[j].path })
	return out
}

// A split archive must list exactly what the same tree lists as a classic one.
func TestSplitPXARListsLikeClassic(t *testing.T) {
	for _, pad := range [][]byte{nil, bytes.Repeat([]byte{0xAB}, 777)} {
		meta, payload := makeSplitPXAR(t, splitOpts{padding: pad}, standardEntries()...)
		got := listOf(t, splitReader(t, meta, payload))
		want := listOf(t, NewPXARReader(makePXAR(t, standardEntries()...)))
		if len(got) != len(want) {
			t.Fatalf("padding %d: %d entries, want %d\n got %v\nwant %v", len(pad), len(got), len(want), got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("padding %d: entry %d = %+v, want %+v", len(pad), i, got[i], want[i])
			}
		}
	}
}

// Extraction (sequential and parallel) must write the referenced bytes.
func TestSplitPXARExtract(t *testing.T) {
	meta, payload := makeSplitPXAR(t, splitOpts{padding: []byte("PADDING-NOT-FILE-DATA")}, standardEntries()...)
	for _, parallel := range []bool{false, true} {
		dest := t.TempDir()
		r := splitReader(t, meta, payload)
		rewriter := func(p string) string { return filepath.Join(dest, filepath.FromSlash(p)) }
		var err error
		if parallel {
			_, err = r.ExtractWithRewriterParallel(rewriter, nil, true, 4)
		} else {
			_, err = r.ExtractWithRewriter(rewriter, nil, true)
		}
		if err != nil {
			t.Fatalf("parallel=%v: extract: %v", parallel, err)
		}
		for _, e := range standardEntries() {
			if e.isDir {
				continue
			}
			b, err := os.ReadFile(filepath.Join(dest, filepath.FromSlash(e.name)))
			if err != nil {
				t.Fatalf("parallel=%v: read %s: %v", parallel, e.name, err)
			}
			if !bytes.Equal(b, e.data) {
				t.Errorf("parallel=%v: %s = %q, want %q", parallel, e.name, b, e.data)
			}
		}
	}
}

// Selective extraction of a subtree from a split archive.
func TestSplitPXARExtractFiltered(t *testing.T) {
	meta, payload := makeSplitPXAR(t, splitOpts{padding: []byte("xx")}, standardEntries()...)
	dest := t.TempDir()
	r := splitReader(t, meta, payload)
	if _, err := r.ExtractFiltered(dest, []string{"sub"}, true); err != nil {
		t.Fatalf("ExtractFiltered: %v", err)
	}
	if b, err := os.ReadFile(filepath.Join(dest, "sub", "nested", "deep.txt")); err != nil || string(b) != "deep\n" {
		t.Fatalf("sub/nested/deep.txt = %q, %v", b, err)
	}
	if _, err := os.Stat(filepath.Join(dest, "hello.txt")); !os.IsNotExist(err) {
		t.Fatalf("hello.txt should not have been extracted (err=%v)", err)
	}
}

// The root virtual meta file must be readable from a split archive too.
func TestSplitPXARReadVirtualFile(t *testing.T) {
	meta, payload := makeSplitPXAR(t, splitOpts{}, pxarFile(".proxmox_backup_client_meta.json", []byte(`{"x":1}`)), pxarFile("a.txt", []byte("a")))
	b, err := splitReader(t, meta, payload).ReadVirtualFile(".proxmox_backup_client_meta.json")
	if err != nil || string(b) != `{"x":1}` {
		t.Fatalf("ReadVirtualFile = %q, %v", b, err)
	}
}

// Stream headers are checked when the reader is created.
func TestSplitPXARRejectsBadStreams(t *testing.T) {
	meta, payload := makeSplitPXAR(t, splitOpts{}, standardEntries()...)
	open := func(m, p []byte) error {
		_, err := NewSplitPXARReaderAt(bytes.NewReader(m), int64(len(m)), bytes.NewReader(p), int64(len(p)))
		return err
	}
	if err := open(meta, payload); err != nil {
		t.Fatalf("valid pair rejected: %v", err)
	}
	classic := makePXAR(t, standardEntries()...)
	if err := open(classic, payload); err == nil {
		t.Error("a classic archive was accepted as a metadata stream")
	}
	if err := open(meta, meta); err == nil {
		t.Error("a metadata stream was accepted as a payload stream")
	}
	v3 := append([]byte(nil), meta...)
	binary.LittleEndian.PutUint64(v3[16:24], 3)
	if err := open(v3, payload); err == nil || !strings.Contains(err.Error(), "version 3") {
		t.Errorf("format version 3 not rejected clearly: %v", err)
	}
	if err := open(meta, payload[:8]); err == nil {
		t.Error("a truncated payload stream was accepted")
	}
}

// A reference that does not land on a matching PAYLOAD header (into padding,
// past the end, or with the wrong size) must fail, never return wrong bytes.
func TestSplitPXARRejectsBadReferences(t *testing.T) {
	entries := []pxarEntry{pxarFile("a.txt", []byte("aaaa")), pxarFile("b.txt", []byte("bbbbbbbb"))}
	meta, payload := makeSplitPXAR(t, splitOpts{padding: bytes.Repeat([]byte{0}, 64)}, entries...)

	refPos := bytes.Index(meta, func() []byte {
		b := make([]byte, 8)
		binary.LittleEndian.PutUint64(b, PXAR_PAYLOAD_REF)
		return b
	}())
	if refPos < 0 {
		t.Fatal("no PAYLOAD_REF in fixture")
	}
	corrupt := func(offset, size uint64) error {
		m := append([]byte(nil), meta...)
		binary.LittleEndian.PutUint64(m[refPos+16:refPos+24], offset)
		binary.LittleEndian.PutUint64(m[refPos+24:refPos+32], size)
		_, err := splitReader(t, m, payload).ListEntries()
		return err
	}
	cases := map[string][2]uint64{
		"into padding":  {16 + 8, 4},
		"past the end":  {uint64(len(payload)) + 100, 4},
		"wrong size":    {16 + 64, 5},
		"size overflow": {16 + 64, ^uint64(0) - 8},
	}
	for name, c := range cases {
		if err := corrupt(c[0], c[1]); err == nil {
			t.Errorf("%s: corrupt reference accepted", name)
		}
	}
}

// A payload reference inside a classic (non-split) archive is an error.
func TestClassicPXARRejectsPayloadRef(t *testing.T) {
	ref := make([]byte, 16)
	archive := pxarSection(t, PXAR_ENTRY, pxarEntryPayload(t, IFDIR|0755, 1600000000))
	archive = append(archive, pxarSection(t, PXAR_FILENAME, []byte("x"))...)
	archive = append(archive, pxarSection(t, PXAR_ENTRY, pxarEntryPayload(t, IFREG|0644, 1))...)
	archive = append(archive, pxarSection(t, PXAR_PAYLOAD_REF, ref)...)
	archive = append(archive, pxarSection(t, PXAR_GOODBYE, nil)...)
	if _, err := NewPXARReader(archive).ListEntries(); err == nil {
		t.Fatal("payload reference in a classic archive was accepted")
	}
}

