package pbscommon

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// newFakeChunkServer serves /chunk?digest=<digest> from the given plaintext
// chunks, in PBS's uncompressed-blob wire format (8-byte magic + 4-byte
// placeholder + payload), and counts how many times each digest was
// requested — used to prove the inflight map coalesces concurrent requests
// for the same chunk instead of fetching it twice.
func newFakeChunkServer(t *testing.T, chunks [][]byte, perRequestDelay time.Duration) (*httptest.Server, map[string]*atomic.Int32) {
	t.Helper()
	byDigest := make(map[string][]byte, len(chunks))
	hitCounts := make(map[string]*atomic.Int32, len(chunks))
	for _, c := range chunks {
		sum := sha256.Sum256(c)
		digest := hex.EncodeToString(sum[:])
		byDigest[digest] = c
		hitCounts[digest] = &atomic.Int32{}
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		digest := r.URL.Query().Get("digest")
		payload, ok := byDigest[digest]
		if !ok {
			http.NotFound(w, r)
			return
		}
		if cnt, ok := hitCounts[digest]; ok {
			cnt.Add(1)
		}
		if perRequestDelay > 0 {
			time.Sleep(perRequestDelay)
		}
		w.Write(blobUncompressedMagic)
		w.Write([]byte{0, 0, 0, 0}) // 4-byte placeholder, unused by GetChunkData
		w.Write(payload)
	}))
	return srv, hitCounts
}

// buildFakeIndex constructs a didxIndex for the given chunks, matching the
// invariants downloadDIDXIndex itself enforces (strictly ascending cumulative
// end-offsets).
func buildFakeIndex(chunks [][]byte) *didxIndex {
	idx := &didxIndex{
		offsets: make([]uint64, len(chunks)),
		digests: make([]string, len(chunks)),
	}
	var cum uint64
	for i, c := range chunks {
		sum := sha256.Sum256(c)
		idx.digests[i] = hex.EncodeToString(sum[:])
		cum += uint64(len(c))
		idx.offsets[i] = cum
	}
	idx.total = cum
	return idx
}

func newTestReaderAt(pbs *PBSClient, idx *didxIndex) *DIDXReaderAt {
	return &DIDXReaderAt{
		pbs:         pbs,
		idx:         idx,
		cache:       newChunkCache(64),
		ctx:         nil,
		inflight:    make(map[int]chan struct{}),
		prefetchSem: make(chan struct{}, chunkPrefetchConcurrency),
	}
}

// TestDIDXReaderAt_SequentialReadMatchesSource proves the core correctness
// property the prefetch rework must not break: reading the whole stream
// sequentially (as PXAR extraction does) reproduces the exact original bytes,
// with prefetching turned on.
func TestDIDXReaderAt_SequentialReadMatchesSource(t *testing.T) {
	var chunks [][]byte
	var want []byte
	for i := 0; i < 20; i++ {
		c := []byte(fmt.Sprintf("chunk-%02d-payload-data", i))
		chunks = append(chunks, c)
		want = append(want, c...)
	}

	srv, hitCounts := newFakeChunkServer(t, chunks, 0)
	defer srv.Close()

	pbs := &PBSClient{BaseURL: srv.URL, Client: *srv.Client()}
	idx := buildFakeIndex(chunks)
	r := newTestReaderAt(pbs, idx)

	got := make([]byte, idx.total)
	n, err := r.ReadAt(got, 0)
	if err != nil {
		t.Fatalf("ReadAt: %v", err)
	}
	if n != len(got) {
		t.Fatalf("ReadAt returned %d bytes, want %d", n, len(got))
	}
	if string(got) != string(want) {
		t.Fatalf("content mismatch:\n got=%q\nwant=%q", got, want)
	}

	// Prefetch can race ahead of the sequential reader, but every chunk must
	// still be fetched EXACTLY once — the inflight map's job.
	for digest, cnt := range hitCounts {
		if got := cnt.Load(); got != 1 {
			t.Errorf("digest %s fetched %d times, want exactly 1", digest, got)
		}
	}
}

// TestDIDXReaderAt_ConcurrentReadersCoalesce proves the inflight map actually
// does its job under real concurrency: many goroutines hammering ReadAt for
// overlapping ranges (plus prefetch workers doing the same chunks in the
// background) must never cause the same chunk to be fetched twice.
func TestDIDXReaderAt_ConcurrentReadersCoalesce(t *testing.T) {
	var chunks [][]byte
	for i := 0; i < 10; i++ {
		chunks = append(chunks, []byte(fmt.Sprintf("payload-%d", i)))
	}
	srv, hitCounts := newFakeChunkServer(t, chunks, 5*time.Millisecond)
	defer srv.Close()

	pbs := &PBSClient{BaseURL: srv.URL, Client: *srv.Client()}
	idx := buildFakeIndex(chunks)
	r := newTestReaderAt(pbs, idx)

	done := make(chan error, 8)
	for g := 0; g < 8; g++ {
		go func() {
			buf := make([]byte, idx.total)
			_, err := r.ReadAt(buf, 0)
			done <- err
		}()
	}
	for i := 0; i < 8; i++ {
		if err := <-done; err != nil {
			t.Fatalf("concurrent ReadAt: %v", err)
		}
	}

	for digest, cnt := range hitCounts {
		if got := cnt.Load(); got != 1 {
			t.Errorf("digest %s fetched %d times under concurrent readers, want exactly 1", digest, got)
		}
	}
}

// TestDIDXReaderAt_PrefetchOverlapsLatency proves prefetch actually buys
// something: with an artificial per-chunk network delay, walking the stream
// sequentially (the real PXAR extraction pattern, one ReadAt call per chunk
// boundary) should take meaningfully less than chunkCount*delay, since later
// chunks were already fetched in the background while earlier ones were
// being "consumed" by the loop.
func TestDIDXReaderAt_PrefetchOverlapsLatency(t *testing.T) {
	const chunkCount = 12
	const delay = 40 * time.Millisecond
	var chunks [][]byte
	for i := 0; i < chunkCount; i++ {
		chunks = append(chunks, []byte(fmt.Sprintf("chunk-%d", i)))
	}
	srv, _ := newFakeChunkServer(t, chunks, delay)
	defer srv.Close()

	pbs := &PBSClient{BaseURL: srv.URL, Client: *srv.Client()}
	idx := buildFakeIndex(chunks)
	r := newTestReaderAt(pbs, idx)

	start := time.Now()
	buf := make([]byte, 1)
	for i := 0; i < chunkCount; i++ {
		off, _ := idx.chunkRange(i)
		if _, err := r.ReadAt(buf, int64(off)); err != nil {
			t.Fatalf("ReadAt chunk %d: %v", i, err)
		}
		time.Sleep(2 * time.Millisecond) // stand-in for local extraction/write work
	}
	elapsed := time.Since(start)

	fullySerial := time.Duration(chunkCount) * delay
	if elapsed >= fullySerial {
		t.Fatalf("prefetch bought nothing: sequential walk took %v, no better than the fully-serial bound %v", elapsed, fullySerial)
	}
	t.Logf("sequential walk with prefetch: %v (fully-serial bound: %v)", elapsed, fullySerial)
}
