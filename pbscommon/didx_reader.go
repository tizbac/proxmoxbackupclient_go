package pbscommon

import (
	"container/list"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// chunkFetchTimeout bounds a SINGLE chunk fetch (see GetChunkData's doc
// comment for why this exists — a stuck HTTP/2 stream on an otherwise-healthy
// connection can hang forever with no other protection). Chunks are at most
// ~16MB (the chunker's own max size), so even a slow link should comfortably
// finish well inside this.
const chunkFetchTimeout = 60 * time.Second

// chunkPrefetchWindow/chunkPrefetchConcurrency bound the background read-ahead
// added 2026-09-23 after a restore measured at ~6MB/s: chunkAt used to fetch
// exactly one chunk at a time, fully sequential (request, wait for the full
// round-trip, only then request the next), so throughput was bounded by
// per-chunk latency rather than link bandwidth. Since PXAR extraction walks
// the stream in mostly-linear order, whichever chunk chunkAt just resolved
// triggers background fetches of the next few indices into the same LRU
// cache, so by the time the sequential reader actually asks for them they are
// often already there. HTTP/2 (already in use for the PBS connection)
// multiplexes concurrent streams over the one connection, so this needs no
// extra connections, just extra goroutines issuing concurrent requests.
const (
	chunkPrefetchWindow      = 32 // how many chunks ahead of the one just resolved to prefetch
	chunkPrefetchConcurrency = 16 // max background prefetch fetches in flight at once
)

// ErrReadCancelled is returned by a DIDXReaderAt read once its cancel predicate
// reports true, so a long lazy walk (e.g. a cross-snapshot search) can be
// aborted between chunk fetches instead of running to completion.
var ErrReadCancelled = errors.New("read cancelled")

// DIDXReaderAt is an io.ReaderAt over a PBS dynamic-index archive that fetches
// referenced chunks from the server ON DEMAND, with a small LRU cache, instead
// of downloading and reassembling the whole archive into a temp file first.
//
// This is what makes selective restore cheap: PXARReader.walk skips the payload
// of files the caller did not select (it never ReadAt's those byte ranges), so
// the chunks that hold only unselected payload are never fetched. It also removes
// the "free %TEMP% space == archive size" requirement of the temp-file path,
// which is the prerequisite for backing up large drives without splitting.
//
// A cache is mandatory, not an optimization: walking reads many tiny headers
// (16 bytes) that fall inside the same multi-MB chunk, so without caching the
// last-used chunks every header read would re-download and re-decompress a whole
// chunk.
type DIDXReaderAt struct {
	pbs      *PBSClient
	idx      *didxIndex
	cache    *chunkCache
	progress func(fetched, total int)
	cancel   func() bool // optional; when it returns true, reads abort with ErrReadCancelled

	// ctx is the parent for each chunk fetch's own chunkFetchTimeout deadline
	// (see that const's doc comment). Cancelling ctx (e.g. a user-triggered
	// restore cancel) aborts an in-flight fetch immediately, same as the
	// timeout firing on its own if nothing cancels it first. Defaults to
	// context.Background() when the caller has none to offer (e.g. plain
	// browsing calls that don't go through a cancellable restore).
	ctx context.Context

	mu      sync.Mutex // serializes fetch bookkeeping (fetched counter)
	fetched int

	// inflight coalesces concurrent requests for the SAME chunk index (from
	// ReadAt's own caller racing a background prefetch worker, or two
	// prefetch triggers overlapping) into a single network fetch instead of
	// issuing duplicate requests. Keyed by chunk index; the channel is closed
	// once that chunk's fetch attempt finishes (success or failure).
	inflightMu sync.Mutex
	inflight   map[int]chan struct{}

	// prefetchSem bounds how many background prefetch fetches run at once,
	// independent of however many ReadAt callers exist.
	prefetchSem chan struct{}

	// fetchNanos/hashNanos accumulate wall-clock time spent in GetChunkData
	// (network round-trip + zstd decompression) and in the SHA-256 digest
	// check, across every chunk this reader has fetched — including
	// background prefetch workers, all summing into the same counters.
	// Diagnostic only (see Stats' doc comment): added 2026-09-23 to find out
	// where restore time actually goes once network latency was no longer
	// the dominant cost after prefetching landed.
	fetchNanos atomic.Int64
	hashNanos  atomic.Int64
}

// DIDXReaderAtStats is a snapshot of where this reader's cumulative fetch
// time went. Nanos are SUMMED across however many chunks/goroutines did the
// work, not wall-clock — with prefetch running several fetches concurrently,
// this can exceed the restore's real elapsed time. That's fine for its
// purpose: comparing FetchNanos against HashNanos (and, in the caller, against
// the extraction/write side's own totals) shows which phase dominates the
// real work, without needing per-chunk log spam.
type DIDXReaderAtStats struct {
	FetchNanos    int64 // cumulative time inside GetChunkData (network + decompress)
	HashNanos     int64 // cumulative time verifying each chunk's SHA-256
	ChunksFetched int   // count of chunks actually fetched from the server (cache hits don't count)
}

// Stats returns a snapshot of this reader's cumulative fetch/hash time so far.
func (r *DIDXReaderAt) Stats() DIDXReaderAtStats {
	r.mu.Lock()
	fetched := r.fetched
	r.mu.Unlock()
	return DIDXReaderAtStats{
		FetchNanos:    r.fetchNanos.Load(),
		HashNanos:     r.hashNanos.Load(),
		ChunksFetched: fetched,
	}
}

// SetCancelCheck installs a predicate polled before each read and chunk fetch;
// once it returns true the reader aborts with ErrReadCancelled. Pass nil to
// disable. Set it immediately after construction and before the first read —
// it is read without locking and is not safe to change concurrently with reads.
func (r *DIDXReaderAt) SetCancelCheck(fn func() bool) { r.cancel = fn }

// SetContext installs the parent context each chunk fetch's own
// chunkFetchTimeout deadline derives from (see the ctx field's doc comment).
// Pass nil to reset to context.Background(). Set immediately after
// construction and before the first read, same rule as SetCancelCheck.
func (r *DIDXReaderAt) SetContext(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	r.ctx = ctx
}

// NewDIDXReaderAt downloads and parses the .didx index for archiveName and
// returns a lazy reader over the reconstructed stream plus its total size. The
// PBSClient must stay Connected (reader session) for the lifetime of the reader,
// since chunks are fetched as the caller reads. cacheChunks defaults to 32.
// progress (optional) is called with (chunksFetchedSoFar, totalChunks) each time
// a NEW chunk is fetched from the server (cache hits do not advance it).
func (pbs *PBSClient) NewDIDXReaderAt(archiveName string, cacheChunks int, progress func(fetched, total int)) (*DIDXReaderAt, int64, error) {
	if cacheChunks <= 0 {
		cacheChunks = 32
	}
	idx, err := pbs.downloadDIDXIndex(archiveName)
	if err != nil {
		return nil, 0, err
	}
	return &DIDXReaderAt{
		pbs:         pbs,
		idx:         idx,
		cache:       newChunkCache(cacheChunks),
		progress:    progress,
		ctx:         context.Background(),
		inflight:    make(map[int]chan struct{}),
		prefetchSem: make(chan struct{}, chunkPrefetchConcurrency),
	}, int64(idx.total), nil
}

// chunkIndexAt returns the index of the chunk whose [start,end) span contains pos.
func (r *DIDXReaderAt) chunkIndexAt(pos uint64) int {
	// offsets[i] is the cumulative END offset of chunk i (ascending), so the chunk
	// containing pos is the first one whose end offset is strictly greater than pos.
	return sort.Search(len(r.idx.offsets), func(i int) bool {
		return r.idx.offsets[i] > pos
	})
}

// chunkAt returns the decompressed bytes of chunk ci, from cache or by fetching
// it from the server (verifying size and SHA-256 against the index digest).
// On a cache miss it also kicks off background prefetch of the next several
// chunks, so a mostly-linear walk (the common case) turns most subsequent
// fetches into cache hits instead of a fresh network round-trip each time.
func (r *DIDXReaderAt) chunkAt(ci int) ([]byte, error) {
	if r.cancel != nil && r.cancel() {
		return nil, ErrReadCancelled
	}
	data, err := r.ensureChunk(ci)
	if err != nil {
		return nil, err
	}
	r.triggerPrefetch(ci)
	return data, nil
}

// ensureChunk returns chunk ci's bytes, from cache if present, otherwise by
// fetching it — coalescing concurrent requests for the SAME index (ReadAt's
// caller racing a background prefetch worker, or two overlapping prefetch
// triggers) into a single network fetch via the inflight map.
func (r *DIDXReaderAt) ensureChunk(ci int) ([]byte, error) {
	if data, ok := r.cache.get(ci); ok {
		return data, nil
	}

	r.inflightMu.Lock()
	if ch, ok := r.inflight[ci]; ok {
		r.inflightMu.Unlock()
		<-ch
		if data, ok := r.cache.get(ci); ok {
			return data, nil
		}
		// The in-flight attempt failed and isn't propagated to a second
		// waiter by design (keeps this simple) — retry once, synchronously.
		return r.fetchOnce(ci)
	}
	ch := make(chan struct{})
	r.inflight[ci] = ch
	r.inflightMu.Unlock()

	data, err := r.fetchOnce(ci)

	r.inflightMu.Lock()
	delete(r.inflight, ci)
	r.inflightMu.Unlock()
	close(ch)

	return data, err
}

// fetchOnce does the actual network fetch, validation, cache population and
// progress bump for chunk ci. Callers arrange for it to run at most once per
// index at a time (see ensureChunk) — this does no coalescing of its own.
func (r *DIDXReaderAt) fetchOnce(ci int) ([]byte, error) {
	digest := r.idx.digests[ci]
	ctx := r.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	fetchStart := time.Now()
	fetchCtx, cancelFetch := context.WithTimeout(ctx, chunkFetchTimeout)
	chunk, err := r.pbs.GetChunkData(fetchCtx, digest)
	cancelFetch()
	r.fetchNanos.Add(int64(time.Since(fetchStart)))
	if err != nil {
		return nil, fmt.Errorf("fetch chunk %s (index %d/%d): %w", digest, ci, len(r.idx.digests), err)
	}
	start, end := r.idx.chunkRange(ci)
	if uint64(len(chunk)) != end-start {
		return nil, fmt.Errorf("chunk %s (index %d): decompressed size %d != expected %d", digest, ci, len(chunk), end-start)
	}
	// PBS dynamic-index digests are the SHA-256 of the chunk plaintext; a mismatch
	// means a corrupted or tampered chunk — fail rather than serve wrong data.
	hashStart := time.Now()
	sum := sha256.Sum256(chunk)
	r.hashNanos.Add(int64(time.Since(hashStart)))
	if hex.EncodeToString(sum[:]) != digest {
		return nil, fmt.Errorf("chunk %s (index %d): content hash mismatch", digest, ci)
	}
	r.cache.put(ci, chunk)

	r.mu.Lock()
	r.fetched++
	fetched := r.fetched
	r.mu.Unlock()
	if r.progress != nil {
		r.progress(fetched, len(r.idx.digests))
	}
	return chunk, nil
}

// triggerPrefetch schedules background fetches for the chunkPrefetchWindow
// indices following ci, skipping any already cached or already in flight,
// bounded by prefetchSem so at most chunkPrefetchConcurrency run at once. A
// full semaphore just means this call schedules fewer than the full window —
// the next chunkAt along the walk tries again from its own position.
func (r *DIDXReaderAt) triggerPrefetch(ci int) {
	total := len(r.idx.digests)
	for offset := 1; offset <= chunkPrefetchWindow; offset++ {
		next := ci + offset
		if next >= total {
			break
		}
		if _, ok := r.cache.get(next); ok {
			continue
		}
		r.inflightMu.Lock()
		_, already := r.inflight[next]
		r.inflightMu.Unlock()
		if already {
			continue
		}
		select {
		case r.prefetchSem <- struct{}{}:
		default:
			return // worker pool full — try again from a later chunkAt call
		}
		go func(idx int) {
			defer func() { <-r.prefetchSem }()
			if r.ctx != nil {
				select {
				case <-r.ctx.Done():
					return
				default:
				}
			}
			if r.cancel != nil && r.cancel() {
				return
			}
			_, _ = r.ensureChunk(idx) // best-effort; the real consumer surfaces any error when it gets here
		}(next)
	}
}

// ReadAt implements io.ReaderAt over the reconstructed stream, fetching only the
// chunks that overlap [off, off+len(p)). It satisfies the io.ReaderAt contract:
// it fills p fully unless it reaches the end of the stream, in which case it
// returns the bytes read and io.EOF.
func (r *DIDXReaderAt) ReadAt(p []byte, off int64) (int, error) {
	if r.cancel != nil && r.cancel() {
		return 0, ErrReadCancelled
	}
	if off < 0 {
		return 0, fmt.Errorf("didx readerat: negative offset %d", off)
	}
	total := int64(r.idx.total)
	if off >= total {
		return 0, io.EOF
	}
	n := 0
	for n < len(p) {
		pos := uint64(off) + uint64(n)
		if pos >= r.idx.total {
			break
		}
		ci := r.chunkIndexAt(pos)
		chunk, err := r.chunkAt(ci)
		if err != nil {
			return n, err
		}
		start, _ := r.idx.chunkRange(ci)
		n += copy(p[n:], chunk[pos-start:])
	}
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}

// chunkCache is a small LRU cache of decompressed chunks keyed by chunk index.
type chunkCache struct {
	mu    sync.Mutex
	cap   int
	ll    *list.List
	items map[int]*list.Element
}

type chunkCacheEntry struct {
	idx  int
	data []byte
}

func newChunkCache(capacity int) *chunkCache {
	return &chunkCache{
		cap:   capacity,
		ll:    list.New(),
		items: make(map[int]*list.Element, capacity),
	}
}

func (c *chunkCache) get(idx int) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[idx]; ok {
		c.ll.MoveToFront(el)
		return el.Value.(*chunkCacheEntry).data, true
	}
	return nil, false
}

func (c *chunkCache) put(idx int, data []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[idx]; ok {
		c.ll.MoveToFront(el)
		el.Value.(*chunkCacheEntry).data = data
		return
	}
	el := c.ll.PushFront(&chunkCacheEntry{idx: idx, data: data})
	c.items[idx] = el
	for c.ll.Len() > c.cap {
		oldest := c.ll.Back()
		if oldest == nil {
			break
		}
		c.ll.Remove(oldest)
		delete(c.items, oldest.Value.(*chunkCacheEntry).idx)
	}
}
