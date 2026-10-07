package pbscommon

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// recordingReaderAt records the byte range of every ReadAt, so a test can
// prove a selective restore never touched an unselected file's payload.
type recordingReaderAt struct {
	ra    io.ReaderAt
	mu    sync.Mutex
	reads [][2]int64
}

func (r *recordingReaderAt) ReadAt(p []byte, off int64) (int, error) {
	r.mu.Lock()
	r.reads = append(r.reads, [2]int64{off, off + int64(len(p))})
	r.mu.Unlock()
	return r.ra.ReadAt(p, off)
}

func (r *recordingReaderAt) take() [][2]int64 {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := r.reads
	r.reads = nil
	return out
}

func selectiveEntries() []pxarEntry {
	return []pxarEntry{
		pxarFile("a/one.txt", bytes.Repeat([]byte("A"), 100)),
		pxarFile("a/two.txt", bytes.Repeat([]byte("a"), 50)),
		pxarFile("b/big.bin", bytes.Repeat([]byte("B"), 5000)),
		pxarFile("b/sub/x.txt", []byte("bx")),
		pxarFile("c/three.txt", bytes.Repeat([]byte("C"), 70)),
		pxarFile("d/four.txt", []byte("dddd")),
	}
}

// payloadRangesOf maps each file of a split archive to the payload bytes it
// owns (its PAYLOAD header plus content), read from the metadata alone.
func payloadRangesOf(t *testing.T, meta []byte) map[string][2]int64 {
	t.Helper()
	mr, err := NewSplitMetadataReaderAt(bytes.NewReader(meta), int64(len(meta)))
	if err != nil {
		t.Fatal(err)
	}
	entries, err := mr.ListEntries()
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][2]int64{}
	for _, e := range entries {
		if e.HasPayloadRef {
			out[e.Path] = [2]int64{int64(e.PayloadOffset), int64(e.PayloadOffset) + 16 + int64(e.Size)}
		}
	}
	return out
}

// assertReadsInside fails for any recorded read that is not wholly inside the
// payload of a file under one of the selected prefixes.
func assertReadsInside(t *testing.T, label string, reads [][2]int64, owned map[string][2]int64, selected ...string) {
	t.Helper()
	for _, rd := range reads {
		ok := false
		for path, rg := range owned {
			sel := false
			for _, s := range selected {
				if path == s || strings.HasPrefix(path, s+"/") {
					sel = true
				}
			}
			if sel && rd[0] >= rg[0] && rd[1] <= rg[1] {
				ok = true
				break
			}
		}
		if !ok {
			t.Errorf("%s: payload read [%d,%d) lies outside the selected files", label, rd[0], rd[1])
		}
	}
}

// The 2026-10-07 regression: a selective restore from a split snapshot read
// 16 bytes of payload for every file it walked past (the header check), so it
// downloaded most of the data between selections. Only selected files' payload
// may be read, by either extractor.
func TestSplitSelectiveExtractReadsOnlySelectedPayload(t *testing.T) {
	meta, payload := makeSplitPXAR(t, splitOpts{padding: []byte("pad")}, selectiveEntries()...)
	owned := payloadRangesOf(t, meta)
	for _, parallel := range []bool{false, true} {
		rec := &recordingReaderAt{ra: bytes.NewReader(payload)}
		r, err := NewSplitPXARReaderAt(bytes.NewReader(meta), int64(len(meta)), rec, int64(len(payload)))
		if err != nil {
			t.Fatal(err)
		}
		rec.take() // the stream-start check made by the constructor
		dest := t.TempDir()
		rewriter := func(p string) string { return filepath.Join(dest, filepath.FromSlash(p)) }
		if parallel {
			_, err = r.ExtractWithRewriterParallel(rewriter, []string{"a", "c"}, true, 4)
		} else {
			_, err = r.ExtractWithRewriter(rewriter, []string{"a", "c"}, true)
		}
		if err != nil {
			t.Fatalf("parallel=%v: extract: %v", parallel, err)
		}
		label := fmt.Sprintf("parallel=%v", parallel)
		assertReadsInside(t, label, rec.take(), owned, "a", "c")
		for _, e := range selectiveEntries() {
			got, err := os.ReadFile(filepath.Join(dest, filepath.FromSlash(e.name)))
			wanted := strings.HasPrefix(e.name, "a/") || strings.HasPrefix(e.name, "c/")
			if wanted && (err != nil || !bytes.Equal(got, e.data)) {
				t.Errorf("%s: %s = %q, %v", label, e.name, got, err)
			}
			if !wanted && !os.IsNotExist(err) {
				t.Errorf("%s: %s restored but not selected", label, e.name)
			}
		}
	}
}

// SelectionRanges on a split archive walks only the metadata and returns the
// selected files' payload, merged, plus their content size.
func TestSplitSelectionRanges(t *testing.T) {
	meta, payload := makeSplitPXAR(t, splitOpts{}, selectiveEntries()...)
	owned := payloadRangesOf(t, meta)
	rec := &recordingReaderAt{ra: bytes.NewReader(payload)}
	r, err := NewSplitPXARReaderAt(bytes.NewReader(meta), int64(len(meta)), rec, int64(len(payload)))
	if err != nil {
		t.Fatal(err)
	}
	rec.take()
	ranges, selected, err := r.SelectionRanges([]string{"c", "a"})
	if err != nil {
		t.Fatal(err)
	}
	if reads := rec.take(); len(reads) != 0 {
		t.Errorf("SelectionRanges read the payload stream %d times", len(reads))
	}
	if selected != 100+50+70 {
		t.Errorf("selected bytes = %d, want 220", selected)
	}
	// a/one and a/two are adjacent in the payload (no padding), so they merge.
	want := [][2]int64{
		{owned["a/one.txt"][0], owned["a/two.txt"][1]},
		owned["c/three.txt"],
	}
	if fmt.Sprint(ranges) != fmt.Sprint(want) {
		t.Errorf("ranges = %v, want %v", ranges, want)
	}
	if _, _, err := r.SelectionRanges(nil); err == nil {
		t.Error("an empty selection was narrowed")
	}
}

// With SetSelectedProgress the progress total is the selected files' content
// size, exactly what SelectionRanges reports, for both extractors.
func TestSplitSelectedProgressMatchesSelection(t *testing.T) {
	meta, payload := makeSplitPXAR(t, splitOpts{padding: []byte("pp")}, selectiveEntries()...)
	for _, parallel := range []bool{false, true} {
		r := splitReader(t, meta, payload)
		_, selected, err := r.SelectionRanges([]string{"a", "c"})
		if err != nil {
			t.Fatal(err)
		}
		r.SetSelectedProgress(true)
		var last atomic.Int64
		r.SetProgressCallback(func(done int64) {
			for {
				cur := last.Load()
				if done <= cur || last.CompareAndSwap(cur, done) {
					return
				}
			}
		})
		dest := t.TempDir()
		rewriter := func(p string) string { return filepath.Join(dest, filepath.FromSlash(p)) }
		if parallel {
			_, err = r.ExtractWithRewriterParallel(rewriter, []string{"a", "c"}, true, 4)
		} else {
			_, err = r.ExtractWithRewriter(rewriter, []string{"a", "c"}, true)
		}
		if err != nil {
			t.Fatal(err)
		}
		if got := last.Load(); got != selected {
			t.Errorf("parallel=%v: progress ended at %d, want %d", parallel, got, selected)
		}
	}
}

// A bad reference in an unselected file no longer matters to a selective
// restore, and one in a selected file still fails that file, never writing
// wrong bytes. Listing keeps failing on it.
func TestSplitBadReferenceCheckedOnRead(t *testing.T) {
	meta, payload := makeSplitPXAR(t, splitOpts{}, selectiveEntries()...)
	marker := make([]byte, 8)
	binary.LittleEndian.PutUint64(marker, PXAR_PAYLOAD_REF)
	refPos := bytes.LastIndex(meta, marker) // d/four.txt, the last file
	if refPos < 0 {
		t.Fatal("no PAYLOAD_REF in fixture")
	}
	bad := append([]byte(nil), meta...)
	size := binary.LittleEndian.Uint64(bad[refPos+24 : refPos+32])
	binary.LittleEndian.PutUint64(bad[refPos+24:refPos+32], size+1) // header no longer matches

	if _, err := splitReader(t, bad, payload).ListEntries(); err == nil {
		t.Error("listing accepted a reference with a mismatched header")
	}

	dest := t.TempDir()
	rewriter := func(p string) string { return filepath.Join(dest, filepath.FromSlash(p)) }
	if _, err := splitReader(t, bad, payload).ExtractWithRewriter(rewriter, []string{"a"}, true); err != nil {
		t.Fatalf("restore of a/ failed over an unselected bad reference: %v", err)
	}

	for _, parallel := range []bool{false, true} {
		r := splitReader(t, bad, payload)
		var res []PXARExtractedFile
		var err error
		if parallel {
			res, err = r.ExtractWithRewriterParallel(rewriter, []string{"d"}, true, 2)
		} else {
			res, err = r.ExtractWithRewriter(rewriter, []string{"d"}, true)
		}
		if err != nil {
			t.Fatal(err)
		}
		failed := false
		for _, f := range res {
			if strings.HasSuffix(filepath.ToSlash(f.Path), "d/four.txt") {
				failed = f.Skipped && !f.Expected && strings.Contains(f.SkipReason, "matching payload header")
			}
		}
		if !failed {
			t.Errorf("parallel=%v: selected file with a bad reference was not failed: %+v", parallel, res)
		}
		if _, err := os.Stat(filepath.Join(dest, "d", "four.txt")); !os.IsNotExist(err) {
			t.Errorf("parallel=%v: d/four.txt was written despite a bad reference", parallel)
		}
	}
}

// selectiveWriteTree is a real tree for the writer, whose archives carry the
// GOODBYE BST tables the multi-include fast path resolves through.
func selectiveWriteTree(t *testing.T) (string, map[string][]byte) {
	t.Helper()
	root := t.TempDir()
	files := map[string][]byte{}
	for _, dir := range []string{"alpha", "beta", "gamma", "delta"} {
		for i := 0; i < 3; i++ {
			data := make([]byte, 40*1024+i*777)
			rand.Read(data)
			files[fmt.Sprintf("%s/f%d.bin", dir, i)] = data
		}
	}
	files["gamma/inner/deep.txt"] = []byte("deep\n")
	files["top.txt"] = []byte("top\n")
	for p, data := range files {
		full := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	return root, files
}

// On a real split archive, several includes resolve through the BST, only
// their subtrees are walked, and only their payload is read.
func TestSplitMultiIncludeFastPath(t *testing.T) {
	root, files := selectiveWriteTree(t)
	meta, payload, _ := writeArchive(t, root, true)
	owned := payloadRangesOf(t, meta)

	r := splitReader(t, meta, payload)
	if _, ok := r.resolveIncludeSpans([]string{"alpha", "gamma", "gamma/inner"}); !ok {
		t.Fatal("includes did not resolve through the GOODBYE BST")
	}

	for _, parallel := range []bool{false, true} {
		rec := &recordingReaderAt{ra: bytes.NewReader(payload)}
		r, err := NewSplitPXARReaderAt(bytes.NewReader(meta), int64(len(meta)), rec, int64(len(payload)))
		if err != nil {
			t.Fatal(err)
		}
		rec.take()
		dest := t.TempDir()
		rewriter := func(p string) string { return filepath.Join(dest, filepath.FromSlash(p)) }
		if parallel {
			_, err = r.ExtractWithRewriterParallel(rewriter, []string{"gamma", "alpha"}, true, 4)
		} else {
			_, err = r.ExtractWithRewriter(rewriter, []string{"gamma", "alpha"}, true)
		}
		if err != nil {
			t.Fatalf("parallel=%v: %v", parallel, err)
		}
		assertReadsInside(t, fmt.Sprintf("parallel=%v", parallel), rec.take(), owned, "alpha", "gamma")
		for p, data := range files {
			got, err := os.ReadFile(filepath.Join(dest, filepath.FromSlash(p)))
			wanted := strings.HasPrefix(p, "alpha/") || strings.HasPrefix(p, "gamma/")
			if wanted && (err != nil || !bytes.Equal(got, data)) {
				t.Errorf("parallel=%v: %s not restored correctly (%v)", parallel, p, err)
			}
			if !wanted && !os.IsNotExist(err) {
				t.Errorf("parallel=%v: %s restored but not selected", parallel, p)
			}
		}
	}
}

// A classic archive narrows several includes to their subtree spans, and the
// multi-include fast path restores exactly those subtrees.
func TestClassicMultiIncludeSelection(t *testing.T) {
	root, files := selectiveWriteTree(t)
	archive, _, _ := writeArchive(t, root, false)
	r := NewPXARReader(archive)

	ranges, selected, err := r.SelectionRanges([]string{"delta", "alpha"})
	if err != nil {
		t.Fatalf("SelectionRanges: %v", err)
	}
	if len(ranges) != 2 {
		t.Fatalf("ranges = %v, want 2 spans (alpha and delta are not adjacent)", ranges)
	}
	var sum int64
	for _, rg := range ranges {
		sum += rg[1] - rg[0]
	}
	if sum != selected || selected <= 3*40*1024*2 {
		t.Errorf("selected = %d, span sum = %d", selected, sum)
	}
	if _, _, err := r.SelectionRanges([]string{"beta", "no-such-dir"}); err == nil {
		t.Error("an unresolvable classic selection was narrowed")
	}

	dest := t.TempDir()
	if _, err := r.ExtractFiltered(dest, []string{"delta", "alpha"}, true); err != nil {
		t.Fatal(err)
	}
	for p, data := range files {
		got, err := os.ReadFile(filepath.Join(dest, filepath.FromSlash(p)))
		wanted := strings.HasPrefix(p, "alpha/") || strings.HasPrefix(p, "delta/")
		if wanted && (err != nil || !bytes.Equal(got, data)) {
			t.Errorf("%s not restored correctly (%v)", p, err)
		}
		if !wanted && !os.IsNotExist(err) {
			t.Errorf("%s restored but not selected", p)
		}
	}
}

// Read-ahead stays inside the allowed ranges: reading the first chunk of a
// range prefetches the rest of that range and nothing between ranges.
func TestDIDXReaderAt_PrefetchStaysInRanges(t *testing.T) {
	var chunks [][]byte
	for i := 0; i < 40; i++ {
		chunks = append(chunks, []byte(fmt.Sprintf("chunk-%02d-data", i)))
	}
	srv, hitCounts := newFakeChunkServer(t, chunks, 0)
	defer srv.Close()
	pbs := &PBSClient{BaseURL: srv.URL, Client: *srv.Client()}
	idx := buildFakeIndex(chunks)
	r := newTestReaderAt(pbs, idx)

	start := func(ci int) int64 {
		if ci == 0 {
			return 0
		}
		return int64(idx.offsets[ci-1])
	}
	end := func(ci int) int64 { return int64(idx.offsets[ci]) }
	// chunks 0-2 and 20-22; the second range is given first and the
	// first one overlaps itself, to exercise sorting and merging.
	count := r.LimitPrefetchToRanges([][2]int64{
		{start(20), end(22)},
		{start(0), end(1)},
		{start(1) + 1, end(2)},
	})
	if count != 6 {
		t.Fatalf("LimitPrefetchToRanges counted %d chunks, want 6", count)
	}
	hits := func(ci int) int32 { return hitCounts[idx.digests[ci]].Load() }
	waitFor := func(ci int) {
		deadline := time.Now().Add(2 * time.Second)
		for hits(ci) == 0 && time.Now().Before(deadline) {
			time.Sleep(5 * time.Millisecond)
		}
		if hits(ci) == 0 {
			t.Errorf("chunk %d was not prefetched", ci)
		}
	}

	buf := make([]byte, 4)
	if _, err := r.ReadAt(buf, 0); err != nil {
		t.Fatal(err)
	}
	waitFor(1)
	waitFor(2)
	if _, err := r.ReadAt(buf, start(20)); err != nil {
		t.Fatal(err)
	}
	waitFor(21)
	waitFor(22)
	time.Sleep(100 * time.Millisecond)
	for ci := 0; ci < len(chunks); ci++ {
		inRange := ci <= 2 || (ci >= 20 && ci <= 22)
		if !inRange && hits(ci) != 0 {
			t.Errorf("chunk %d outside the ranges was fetched %d times", ci, hits(ci))
		}
	}

	// No ranges: read-ahead off entirely.
	r2 := newTestReaderAt(pbs, idx)
	if n := r2.LimitPrefetchToRanges(nil); n != 0 {
		t.Errorf("no ranges counted %d chunks", n)
	}
	before := hits(31)
	if _, err := r2.ReadAt(buf, start(30)); err != nil {
		t.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)
	if hits(31) != before {
		t.Error("read-ahead ran with no allowed ranges")
	}
}
