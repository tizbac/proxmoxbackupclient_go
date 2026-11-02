package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"pbscommon"
	"sync"
	"testing"
)

var fidxMagic = [8]byte{47, 127, 65, 237, 145, 253, 15, 205}

// fakeChunks is an in-memory stand-in for *pbscommon.PBSClient. It records how
// many times each digest was fetched so the cache tests can prove a chunk was
// served from memory the second time.
type fakeChunks struct {
	mu       sync.Mutex
	data     map[string][]byte
	calls    map[string]int
	errs     map[string]error
	truncate map[string]int
}

func newFakeChunks() *fakeChunks {
	return &fakeChunks{
		data:     map[string][]byte{},
		calls:    map[string]int{},
		errs:     map[string]error{},
		truncate: map[string]int{},
	}
}

func (f *fakeChunks) GetChunkData(_ context.Context, digest string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls[digest]++
	if err, ok := f.errs[digest]; ok {
		return nil, err
	}
	d, ok := f.data[digest]
	if !ok {
		return nil, fmt.Errorf("fakeChunks: unknown digest %s", digest)
	}
	if n, ok := f.truncate[digest]; ok && n < len(d) {
		return append([]byte(nil), d[:n]...), nil
	}
	return append([]byte(nil), d...), nil
}

func (f *fakeChunks) fetchCount(digest string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[digest]
}

func (f *fakeChunks) totalFetches() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.calls {
		n += c
	}
	return n
}

// buildFIDX serialises a FIDX header (magic + size + chunk size) followed by one
// 32-byte digest per chunk.
func buildFIDX(t *testing.T, size, chunkSize uint64, digests [][32]byte) []byte {
	t.Helper()
	var hdr pbscommon.FIDXHeader
	hdr.Magic = fidxMagic
	hdr.CreationTime = 1600000000
	hdr.Size = size
	hdr.ChunkSize = chunkSize
	for i := range digests {
		copy(hdr.IndexCsum[:], digests[i][:])
	}
	buf := &bytes.Buffer{}
	if err := binary.Write(buf, binary.LittleEndian, &hdr); err != nil {
		t.Fatalf("marshalling FIDX header: %v", err)
	}
	for _, d := range digests {
		buf.Write(d[:])
	}
	return buf.Bytes()
}

// splitImage chops payload into fixed-size chunks, returning the digest table a
// FIDX would carry plus a digest -> payload map for the fake client.
func splitImage(payload []byte, chunkSize int) ([][32]byte, map[string][]byte) {
	var digests [][32]byte
	store := map[string][]byte{}
	for off, i := 0, 0; off < len(payload); off, i = off+chunkSize, i+1 {
		var d [32]byte
		for j := 0; j < chunkSize && j < len(d) && off+j < len(payload); j++ {
			d[j] = byte(off+j) + 1
		}
		end := off + chunkSize
		if end > len(payload) {
			end = len(payload)
		}
		digests = append(digests, d)
		store[hex.EncodeToString(d[:])] = append([]byte(nil), payload[off:end]...)
	}
	return digests, store
}

// newTestFIDX wires a FIDXServer over a fake client holding payload.
func newTestFIDX(t *testing.T, payload []byte, chunkSize int) (*FIDXServer, *fakeChunks, []string) {
	t.Helper()
	digests, store := splitImage(payload, chunkSize)
	fake := newFakeChunks()
	for k, v := range store {
		fake.data[k] = v
	}
	srv, err := NewFIDXServer(buildFIDX(t, uint64(len(payload)), uint64(chunkSize), digests), fake)
	if err != nil {
		t.Fatalf("NewFIDXServer: %v", err)
	}
	names := make([]string, 0, len(digests))
	for _, d := range digests {
		names = append(names, hex.EncodeToString(d[:]))
	}
	return srv, fake, names
}

func TestFIDXParseChunkList(t *testing.T) {
	payload := []byte("ABCDEFGHIJ")
	srv, _, digests := newTestFIDX(t, payload, 4)

	if len(srv.chunks) != 3 {
		t.Fatalf("chunks = %d, want 3 (%v)", len(srv.chunks), srv.chunks)
	}
	for i, want := range digests {
		if srv.chunks[i] != want {
			t.Errorf("chunks[%d] = %s, want %s", i, srv.chunks[i], want)
		}
	}
	if srv.cached == nil {
		t.Fatal("cached map not initialised")
	}
	if got, err := srv.Size(); err != nil || got != int64(len(payload)) {
		t.Errorf("Size() = %d, %v; want %d, nil", got, err, len(payload))
	}
}

func TestFIDXChunkCountIsCeiling(t *testing.T) {
	cases := []struct {
		size, chunkSize uint64
		want            int
	}{
		{0, 4, 0},
		{1, 4, 1},
		{3, 4, 1},
		{4, 4, 1},
		{5, 4, 2},
		{8, 4, 2},
		{9, 4, 3},
		{10, 4, 3},
		{4096, 4096, 1},
		{4097, 4096, 2},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%d_%d", tc.size, tc.chunkSize), func(t *testing.T) {
			digests, store := splitImage(make([]byte, tc.size), int(tc.chunkSize))
			if len(digests) != tc.want {
				t.Fatalf("fixture built %d digests, want %d", len(digests), tc.want)
			}
			fake := newFakeChunks()
			for k, v := range store {
				fake.data[k] = v
			}
			srv, err := NewFIDXServer(buildFIDX(t, tc.size, tc.chunkSize, digests), fake)
			if err != nil {
				t.Fatalf("NewFIDXServer: %v", err)
			}
			if len(srv.chunks) != tc.want {
				t.Errorf("chunks = %d, want %d", len(srv.chunks), tc.want)
			}
		})
	}
}

func TestFIDXRejectsBadMagic(t *testing.T) {
	data := buildFIDX(t, 4, 4, [][32]byte{{}})
	data[0] ^= 0xff
	_, err := NewFIDXServer(data, newFakeChunks())
	if err == nil {
		t.Fatal("expected an error for bad magic")
	}
}

func TestFIDXRejectsTruncatedHeader(t *testing.T) {
	data := buildFIDX(t, 4, 4, [][32]byte{{}})
	_, err := NewFIDXServer(data[:100], newFakeChunks())
	if err == nil {
		t.Fatal("expected an error for a header cut short")
	}
}

func TestFIDXRejectsMissingDigest(t *testing.T) {
	data := buildFIDX(t, 10, 4, make([][32]byte, 3))
	// Drop the final digest but keep Size/ChunkSize promising three chunks.
	_, err := NewFIDXServer(data[:len(data)-32], newFakeChunks())
	if err == nil {
		t.Fatal("expected an error when a chunk digest is missing")
	}
}

func TestFIDXRejectsZeroChunkSize(t *testing.T) {
	// Used to divide by zero on the chunk-count loop and panic.
	data := buildFIDX(t, 10, 4, make([][32]byte, 3))
	var hdr pbscommon.FIDXHeader
	if err := binary.Read(bytes.NewReader(data), binary.LittleEndian, &hdr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	hdr.ChunkSize = 0
	buf := &bytes.Buffer{}
	if err := binary.Write(buf, binary.LittleEndian, &hdr); err != nil {
		t.Fatalf("marshal: %v", err)
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("NewFIDXServer panicked on ChunkSize=0: %v", r)
		}
	}()
	if _, err := NewFIDXServer(buf.Bytes(), newFakeChunks()); err == nil {
		t.Fatal("expected an error for ChunkSize=0")
	}
}

func TestFIDXEmptyImage(t *testing.T) {
	srv, fake, _ := newTestFIDX(t, nil, 4)
	if got, err := srv.Size(); err != nil || got != 0 {
		t.Fatalf("Size() = %d, %v; want 0, nil", got, err)
	}
	if _, err := srv.ReadAt(make([]byte, 512), 0); err != io.EOF {
		t.Fatalf("ReadAt on an empty image = %v, want io.EOF", err)
	}
	if fake.totalFetches() != 0 {
		t.Errorf("fetched %d chunks for an empty image, want 0", fake.totalFetches())
	}
}

func TestFIDXGetChunksIndexes(t *testing.T) {
	srv, _, _ := newTestFIDX(t, make([]byte, 12), 4)

	cases := []struct {
		name      string
		off, size int64
		want      []ChunkIndex
	}{
		{"first chunk", 0, 4, []ChunkIndex{{0, 0, 4}}},
		{"inside one chunk", 1, 2, []ChunkIndex{{0, 1, 3}}},
		{"tail of a chunk", 2, 4, []ChunkIndex{{0, 2, 4}, {1, 0, 2}}},
		{"aligned to a boundary", 4, 4, []ChunkIndex{{1, 0, 4}}},
		{"across three chunks", 3, 9, []ChunkIndex{{0, 3, 4}, {1, 0, 4}, {2, 0, 4}}},
		{"whole image", 0, 12, []ChunkIndex{{0, 0, 4}, {1, 0, 4}, {2, 0, 4}}},
		{"zero length at a boundary", 4, 0, nil},
		{"last partial chunk", 10, 2, []ChunkIndex{{2, 2, 4}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := srv.getChunksIndexes(tc.off, tc.size)
			if len(got) != len(tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("index %d: got %+v, want %+v", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestFIDXReadAtEveryAlignment(t *testing.T) {
	payload := []byte("the quick brown fox jumps over th") // 34 bytes
	const chunkSize = 4
	srv, fake, _ := newTestFIDX(t, payload, chunkSize)

	// Every (offset, length) pair the kernel NBD client could legally issue.
	for off := int64(0); off < int64(len(payload)); off++ {
		for length := 0; off+int64(length) <= int64(len(payload)); length++ {
			buf := make([]byte, length)
			n, err := srv.ReadAt(buf, off)
			if err != nil {
				t.Fatalf("ReadAt(%d, %d): %v", off, length, err)
			}
			if n != length {
				t.Fatalf("ReadAt(%d, %d) n = %d", off, length, n)
			}
			if want := payload[off : off+int64(length)]; !bytes.Equal(buf, want) {
				t.Errorf("ReadAt(%d, %d) = %q, want %q", off, length, buf, want)
			}
		}
	}
	if got, want := fake.totalFetches(), len(payload)/chunkSize+1; got != want {
		t.Errorf("fetched %d chunks, want %d", got, want)
	}
}

func TestFIDXReadAtSpansChunks(t *testing.T) {
	payload := []byte("ABCDEFGHIJ")
	srv, _, digests := newTestFIDX(t, payload, 4)

	buf := make([]byte, 4)
	if _, err := srv.ReadAt(buf, 3); err != nil {
		t.Fatalf("ReadAt: %v", err)
	}
	if want := "DEFG"; string(buf) != want {
		t.Errorf("ReadAt(3, 4) = %q, want %q", buf, want)
	}
	for i := 0; i < 2; i++ {
		if fake := srv.cached[int64(i)]; fake == nil {
			t.Errorf("chunk %d not cached", i)
		}
	}
	if srv.cached[2] != nil {
		t.Error("chunk 2 fetched but not needed")
	}
	if len(digests) != 3 {
		t.Fatalf("fixture has %d digests, want 3", len(digests))
	}
}

func TestFIDXReadAtAtEndReturnsEOF(t *testing.T) {
	payload := []byte("ABCDEFGHIJ")
	srv, _, _ := newTestFIDX(t, payload, 4)

	for _, off := range []int64{10, 11, 4096} {
		n, err := srv.ReadAt(make([]byte, 4), off)
		if n != 0 || err != io.EOF {
			t.Errorf("ReadAt(_, %d) = %d, %v; want 0, io.EOF", off, n, err)
		}
	}
}

func TestFIDXReadAtRunningOffTheEndIsShort(t *testing.T) {
	payload := []byte("ABCDEFGHIJ") // 10 bytes over three 4-byte chunks
	srv, _, _ := newTestFIDX(t, payload, 4)

	buf := make([]byte, 4)
	n, err := srv.ReadAt(buf, 8)
	if err != io.EOF {
		t.Fatalf("ReadAt(_, 8) err = %v, want io.EOF", err)
	}
	if n != 2 {
		t.Errorf("ReadAt(_, 8) n = %d, want 2", n)
	}
	if want := "IJ"; string(buf[:n]) != want {
		t.Errorf("ReadAt(_, 8) = %q, want %q", buf[:n], want)
	}
}

func TestFIDXReadAtRejectsNegativeOffset(t *testing.T) {
	srv, _, _ := newTestFIDX(t, []byte("ABCDEFGHIJ"), 4)
	if _, err := srv.ReadAt(make([]byte, 4), -1); err == nil {
		t.Fatal("expected an error for a negative offset")
	}
}

func TestFIDXReadAtEmptyBufferFetchesNothing(t *testing.T) {
	srv, fake, _ := newTestFIDX(t, []byte("ABCDEFGHIJ"), 4)
	n, err := srv.ReadAt(nil, 0)
	if n != 0 || err != nil {
		t.Fatalf("ReadAt(nil, 0) = %d, %v; want 0, nil", n, err)
	}
	if fake.totalFetches() != 0 {
		t.Errorf("fetched %d chunks for a zero-length read, want 0", fake.totalFetches())
	}
}

func TestFIDXReadAtPropagatesFetchError(t *testing.T) {
	payload := []byte("ABCDEFGHIJ")
	srv, fake, digests := newTestFIDX(t, payload, 4)
	boom := errors.New("connection reset by peer")
	fake.errs[digests[1]] = boom

	// Used to panic(err) and take the whole process down mid-restore. Offset 2
	// spans chunk 0 and chunk 1, so the failing fetch is on the path.
	buf := make([]byte, 4)
	n, err := srv.ReadAt(buf, 2)
	if !errors.Is(err, boom) {
		t.Fatalf("ReadAt err = %v, want %v", err, boom)
	}
	if n != 0 {
		t.Errorf("ReadAt n = %d, want 0", n)
	}
}

func TestFIDXReadAtRejectsTruncatedChunk(t *testing.T) {
	payload := []byte("ABCDEFGHIJ")
	srv, fake, digests := newTestFIDX(t, payload, 4)
	fake.truncate[digests[0]] = 2 // server hands back less than a chunk

	// Used to slice ch.Data[0:4] on a 2-byte slice and panic.
	buf := make([]byte, 4)
	if _, err := srv.ReadAt(buf, 0); err == nil {
		t.Fatal("expected an error for a chunk shorter than ChunkSize")
	}
}

func TestFIDXReadAtRejectsChunkIndexOutOfRange(t *testing.T) {
	fake := newFakeChunks()
	fake.data["aa"] = []byte("ABCD")
	srv := &FIDXServer{
		// Size/ChunkSize promise 25 chunks but only one digest is present.
		header: pbscommon.FIDXHeader{Size: 100, ChunkSize: 4},
		chunks: []string{"aa"},
		cached: map[int64]*CachedChunk{},
		client: fake,
	}
	if _, err := srv.ReadAt(make([]byte, 4), 8); err == nil {
		t.Fatal("expected an error for a chunk index past the digest table")
	}
	if fake.totalFetches() != 0 {
		t.Errorf("fetched %d chunks before validating the index, want 0", fake.totalFetches())
	}
}

func TestFIDXReadAtServesSecondReadFromCache(t *testing.T) {
	payload := []byte("ABCDEFGHIJ")
	srv, fake, digests := newTestFIDX(t, payload, 4)

	for pass := 0; pass < 3; pass++ {
		buf := make([]byte, len(payload))
		if _, err := srv.ReadAt(buf, 0); err != nil {
			t.Fatalf("pass %d: %v", pass, err)
		}
		if !bytes.Equal(buf, payload) {
			t.Fatalf("pass %d: got %q, want %q", pass, buf, payload)
		}
	}
	for i, d := range digests {
		if got := fake.fetchCount(d); got != 1 {
			t.Errorf("chunk %d fetched %d times, want 1", i, got)
		}
	}
	if got := len(srv.cached); got != 3 {
		t.Errorf("cache holds %d chunks, want 3", got)
	}
	for i, c := range srv.cached {
		if c.Life != LRU_CACHE_LIFE {
			t.Errorf("cached chunk %d has Life %d, want %d", i, c.Life, LRU_CACHE_LIFE)
		}
		if c.Index != i {
			t.Errorf("cached chunk %d records Index %d", i, c.Index)
		}
	}
}

func TestFIDXReadAtEvictsExpiredChunks(t *testing.T) {
	fake := newFakeChunks()
	fake.data["aa"] = []byte("ABCD")
	fake.data["bb"] = []byte("ZZZZ")
	srv := &FIDXServer{
		header: pbscommon.FIDXHeader{Size: 8, ChunkSize: 4},
		chunks: []string{"aa", "bb"},
		// Chunk 1 is one miss away from expiring; chunk 0 is absent so the
		// read below takes the miss path and ages every cached entry.
		cached: map[int64]*CachedChunk{1: {Data: []byte("ZZZZ"), Index: 1, Life: 1}},
		client: fake,
	}

	buf := make([]byte, 4)
	if _, err := srv.ReadAt(buf, 0); err != nil {
		t.Fatalf("ReadAt: %v", err)
	}
	if string(buf) != "ABCD" {
		t.Errorf("ReadAt(0, 4) = %q, want %q", buf, "ABCD")
	}
	if _, ok := srv.cached[1]; ok {
		t.Error("chunk 1 should have been evicted once its life ran out")
	}
	c, ok := srv.cached[0]
	if !ok {
		t.Fatal("fetched chunk 0 was not cached")
	}
	if c.Life != LRU_CACHE_LIFE {
		t.Errorf("cached chunk 0 has Life %d, want %d", c.Life, LRU_CACHE_LIFE)
	}
}

func TestFIDXReadAtConcurrent(t *testing.T) {
	payload := make([]byte, 512)
	for i := range payload {
		payload[i] = byte(i*7) + 3
	}
	const chunkSize = 16
	srv, fake, _ := newTestFIDX(t, payload, chunkSize)

	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(seed int64) {
			defer wg.Done()
			r := rand.New(rand.NewSource(seed))
			for i := 0; i < 200; i++ {
				off := int64(r.Intn(len(payload)))
				end := off + int64(r.Intn(len(payload)-int(off))+1)
				buf := make([]byte, end-off)
				if _, err := srv.ReadAt(buf, off); err != nil {
					t.Errorf("ReadAt(%d, %d): %v", off, len(buf), err)
					return
				}
				if !bytes.Equal(buf, payload[off:end]) {
					t.Errorf("ReadAt(%d, %d) mismatch", off, len(buf))
					return
				}
			}
		}(int64(g) + 1)
	}
	wg.Wait()

	// Every fetch must be for a digest that is actually in the image, and the
	// cache can never hold more entries than the image has chunks. Total fetch
	// count is deliberately not pinned: every miss ages all other entries, so
	// under contention the same chunk can legitimately be evicted and refetched.
	wantChunks := len(payload) / chunkSize
	if got := len(srv.cached); got > wantChunks {
		t.Errorf("cache holds %d chunks, more than the %d in the image", got, wantChunks)
	}
	for _, c := range srv.cached {
		if c.Index < 0 || c.Index >= int64(wantChunks) {
			t.Errorf("cache holds out-of-range chunk index %d", c.Index)
		}
	}
	if got := fake.totalFetches(); got < wantChunks {
		t.Errorf("fetched %d chunks, fewer than the %d distinct chunks", got, wantChunks)
	}
}

func TestFIDXIsReadOnly(t *testing.T) {
	srv, _, _ := newTestFIDX(t, []byte("ABCDEFGHIJ"), 4)
	if n, err := srv.WriteAt([]byte("XXXX"), 0); n != 0 || err == nil {
		t.Errorf("WriteAt = %d, %v; want 0, an error", n, err)
	}
	if err := srv.Sync(); err == nil {
		t.Error("Sync = nil, want an error")
	}
	if string([]byte("ABCDEFGHIJ")) != "ABCDEFGHIJ" {
		t.Fatal("unreachable")
	}
}

func TestFIDXSatisfiesNBDBackend(t *testing.T) {
	srv, _, _ := newTestFIDX(t, []byte("ABCDEFGHIJ"), 4)
	var _ interface {
		io.ReaderAt
		io.WriterAt
		Size() (int64, error)
		Sync() error
	} = srv
}
