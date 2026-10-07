package pbscommon

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// memChunks stands in for the server's chunk store and one payload index
// writer: it chunks the payload stream like ChunkState does (boundary hints,
// forced boundaries, injected known chunks) and keeps every chunk's data.
type memChunks struct {
	store   map[[32]byte][]byte
	c       Chunker
	cur     []byte
	pos     uint64
	ends    []uint64
	digests [][32]byte
	reused  int
}

func newMemChunks(store map[[32]byte][]byte) *memChunks {
	m := &memChunks{store: store}
	m.c.New(64 * 1024) // small chunks so files share them
	return m
}

func (m *memChunks) emit() {
	d := sha256.Sum256(m.cur)
	m.store[d] = append([]byte(nil), m.cur...)
	m.pos += uint64(len(m.cur))
	m.ends = append(m.ends, m.pos)
	m.digests = append(m.digests, d)
	m.cur = nil
}

func (m *memChunks) write(b []byte) error {
	n := m.c.Scan(b)
	if n == 0 {
		m.cur = append(m.cur, b...)
		return nil
	}
	for n > 0 {
		m.cur = append(m.cur, b[:n]...)
		m.emit()
		b = b[n:]
		n = m.c.Scan(b)
	}
	m.cur = append(m.cur, b...)
	return nil
}

func (m *memChunks) suggest() error {
	if n := uint64(len(m.cur)); n >= m.c.MinSize() && n <= m.c.MaxSize() {
		m.emit()
		m.c.Reset()
	}
	return nil
}

func (m *memChunks) force() error {
	if len(m.cur) > 0 {
		m.emit()
	}
	m.c.Reset()
	return nil
}

func (m *memChunks) inject(d [32]byte, size uint64) error {
	data, ok := m.store[d]
	if !ok || uint64(len(data)) != size {
		return fmt.Errorf("injected chunk %x unknown or wrong size", d[:4])
	}
	if len(m.cur) != 0 {
		return fmt.Errorf("chunk injected while %d bytes are pending", len(m.cur))
	}
	m.pos += size
	m.ends = append(m.ends, m.pos)
	m.digests = append(m.digests, d)
	m.reused++
	return nil
}

func (m *memChunks) finish() {
	if len(m.cur) > 0 {
		m.emit()
	}
}

func (m *memChunks) stream() []byte {
	var b bytes.Buffer
	for _, d := range m.digests {
		b.Write(m.store[d])
	}
	return b.Bytes()
}

func (m *memChunks) didx() []byte {
	b := make([]byte, didxHeaderSize)
	copy(b, didxMagic)
	for i := range m.ends {
		var e [8]byte
		binary.LittleEndian.PutUint64(e[:], m.ends[i])
		b = append(b, e[:]...)
		b = append(b, m.digests[i][:]...)
	}
	return b
}

type splitRun struct {
	meta    []byte
	payload *memChunks
	reuse   *PayloadReuse
	opened  map[string]int
}

// runSplitBackup backs root up as a split archive, reusing from prev if set.
func runSplitBackup(t *testing.T, root string, store map[[32]byte][]byte, prev *splitRun) *splitRun {
	return runSplitBackupFailing(t, root, store, prev, nil)
}

// runSplitBackupFailing is runSplitBackup with reads of the named files
// failing (a non-device error) once they reach the given offset.
func runSplitBackupFailing(t *testing.T, root string, store map[[32]byte][]byte, prev *splitRun, failAt map[string]int64) *splitRun {
	t.Helper()
	r := &splitRun{payload: newMemChunks(store), opened: map[string]int{}}
	oldOpen := openForRead
	openForRead = func(p string) (io.ReadSeekCloser, error) {
		r.opened[filepath.Base(p)]++
		f, err := os.Open(p)
		if err != nil {
			return nil, err
		}
		if at, ok := failAt[filepath.Base(p)]; ok {
			return &flakyFile{f: f, failAt: at, err: errors.New("locked region")}, nil
		}
		return f, nil
	}
	defer func() { openForRead = oldOpen }()

	var meta bytes.Buffer
	a := &PXARArchive{ArchiveName: "t.mpxar.didx", Split: true}
	a.WriteCB = func(b []byte) error { meta.Write(b); return nil }
	a.PayloadWriteCB = r.payload.write
	a.OnPayloadFileStart = r.payload.suggest
	if prev != nil {
		lr, err := NewSplitMetadataReaderAt(bytes.NewReader(prev.meta), int64(len(prev.meta)))
		if err != nil {
			t.Fatal(err)
		}
		entries, err := lr.ListEntries()
		if err != nil {
			t.Fatal(err)
		}
		pa, err := NewPreviousSplitArchive(entries, prev.payload.didx())
		if err != nil {
			t.Fatal(err)
		}
		r.reuse = NewPayloadReuse(pa, r.payload.force, r.payload.inject)
		a.Reuse = r.reuse
	}
	if _, err := a.WriteDir(root, "", true); err != nil {
		t.Fatalf("WriteDir: %v", err)
	}
	r.payload.finish()
	r.meta = meta.Bytes()
	return r
}

// checkRestore restores run r and compares it with the tree at root.
func checkRestore(t *testing.T, label string, r *splitRun, root string) {
	t.Helper()
	payload := r.payload.stream()
	rd, err := NewSplitPXARReaderAt(bytes.NewReader(r.meta), int64(len(r.meta)), bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		t.Fatalf("%s: %v", label, err)
	}
	dest := t.TempDir()
	if _, err := rd.ExtractAll(dest); err != nil {
		t.Fatalf("%s: restore: %v", label, err)
	}
	n := 0
	filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		want, _ := os.ReadFile(p)
		got, gerr := os.ReadFile(filepath.Join(dest, rel))
		if gerr != nil || !bytes.Equal(got, want) {
			t.Errorf("%s: %s restored %d bytes (err %v), want %d", label, rel, len(got), gerr, len(want))
		}
		n++
		return nil
	})
	if n == 0 {
		t.Fatalf("%s: empty tree", label)
	}
}

func writeRandomFile(t *testing.T, path string, n int, mt time.Time) {
	t.Helper()
	b := make([]byte, n)
	rand.Read(b)
	os.MkdirAll(filepath.Dir(path), 0755)
	if err := os.WriteFile(path, b, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mt, mt); err != nil {
		t.Fatal(err)
	}
}

func reuseTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	mt := time.Date(2025, 5, 6, 7, 8, 9, 123456700, time.UTC)
	for i := 0; i < 40; i++ {
		writeRandomFile(t, filepath.Join(root, "small", fmt.Sprintf("s%02d.txt", i)), 1000+i*357, mt)
	}
	for i := 0; i < 6; i++ {
		writeRandomFile(t, filepath.Join(root, "big", fmt.Sprintf("b%d.bin", i)), 300*1024+i*77777, mt)
	}
	writeRandomFile(t, filepath.Join(root, "top.dat"), 200*1024, mt)
	writeRandomFile(t, filepath.Join(root, "empty.txt"), 0, mt)
	return root
}

func TestMetadataModeReusesUnchangedFiles(t *testing.T) {
	root := reuseTree(t)
	store := map[[32]byte][]byte{}

	r1 := runSplitBackup(t, root, store, nil)
	checkRestore(t, "run 1", r1, root)

	// Change two files, add one, delete one; everything else is untouched.
	later := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	writeRandomFile(t, filepath.Join(root, "big", "b2.bin"), 350*1024, later)
	writeRandomFile(t, filepath.Join(root, "small", "s07.txt"), 2222, later)
	writeRandomFile(t, filepath.Join(root, "small", "new.txt"), 4321, later)
	os.Remove(filepath.Join(root, "small", "s30.txt"))
	// Same size, content changed, mtime changed by one nanosecond only.
	p := filepath.Join(root, "big", "b4.bin")
	b, _ := os.ReadFile(p)
	b[0] ^= 0xff
	os.WriteFile(p, b, 0644)
	nmt := time.Date(2025, 5, 6, 7, 8, 9, 123456800, time.UTC)
	os.Chtimes(p, nmt, nmt)

	r2 := runSplitBackup(t, root, store, r1)
	checkRestore(t, "run 2", r2, root)
	for _, name := range []string{"b2.bin", "s07.txt", "new.txt", "b4.bin"} {
		if r2.opened[name] != 1 {
			t.Errorf("run 2: changed/new file %s opened %d times, want 1", name, r2.opened[name])
		}
	}
	for _, name := range []string{"b0.bin", "b1.bin", "b3.bin", "b5.bin", "top.dat"} {
		if r2.opened[name] != 0 {
			t.Errorf("run 2: unchanged %s was read", name)
		}
	}
	st := r2.reuse.Stats
	t.Logf("run 2: reused %d files, %d bytes, %d chunks, padding %d, read %d files, over-padding %d",
		st.Files, st.Bytes, st.Chunks, st.Padding, len(r2.opened), st.OverPadding)
	if st.Files < 40 {
		t.Errorf("run 2 reused only %d files", st.Files)
	}

	// Nothing changes: a backup of a backup that reused, padding and all.
	r3 := runSplitBackup(t, root, store, r2)
	checkRestore(t, "run 3", r3, root)
	st = r3.reuse.Stats
	t.Logf("run 3: reused %d files, %d chunks, padding %d, read %v", st.Files, st.Chunks, st.Padding, r3.opened)
	if len(r3.opened) != 0 || st.Files != 48 {
		t.Errorf("run 3: nothing changed, yet read %v and reused %d of 48 files", r3.opened, st.Files)
	}
	if st.Padding > uint64(0.1*float64(st.Bytes))+4*1024*1024 {
		t.Errorf("run 3 padding %d over budget for %d reused bytes", st.Padding, st.Bytes)
	}
}

// A planner over a synthetic previous archive: chunks of 100 bytes.
func synthPrev(files map[string][2]uint64, chunks int) *PreviousSplitArchive {
	p := &PreviousSplitArchive{Files: map[string]PreviousFile{}}
	for i := 0; i < chunks; i++ {
		p.ends = append(p.ends, uint64(i+1)*100)
		var d [32]byte
		d[0] = byte(i)
		p.digests = append(p.digests, d)
	}
	for name, r := range files {
		p.Files[name] = PreviousFile{Offset: r[0], Size: r[1]}
	}
	return p
}

func TestPayloadReuseRuns(t *testing.T) {
	// f1 [10,60) f2 [60,250) f3 [250,290) — record = 16-byte header + size.
	prev := synthPrev(map[string][2]uint64{"f1": {10, 34}, "f2": {60, 174}, "f3": {250, 24}}, 5)
	var injected []byte
	forced := 0
	r := NewPayloadReuse(prev, func() error { forced++; return nil }, func(d [32]byte, size uint64) error {
		if size != 100 {
			t.Errorf("chunk %d injected with size %d", d[0], size)
		}
		injected = append(injected, d[0])
		return nil
	})
	r.PaddingAllowance = 1000

	pos := uint64(16) // start marker already written
	off1, inj, ok, err := r.Reuse(prev.Files["f1"], pos)
	if err != nil || !ok || off1 != 16+10 || inj != 100 {
		t.Fatalf("f1: off %d inj %d ok %v err %v", off1, inj, ok, err)
	}
	pos += inj
	off2, inj, ok, _ := r.Reuse(prev.Files["f2"], pos)
	if !ok || off2 != 16+60 || inj != 200 {
		t.Fatalf("f2 continues the run: off %d inj %d ok %v", off2, inj, ok)
	}
	pos += inj
	off3, inj, ok, _ := r.Reuse(prev.Files["f3"], pos)
	if !ok || off3 != 16+250 || inj != 0 {
		t.Fatalf("f3 lies in an appended chunk: off %d inj %d ok %v", off3, inj, ok)
	}
	if forced != 1 || string(injected) != "\x00\x01\x02" {
		t.Fatalf("forced %d, injected %v", forced, injected)
	}

	// Going backwards (f1 again) starts a new run and appends chunk 0 again.
	off, inj, ok, _ := r.Reuse(prev.Files["f1"], pos)
	if !ok || forced != 2 || inj != 100 || off != pos+10 {
		t.Fatalf("non-monotonic reuse: off %d inj %d ok %v forced %d", off, inj, ok, forced)
	}
	if r.Stats.Files != 4 || r.Stats.Chunks != 4 {
		t.Errorf("stats %+v", r.Stats)
	}

	// New data written in between breaks the run.
	r.Break()
	if r.run {
		t.Fatal("run still open after Break")
	}
}

func TestPayloadReuseRespectsPaddingBudget(t *testing.T) {
	// A 4-byte file alone in the middle of a 100-byte chunk, no allowance.
	prev := synthPrev(map[string][2]uint64{"tiny": {140, 4}}, 3)
	r := NewPayloadReuse(prev, func() error { return nil }, func([32]byte, uint64) error { return nil })
	r.PaddingAllowance = 0
	if _, _, ok, _ := r.Reuse(prev.Files["tiny"], 0); ok {
		t.Fatal("reused a file whose chunk is 80% padding")
	}
	if r.Stats.OverPadding != 1 {
		t.Errorf("over-padding count %d", r.Stats.OverPadding)
	}
}

func TestPayloadReuseLookupNeedsExactMetadata(t *testing.T) {
	prev := &PreviousSplitArchive{Files: map[string]PreviousFile{
		"a": {Mode: 0o100644, UID: 1, GID: 2, Secs: 100, Nanos: 5, Size: 10},
	}}
	r := NewPayloadReuse(prev, nil, nil)
	if _, ok := r.Lookup("a", 0o100644, 1, 2, 100, 5, 10); !ok {
		t.Fatal("identical metadata not matched")
	}
	for i, c := range []struct {
		mode, uid, gid uint32
		secs           int64
		nanos          uint32
		size           uint64
	}{
		{0o100600, 1, 2, 100, 5, 10}, {0o100644, 9, 2, 100, 5, 10}, {0o100644, 1, 9, 100, 5, 10},
		{0o100644, 1, 2, 101, 5, 10}, {0o100644, 1, 2, 100, 6, 10}, {0o100644, 1, 2, 100, 5, 11},
	} {
		if _, ok := r.Lookup("a", c.mode, c.uid, c.gid, c.secs, c.nanos, c.size); ok {
			t.Errorf("case %d: changed metadata matched", i)
		}
	}
	if _, ok := r.Lookup("b", 0o100644, 1, 2, 100, 5, 10); ok {
		t.Error("unknown path matched")
	}
}

// A file zero-padded after a read error must not be reused from that
// snapshot: the next metadata-mode run reads it again in full.
func TestPaddedFileIsReadAgainNextRun(t *testing.T) {
	root := reuseTree(t)
	store := map[[32]byte][]byte{}
	r1 := runSplitBackup(t, root, store, nil)

	r2 := runSplitBackupFailing(t, root, store, r1, map[string]int64{"b3.bin": 1000})
	if r2.opened["b3.bin"] != 0 {
		// b3 is unchanged, so run 2 reuses it and never hits the failure.
		t.Fatalf("run 2 read unchanged b3.bin")
	}
	// Force the failure on a file run 2 has to read: change b3's content.
	later := time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC)
	writeRandomFile(t, filepath.Join(root, "big", "b3.bin"), 333*1024, later)
	want, _ := os.ReadFile(filepath.Join(root, "big", "b3.bin"))
	r3 := runSplitBackupFailing(t, root, store, r2, map[string]int64{"b3.bin": 1000})
	if r3.opened["b3.bin"] != 1 {
		t.Fatalf("run 3 opened b3.bin %d times", r3.opened["b3.bin"])
	}
	payload := r3.payload.stream()
	rd, _ := NewSplitPXARReaderAt(bytes.NewReader(r3.meta), int64(len(r3.meta)), bytes.NewReader(payload), int64(len(payload)))
	dest := t.TempDir()
	if _, err := rd.ExtractAll(dest); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(dest, "big", "b3.bin"))
	if len(got) != len(want) || !bytes.Equal(got[:1000], want[:1000]) || bytes.Count(got[1000:], []byte{0}) != len(got)-1000 {
		t.Fatalf("run 3: b3.bin is not the readable 1000 bytes plus zero padding")
	}

	// Run 4: the file is unchanged since run 3 and reads fine now. It must be
	// read again (not reused from run 3's padded copy) and restore in full.
	r4 := runSplitBackup(t, root, store, r3)
	if r4.opened["b3.bin"] != 1 {
		t.Fatalf("run 4 reused the zero-padded b3.bin instead of reading it (opened %d times)", r4.opened["b3.bin"])
	}
	checkRestore(t, "run 4", r4, root)
	if r4.reuse.Stats.Files != 47 {
		t.Errorf("run 4 reused %d files, want every other file (47)", r4.reuse.Stats.Files)
	}
}
