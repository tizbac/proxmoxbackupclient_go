//go:build windows

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Alternate data streams and extended attributes on a file and a folder are
// recorded by the collector (on the same handle as the ACL and attributes),
// survive the side file's gzip and JSON round trip, and are written back onto
// fresh copies without touching the main content or the parent folder's time.
func TestStreamsAndEAsRoundTrip(t *testing.T) {
	src := t.TempDir()
	file := filepath.Join(src, "download.exe")
	dir := filepath.Join(src, "folder")
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(os.WriteFile(file, []byte("main content"), 0o644))
	must(os.Mkdir(dir, 0o755))
	zone := []byte("[ZoneTransfer]\r\nZoneId=3\r\nHostUrl=https://example.com/download.exe\r\n")
	must(os.WriteFile(file+":Zone.Identifier", zone, 0o644))
	must(os.WriteFile(file+":empty", nil, 0o644))
	must(os.WriteFile(dir+":folderstream", []byte("on a folder"), 0o644))
	big := bytes.Repeat([]byte("x"), maxStreamBytes+1)
	must(os.WriteFile(file+":big", big, 0o644))
	if err := setEAs(file, []EAEntry{{Name: "TESTEA", Value: []byte("hello")}, {Name: "SECOND", Value: []byte{0, 1, 2}}}); err != nil {
		t.Skipf("cannot set extended attributes here: %v", err)
	}
	pathW, _ := windows.UTF16PtrFromString(file)
	must(windows.SetFileAttributes(pathW, windows.FILE_ATTRIBUTE_HIDDEN|windows.FILE_ATTRIBUTE_ARCHIVE))

	meta := collectForTest(t, src, map[string]bool{file: false, dir: true})
	byPath := buildFileMetaIndex(meta)

	// The one handle still gives the same ACL and attributes as the lookups
	// by name.
	wantSDDL, wantAttrs := securityByPath(file)
	fe := byPath["download.exe"]
	if got := meta.SDDLs[fe.SDDLIdx]; got != wantSDDL || fe.Attrs != wantAttrs {
		t.Errorf("security: %q %#x, want %q %#x", got, fe.Attrs, wantSDDL, wantAttrs)
	}
	if fe.Attrs&windows.FILE_ATTRIBUTE_HIDDEN == 0 {
		t.Errorf("hidden attribute not recorded: %#x", fe.Attrs)
	}

	names := map[string]StreamEntry{}
	for _, s := range fe.Streams {
		names[s.Name] = s
	}
	if !bytes.Equal(names["Zone.Identifier"].Data, zone) {
		t.Errorf("Zone.Identifier recorded as %q", names["Zone.Identifier"].Data)
	}
	if s, ok := names["empty"]; !ok || s.Skipped != "" || len(s.Data) != 0 {
		t.Errorf("empty stream: %+v (want recorded, zero bytes)", s)
	}
	if s := names["big"]; s.Data != nil || !strings.Contains(s.Skipped, "larger") || s.Size != int64(len(big)) {
		t.Errorf("big stream: size %d, skipped %q, data %d bytes", s.Size, s.Skipped, len(s.Data))
	}
	if len(fe.EAs) != 2 {
		t.Fatalf("EAs: %+v", fe.EAs)
	}
	if len(byPath["folder"].Streams) != 1 {
		t.Fatalf("folder streams: %+v", byPath["folder"].Streams)
	}

	// Onto fresh copies.
	dst := t.TempDir()
	dfile := filepath.Join(dst, "download.exe")
	ddir := filepath.Join(dst, "folder")
	must(os.WriteFile(dfile, []byte("main content"), 0o644))
	must(os.Mkdir(ddir, 0o755))
	old := time.Date(2015, 3, 4, 5, 6, 7, 0, time.UTC)
	must(os.Chtimes(dst, old, old))
	w, sk, err := writeStreamsAndEAs(dfile, fe.Streams, fe.EAs)
	if err != nil || w != 2 || sk != 1 {
		t.Errorf("file: wrote %d, skipped %d, err %v; want 2, 1, nil", w, sk, err)
	}
	if w, _, err := writeStreamsAndEAs(ddir, byPath["folder"].Streams, nil); err != nil || w != 1 {
		t.Errorf("folder: wrote %d, err %v", w, err)
	}
	if fi, _ := os.Stat(dst); !fi.ModTime().Equal(old) {
		t.Errorf("parent folder time changed to %v by writing streams on its contents", fi.ModTime())
	}
	got, err := os.ReadFile(dfile + ":Zone.Identifier")
	if err != nil || !bytes.Equal(got, zone) {
		t.Errorf("restored Zone.Identifier: %q, %v", got, err)
	}
	if fi, err := os.Stat(dfile + ":empty"); err != nil || fi.Size() != 0 {
		t.Errorf("restored empty stream: %v", err)
	}
	if _, err := os.Stat(dfile + ":big"); err == nil {
		t.Errorf("a stream recorded by name only was created")
	}
	if got, err := os.ReadFile(ddir + ":folderstream"); err != nil || string(got) != "on a folder" {
		t.Errorf("restored folder stream: %q, %v", got, err)
	}
	if main, _ := os.ReadFile(dfile); string(main) != "main content" {
		t.Errorf("main content changed: %q", main)
	}
	budget := int64(maxStreamBytesTotal)
	_, readEAs, err := readStreamsAndEAs(dfile, &budget)
	must(err)
	vals := map[string]string{}
	for _, e := range readEAs {
		vals[e.Name] = string(e.Value)
	}
	if vals["TESTEA"] != "hello" || vals["SECOND"] != "\x00\x01\x02" {
		t.Errorf("restored EAs: %q", vals)
	}
}

// The collector counts what it recorded, and the archive's budget limits how
// much stream content it records.
func TestStreamBudget(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "f")
	if err := os.WriteFile(f, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f+":s", []byte("12345"), 0o644); err != nil {
		t.Fatal(err)
	}
	budget := int64(4)
	streams, _, err := readStreamsAndEAs(f, &budget)
	if err != nil || len(streams) != 1 || streams[0].Data != nil || streams[0].Skipped == "" || budget != 4 {
		t.Errorf("over budget: %+v, budget %d, err %v", streams, budget, err)
	}
	budget = 10
	streams, _, _ = readStreamsAndEAs(f, &budget)
	if len(streams) != 1 || string(streams[0].Data) != "12345" || budget != 5 {
		t.Errorf("within budget: %+v, budget %d", streams, budget)
	}

	c := NewNTFSMetaCollector(dir, "h")
	c.streamBudget = 4
	fi, _ := os.Lstat(f)
	if err := c.Collect(f, fi, false); err != nil {
		t.Fatal(err)
	}
	if s, sk, _, _ := c.StreamStats(); s != 0 || sk != 1 || c.streamBudget != 4 {
		t.Errorf("collector over budget: %d recorded, %d by name, budget %d", s, sk, c.streamBudget)
	}
}

// Over the blob limit, stream contents are dropped and everything else kept.
func TestSerializeFileMetaCappedDropsStreams(t *testing.T) {
	m := &BackupFileMeta{SDDLs: []string{"O:BA"}, Entries: []FileMetaEntry{
		{Path: "x", Attrs: 32, Created: 5, Streams: []StreamEntry{{Name: "s", Size: 3, Data: []byte("abc")}, {Name: "t", Size: 9, Skipped: "larger"}}},
	}}
	_, dropped, err := serializeFileMetaCapped(m, 1<<20)
	if err != nil || dropped != 0 || m.Entries[0].Streams[0].Data == nil {
		t.Fatalf("under the limit: dropped %d, err %v", dropped, err)
	}
	blob, dropped, err := serializeFileMetaCapped(m, 10)
	if err != nil || dropped != 1 || len(blob) == 0 {
		t.Fatalf("dropped %d, err %v", dropped, err)
	}
	e := m.Entries[0]
	if e.Streams[0].Data != nil || e.Streams[0].Skipped == "" || e.Streams[1].Skipped != "larger" || e.Attrs != 32 || e.Created != 5 {
		t.Errorf("after drop: %+v", e)
	}
}

// Buffers for FileStreamInfo and the EA calls are 8-byte aligned whatever
// their size: an unaligned one makes the call fail with ERROR_NOACCESS, and a
// []byte from make is aligned only some of the time.
func TestAlignedBuffer(t *testing.T) {
	f := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f+":s", []byte("12345"), 0o644); err != nil {
		t.Fatal(err)
	}
	h, err := openMetaHandle(f)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(h)
	base := alignedBuffer(4096 + 1)
	if err := windows.GetFileInformationByHandleEx(h, windows.FileStreamInfo, &base[1], 4096); err != windows.ERROR_NOACCESS {
		t.Logf("unaligned buffer: %v (expected ERROR_NOACCESS)", err)
	}
	if s := streamNames(h); len(s) != 1 || s[0].name != "s" || s[0].size != 5 {
		t.Errorf("streamNames = %+v", s)
	}

	var keep [][]byte
	for n := 1; n < 4096; n += 7 {
		b := alignedBuffer(n)
		if len(b) != n {
			t.Fatalf("len %d, want %d", len(b), n)
		}
		if p := uintptr(unsafe.Pointer(&b[0])); p%8 != 0 {
			t.Fatalf("buffer of %d bytes at %#x is not 8-byte aligned", n, p)
		}
		keep = append(keep, b, make([]byte, n%5+1)) // odd allocations in between
	}
	_ = keep
}

// The EA buffer aligns every entry on 4 bytes and ends with offset 0.
func TestBuildEABuffer(t *testing.T) {
	b := buildEABuffer([]EAEntry{{Name: "A", Value: []byte("1")}, {Name: "BB", Value: []byte("22")}, {Name: ""}})
	first := int(b[0]) | int(b[1])<<8
	if first%4 != 0 || first == 0 {
		t.Fatalf("first NextEntryOffset %d", first)
	}
	if b[first+5] != 2 || string(b[first+8:first+10]) != "BB" {
		t.Errorf("second entry: % x", b[first:])
	}
	if b[first] != 0 || b[first+1] != 0 {
		t.Errorf("last entry's offset is not 0")
	}
}
