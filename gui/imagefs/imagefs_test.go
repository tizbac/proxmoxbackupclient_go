package imagefs

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"testing"
)

// The fixture is a fixed VHD made with diskpart on Windows 10: a GPT disk
// with a 40 MB NTFS partition (files below) and a FAT32 one. A fixed VHD is
// the raw disk followed by a 512-byte footer. populate.ps1 (kept with the
// session notes) wrote gpt-ntfs-fat32.expected.json from the mounted volume:
// a resident file with an alternate stream, a 5 MB file spanning two 4 MiB
// image chunks, a hard link, a Unicode name, an empty file, a sparse file,
// an NTFS-compressed file, and a deleted file (which must not be listed).
func loadFixture(t *testing.T) (*bytes.Reader, int64) {
	t.Helper()
	f, err := os.Open("testdata/gpt-ntfs-fat32.vhd.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	return bytes.NewReader(b), int64(len(b)) - 512
}

type expected struct {
	Path    string `json:"path"`
	IsDir   bool   `json:"isDir"`
	Size    uint64 `json:"size"`
	SHA256  string `json:"sha256"`
	MTime   int64  `json:"mtime"`
	Created int64  `json:"created"`
}

func TestReadPartitionsGPT(t *testing.T) {
	r, size := loadFixture(t)
	parts, err := ReadPartitions(r, size)
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 2 {
		t.Fatalf("got %d partitions: %+v", len(parts), parts)
	}
	if p := parts[0]; p.Scheme != "gpt" || p.FS != "ntfs" || p.Type != "Microsoft basic data" || p.Size != 40<<20 {
		t.Errorf("partition 1 = %+v", p)
	}
	if p := parts[1]; p.FS != "fat32" || p.Start < parts[0].Start+parts[0].Size {
		t.Errorf("partition 2 = %+v", p)
	}
	if l := parts[0].Label(); l != "Partition 1 (NTFS, 41.9 MB)" {
		t.Errorf("label %q", l)
	}
}

func TestNTFSListAndRead(t *testing.T) {
	r, size := loadFixture(t)
	parts, err := ReadPartitions(r, size)
	if err != nil {
		t.Fatal(err)
	}
	vol, err := OpenNTFS(io.NewSectionReader(r, parts[0].Start, parts[0].Size))
	if err != nil {
		t.Fatal(err)
	}
	var lastDone, lastTotal int64
	entries, err := vol.List(nil, func(d, tot int64) { lastDone, lastTotal = d, tot })
	if err != nil {
		t.Fatal(err)
	}
	if lastTotal == 0 || lastDone != lastTotal {
		t.Errorf("progress ended at %d of %d", lastDone, lastTotal)
	}

	raw, err := os.ReadFile("testdata/gpt-ntfs-fat32.expected.json")
	if err != nil {
		t.Fatal(err)
	}
	raw = bytes.TrimPrefix(raw, []byte("\xef\xbb\xbf"))
	var want []expected
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}

	got := map[string]Entry{}
	for _, e := range entries {
		got[e.Path] = e
	}
	for _, w := range want {
		e, ok := got[w.Path]
		if !ok {
			t.Errorf("%s: not listed", w.Path)
			continue
		}
		if e.IsDir != w.IsDir || (!w.IsDir && e.Size != w.Size) {
			t.Errorf("%s: dir=%v size=%d, want dir=%v size=%d", w.Path, e.IsDir, e.Size, w.IsDir, w.Size)
		}
		// A folder's times are compared only for files: Windows reports a
		// folder's times from its parent's directory index, which it updates
		// later than the folder's own record, and the record is what we read.
		if !w.IsDir && (e.MTime != w.MTime || e.Created != w.Created) {
			t.Errorf("%s: mtime %d created %d, want %d %d", w.Path, e.MTime, e.Created, w.MTime, w.Created)
		}
		if w.IsDir {
			continue
		}
		f, err := vol.Open(e.Record, int64(e.Size))
		if err != nil {
			t.Errorf("%s: open: %v", w.Path, err)
			continue
		}
		h := sha256.New()
		if _, err := io.Copy(h, io.NewSectionReader(f, 0, f.Size())); err != nil {
			t.Errorf("%s: read: %v", w.Path, err)
			continue
		}
		if s := hex.EncodeToString(h.Sum(nil)); s != w.SHA256 {
			t.Errorf("%s: sha256 %s, want %s", w.Path, s, w.SHA256)
		}
	}
	for _, e := range entries {
		switch {
		case e.Path == "deleted.txt":
			t.Error("a deleted file is listed")
		case len(e.Path) > 0 && e.Path[0] == '$':
			t.Errorf("metafile listed: %s", e.Path)
		}
	}
	if a, b := got["docs/report.docx"], got["docs/link-to-report.docx"]; a.Record == 0 || a.Record != b.Record {
		t.Errorf("hard links should share a record: %d, %d", a.Record, b.Record)
	}
	if got["hello.txt"].MTime != 132224078450000000 { // 2020-01-02 03:04:05 UTC
		t.Errorf("hello.txt mtime %d", got["hello.txt"].MTime)
	}
	if UnixSeconds(got["hello.txt"].MTime) != 1577934245 {
		t.Errorf("UnixSeconds = %d", UnixSeconds(got["hello.txt"].MTime))
	}
}

func TestReadPartitionsMBRWithLogical(t *testing.T) {
	disk := make([]byte, 64<<20)
	put32 := func(b []byte, v uint32) { b[0], b[1], b[2], b[3] = byte(v), byte(v>>8), byte(v>>16), byte(v>>24) }
	entry := func(b []byte, typ byte, lba, sectors uint32) { b[4] = typ; put32(b[8:], lba); put32(b[12:], sectors) }
	// Primary NTFS at 1 MiB (16 MiB), extended from 20 MiB holding two logicals.
	entry(disk[446:], 0x07, 2048, 32768)
	entry(disk[462:], 0x05, 40960, 40960)
	disk[510], disk[511] = 0x55, 0xAA
	copy(disk[2048*512+3:], "NTFS    ")
	ebr1 := disk[40960*512:]
	entry(ebr1[446:], 0x83, 2048, 8192)   // logical 5 at 21 MiB
	entry(ebr1[462:], 0x05, 12288, 20480) // next EBR at extended + 6 MiB
	ebr1[510], ebr1[511] = 0x55, 0xAA
	ebr2 := disk[(40960+12288)*512:]
	entry(ebr2[446:], 0x0B, 2048, 8192)
	ebr2[510], ebr2[511] = 0x55, 0xAA

	parts, err := ReadPartitions(bytes.NewReader(disk), int64(len(disk)))
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 3 {
		t.Fatalf("got %+v", parts)
	}
	if parts[0].FS != "ntfs" || parts[0].Start != 1<<20 || parts[0].Number != 1 {
		t.Errorf("primary %+v", parts[0])
	}
	if parts[1].Number != 5 || parts[1].Start != (40960+2048)*512 || parts[1].Type != "Linux" {
		t.Errorf("logical 5 %+v", parts[1])
	}
	if parts[2].Number != 6 || parts[2].Start != (40960+12288+2048)*512 || parts[2].Type != "FAT32" {
		t.Errorf("logical 6 %+v", parts[2])
	}
}

func TestNoPartitionTable(t *testing.T) {
	if _, err := ReadPartitions(bytes.NewReader(make([]byte, 8192)), 8192); err != ErrNoPartitionTable {
		t.Fatalf("err = %v", err)
	}
}
