package pbscommon

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// ParsePreviousDIDXChunkDigests
// ---------------------------------------------------------------------------

func TestParsePreviousDIDXChunkDigests(t *testing.T) {
	a := makeDIDX(t, nil, payloadsOf(3, 64)...)

	got := ParsePreviousDIDXChunkDigests(a.didx)
	want := a.order
	if len(got) != len(want) {
		t.Fatalf("got %d digests, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("digest %d = %s, want %s", i, got[i], want[i])
		}
		if len(got[i]) != 64 {
			t.Errorf("digest %d is not 64 hex chars: %q", i, got[i])
		}
	}
}

func TestParsePreviousDIDXChunkDigestsHeaderOnly(t *testing.T) {
	// A valid but empty index must yield an empty, non-nil slice: the caller
	// distinguishes "no chunks" from "unusable index" via nil.
	a := makeDIDX(t, nil) // magic + 4096-byte header, zero chunk records
	got := ParsePreviousDIDXChunkDigests(a.didx)
	if got == nil {
		t.Fatal("header-only index should yield an empty non-nil slice")
	}
	if len(got) != 0 {
		t.Errorf("got %d digests, want 0", len(got))
	}
}

// Every rejection path returns nil rather than panicking. A nil result is safe:
// the caller just re-uploads everything instead of deduplicating.
func TestParsePreviousDIDXChunkDigestsRejectsBadInput(t *testing.T) {
	valid := makeDIDX(t, nil, payloadsOf(2, 32)...)

	badMagic := make([]byte, len(valid.didx))
	copy(badMagic, valid.didx)
	copy(badMagic[:8], []byte{0, 1, 2, 3, 4, 5, 6, 7})

	tornEntry := append([]byte(nil), valid.didx...)
	tornEntry = append(tornEntry, 0x01, 0x02, 0x03) // not a multiple of 40

	cases := []struct {
		name string
		body []byte
	}{
		{"nil", nil},
		{"empty", []byte{}},
		{"tiny 404 body", []byte("404 Not Found")},
		{"one byte short of header", valid.didx[:didxHeaderSize-1]},
		{"bad magic", badMagic},
		{"torn trailing entry", tornEntry},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ParsePreviousDIDXChunkDigests(tc.body); got != nil {
				t.Errorf("expected nil, got %v", got)
			}
		})
	}
}

// The old ad-hoc previousDidx[4096:] slicing panicked on these inputs; the
// parser must never panic on arbitrary bytes.
func TestParsePreviousDIDXChunkDigestsNeverPanics(t *testing.T) {
	seeds := [][]byte{
		make([]byte, 0),
		make([]byte, 100),
		make([]byte, 4096),
		make([]byte, 4095),
		make([]byte, 4136),
		make([]byte, 4135),
	}
	for i := range seeds {
		// scribble over the header so the magic check is exercised both ways
		for j := 0; j < 8 && j < len(seeds[i]); j++ {
			seeds[i][j] = didxMagic[j]
		}
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("panic on seed %d (len %d): %v", i, len(seeds[i]), r)
				}
			}()
			ParsePreviousDIDXChunkDigests(seeds[i])
		}()
	}
}

// ---------------------------------------------------------------------------
// downloadDIDXIndex
// ---------------------------------------------------------------------------

func TestDownloadDIDXIndex(t *testing.T) {
	a := makeDIDX(t, nil, payloadsOf(4, 100)...)
	f := newFakePBSWithDIDX(t, "backup.pxar.didx", a, nil)

	idx, err := f.downloadDIDXIndex("backup.pxar.didx")
	if err != nil {
		t.Fatalf("downloadDIDXIndex: %v", err)
	}
	if len(idx.digests) != 4 {
		t.Fatalf("got %d chunks, want 4", len(idx.digests))
	}
	for i, want := range a.order {
		if idx.digests[i] != want {
			t.Errorf("digest %d = %s, want %s", i, idx.digests[i], want)
		}
		if idx.offsets[i] != a.offsets[i] {
			t.Errorf("offset %d = %d, want %d", i, idx.offsets[i], a.offsets[i])
		}
	}
	if idx.total != a.offsets[len(a.offsets)-1] {
		t.Errorf("total = %d, want %d", idx.total, a.offsets[len(a.offsets)-1])
	}
}

func TestDownloadDIDXIndexEmptyArchive(t *testing.T) {
	a := makeDIDX(t, nil) // zero chunks
	f := newFakePBSWithDIDX(t, "empty.didx", a, nil)

	idx, err := f.downloadDIDXIndex("empty.didx")
	if err != nil {
		t.Fatalf("downloadDIDXIndex: %v", err)
	}
	if len(idx.digests) != 0 {
		t.Errorf("got %d chunks, want 0", len(idx.digests))
	}
	if idx.total != 0 {
		t.Errorf("total = %d, want 0", idx.total)
	}
}

func TestDownloadDIDXIndexRejectsMalformed(t *testing.T) {
	good := makeDIDX(t, nil, payloadsOf(3, 50)...)

	badMagic := make([]byte, len(good.didx))
	copy(badMagic, good.didx)
	copy(badMagic[:8], []byte{9, 9, 9, 9, 9, 9, 9, 9})

	short := good.didx[:didxHeaderSize-1]
	torn := append(append([]byte(nil), good.didx...), 0xff)

	// Non-monotonic offsets: entry 2's cumulative end goes backwards. Without the
	// guard, chunkIndexAt's sort.Search would return an index whose chunkRange
	// start exceeds the requested position, underflowing chunk[pos-start:] in ReadAt.
	nonMonotonic := make([]byte, len(good.didx))
	copy(nonMonotonic, good.didx)
	binary.LittleEndian.PutUint64(nonMonotonic[didxHeaderSize+2*didxEntrySize:], 1)

	// A zero-length first chunk makes offsets[0] == 0, which is also rejected.
	zeroFirst := make([]byte, len(good.didx))
	copy(zeroFirst, good.didx)
	binary.LittleEndian.PutUint64(zeroFirst[didxHeaderSize:], 0)

	cases := []struct {
		name     string
		body     []byte
		wantText string
	}{
		{"short read", short, "short read"},
		{"bad magic", badMagic, "invalid DIDX magic"},
		{"torn entry", torn, "not a multiple of"},
		{"non-monotonic offsets", nonMonotonic, "non-monotonic offset"},
		{"zero-length first chunk", zeroFirst, "non-monotonic offset"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFakePBSWithDIDX(t, "x.didx", good, nil)
			f.overrideDidx = tc.body
			_, err := f.downloadDIDXIndex("x.didx")
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), tc.wantText) {
				t.Errorf("error %q does not mention %q", err, tc.wantText)
			}
		})
	}
}

func TestDownloadDIDXIndexPropagatesDownloadError(t *testing.T) {
	a := makeDIDX(t, nil, payloadsOf(1, 16)...)
	f := newFakePBSWithDIDX(t, "present.didx", a, nil)

	if _, err := f.downloadDIDXIndex("missing.didx"); err == nil {
		t.Fatal("expected an error for an archive the server does not have")
	} else if !strings.Contains(err.Error(), "missing.didx") {
		t.Errorf("error should name the archive, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// chunkRange
// ---------------------------------------------------------------------------

func TestChunkRange(t *testing.T) {
	// cumulative end offsets: 10, 25, 25+7
	idx := &didxIndex{offsets: []uint64{10, 25, 32}}

	cases := []struct {
		i         int
		wantStart uint64
		wantEnd   uint64
	}{
		{0, 0, 10},
		{1, 10, 25},
		{2, 25, 32},
	}
	for _, tc := range cases {
		start, end := idx.chunkRange(tc.i)
		if start != tc.wantStart || end != tc.wantEnd {
			t.Errorf("chunkRange(%d) = (%d,%d), want (%d,%d)",
				tc.i, start, end, tc.wantStart, tc.wantEnd)
		}
		if end-start == 0 {
			t.Errorf("chunkRange(%d) is empty", tc.i)
		}
	}
}

// chunkRange must be consistent with the digests ParsePreviousDIDXChunkDigests
// reports, since dedup relies on both reading the same index layout.
func TestParsePreviousDigestsMatchDownloadedIndex(t *testing.T) {
	a := makeDIDX(t, nil, payloadsOf(5, 33)...)
	f := newFakePBSWithDIDX(t, "backup.pxar.didx", a, nil)

	idx, err := f.downloadDIDXIndex("backup.pxar.didx")
	if err != nil {
		t.Fatalf("downloadDIDXIndex: %v", err)
	}
	prev := ParsePreviousDIDXChunkDigests(a.didx)
	if len(prev) != len(idx.digests) {
		t.Fatalf("previous parser returned %d digests, index has %d", len(prev), len(idx.digests))
	}
	for i := range prev {
		if prev[i] != idx.digests[i] {
			t.Errorf("digest %d: previous=%s index=%s", i, prev[i], idx.digests[i])
		}
	}
}

// The magic and the record layout are an on-the-wire contract with the PBS
// server; pin them so an accidental change is caught here.
func TestDIDXWireConstants(t *testing.T) {
	if want := []byte{28, 145, 78, 165, 25, 186, 179, 205}; !bytes.Equal(didxMagic, want) {
		t.Errorf("didxMagic = %v, want %v", didxMagic, want)
	}
	if didxHeaderSize != 4096 {
		t.Errorf("didxHeaderSize = %d, want 4096", didxHeaderSize)
	}
	if didxEntrySize != 40 {
		t.Errorf("didxEntrySize = %d, want 40", didxEntrySize)
	}
}

// A digest is stored raw, not as ASCII hex, inside the 40-byte record.
func TestDIDXEntryLayout(t *testing.T) {
	payload := payloadsOf(1, 8)[0]
	a := makeDIDX(t, nil, payload)

	entry := a.didx[didxHeaderSize:]
	if len(entry) != didxEntrySize {
		t.Fatalf("entry section is %d bytes, want %d", len(entry), didxEntrySize)
	}
	if got := binary.LittleEndian.Uint64(entry[:8]); got != uint64(len(payload)) {
		t.Errorf("cumulative end offset = %d, want %d", got, len(payload))
	}
	want, err := hex.DecodeString(a.order[0])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(entry[8:40], want) {
		t.Errorf("stored digest bytes = %x, want %x", entry[8:40], want)
	}
}
