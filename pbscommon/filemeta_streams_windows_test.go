//go:build windows

package pbscommon

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Alternate data streams and extended attributes on a file and a folder are
// recorded by the collector, survive the blob's JSON round trip, and are
// written back onto fresh copies.
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
	big := bytes.Repeat([]byte("x"), MaxStreamBytes+1)
	must(os.WriteFile(file+":big", big, 0o644))
	if err := setEAs(file, []EAEntry{{Name: "TESTEA", Value: []byte("hello")}, {Name: "SECOND", Value: []byte{0, 1, 2}}}); err != nil {
		t.Skipf("cannot set extended attributes here: %v", err)
	}

	c := NewNTFSMetaCollector(src, "h")
	for _, p := range []struct {
		path  string
		isDir bool
	}{{file, false}, {dir, true}} {
		fi, err := os.Lstat(p.path)
		must(err)
		must(c.Collect(p.path, fi, p.isDir))
	}
	meta, err := c.FinalizeRaw()
	must(err)
	streams, skipped, eas := c.StreamStats()
	if streams != 3 || skipped != 1 || eas != 2 {
		t.Errorf("stats: %d streams, %d skipped, %d EAs; want 3, 1, 2", streams, skipped, eas)
	}

	// Through the blob format and back.
	blob, dropped, err := (&CombinedBackupFileMeta{Version: FileMetaVersion, Archives: map[string]*BackupFileMeta{"a": meta}}).SerializeCapped(MaxFileMetaBlobBytes)
	must(err)
	if dropped != 0 {
		t.Errorf("dropped %d streams under the limit", dropped)
	}
	zr, err := gzip.NewReader(bytes.NewReader(blob))
	must(err)
	var back CombinedBackupFileMeta
	must(json.NewDecoder(zr).Decode(&back))
	byPath := map[string]FileMetaEntry{}
	for _, e := range back.Archives["a"].Entries {
		byPath[e.Path] = e
	}

	fe := byPath["download.exe"]
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

	// Onto fresh copies.
	dst := t.TempDir()
	dfile := filepath.Join(dst, "download.exe")
	ddir := filepath.Join(dst, "folder")
	must(os.WriteFile(dfile, []byte("main content"), 0o644))
	must(os.Mkdir(ddir, 0o755))
	w, sk, err := WriteStreamsAndEAs(dfile, fe.Streams, fe.EAs)
	if err != nil || w != 2 || sk != 1 {
		t.Errorf("file: wrote %d, skipped %d, err %v; want 2, 1, nil", w, sk, err)
	}
	if w, _, err := WriteStreamsAndEAs(ddir, byPath["folder"].Streams, nil); err != nil || w != 1 {
		t.Errorf("folder: wrote %d, err %v", w, err)
	}
	got, err := os.ReadFile(dfile + ":Zone.Identifier")
	if err != nil || !bytes.Equal(got, zone) {
		t.Errorf("restored Zone.Identifier: %q, %v", got, err)
	}
	if got, err := os.ReadFile(ddir + ":folderstream"); err != nil || string(got) != "on a folder" {
		t.Errorf("restored folder stream: %q, %v", got, err)
	}
	if main, _ := os.ReadFile(dfile); string(main) != "main content" {
		t.Errorf("main content changed: %q", main)
	}
	budget := int64(MaxStreamBytesTotal)
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

// The budget limits how much stream content one archive records.
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
}

// Over the blob limit, stream contents are dropped and everything else kept.
func TestSerializeCappedDropsStreams(t *testing.T) {
	m := &CombinedBackupFileMeta{Archives: map[string]*BackupFileMeta{"a": {SDDLs: []string{"O:BA"}, Entries: []FileMetaEntry{
		{Path: "x", Attrs: 32, Streams: []StreamEntry{{Name: "s", Size: 3, Data: []byte("abc")}}},
	}}}}
	blob, dropped, err := m.SerializeCapped(10)
	if err != nil || dropped != 1 || len(blob) == 0 {
		t.Fatalf("dropped %d, err %v", dropped, err)
	}
	s := m.Archives["a"].Entries[0].Streams[0]
	if s.Data != nil || s.Skipped == "" || m.Archives["a"].Entries[0].Attrs != 32 {
		t.Errorf("after drop: %+v", m.Archives["a"].Entries[0])
	}
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
