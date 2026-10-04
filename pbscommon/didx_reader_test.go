package pbscommon

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// DIDXReaderAt: io.ReaderAt semantics
// ---------------------------------------------------------------------------

func TestDIDXReaderAtReadsWholeStream(t *testing.T) {
	payloads := payloadsOf(4, 256)
	a := makeDIDX(t, nil, payloads...)
	want := bytes.Join(payloads, nil)

	f := newFakePBSWithDIDX(t, "backup.pxar.didx", a, nil)
	r, total, err := f.NewDIDXReaderAt("backup.pxar.didx", 8, nil)
	if err != nil {
		t.Fatalf("NewDIDXReaderAt: %v", err)
	}
	if total != int64(len(want)) {
		t.Errorf("total = %d, want %d", total, len(want))
	}

	got := make([]byte, len(want))
	n, err := r.ReadAt(got, 0)
	if err != nil && !errors.Is(err, io.EOF) {
		t.Fatalf("ReadAt: %v", err)
	}
	if n != len(want) {
		t.Fatalf("read %d bytes, want %d", n, len(want))
	}
	if !bytes.Equal(got, want) {
		t.Error("stream contents differ from the concatenated chunks")
	}
}

func TestDIDXReaderAtRandomOffsetsMatchSource(t *testing.T) {
	payloads := payloadsOf(5, 100)
	a := makeDIDX(t, nil, payloads...)
	want := bytes.Join(payloads, nil)

	f := newFakePBSWithDIDX(t, "a.didx", a, nil)
	r, total, err := f.NewDIDXReaderAt("a.didx", 8, nil)
	if err != nil {
		t.Fatalf("NewDIDXReaderAt: %v", err)
	}

	// Exercise reads that start inside a chunk, span several chunks, and are
	// shorter than a chunk.
	lengths := []int{1, 7, 99, 100, 101, 250, 499}
	offs := []int64{0, 1, 50, 99, 100, 101, 250, total - 1, total - 100, total - 499}
	for _, off := range offs {
		if off < 0 || off >= total {
			continue
		}
		for _, l := range lengths {
			buf := make([]byte, l)
			n, err := r.ReadAt(buf, off)
			if err != nil && !errors.Is(err, io.EOF) {
				t.Fatalf("ReadAt(off=%d, len=%d): %v", off, l, err)
			}
			wantN := l
			if int64(wantN) > total-off {
				wantN = int(total - off)
			}
			if n != wantN {
				t.Errorf("ReadAt(off=%d, len=%d) = %d bytes, want %d", off, l, n, wantN)
				continue
			}
			if !bytes.Equal(buf[:n], want[off:off+int64(n)]) {
				t.Errorf("ReadAt(off=%d, len=%d) returned wrong bytes", off, l)
			}
		}
	}
}

func TestDIDXReaderAtPartialReadAtEndReturnsEOF(t *testing.T) {
	payloads := payloadsOf(2, 64)
	a := makeDIDX(t, nil, payloads...)
	total := int64(len(payloads[0]) + len(payloads[1]))

	f := newFakePBSWithDIDX(t, "a.didx", a, nil)
	r, _, err := f.NewDIDXReaderAt("a.didx", 4, nil)
	if err != nil {
		t.Fatalf("NewDIDXReaderAt: %v", err)
	}

	// Ask for more than remains: the ReaderAt contract allows a short read plus
	// io.EOF.
	buf := make([]byte, 200)
	n, err := r.ReadAt(buf, total-10)
	if !errors.Is(err, io.EOF) {
		t.Fatalf("err = %v, want io.EOF", err)
	}
	if n != 10 {
		t.Errorf("read %d bytes, want 10", n)
	}
	want := bytes.Join(payloads, nil)
	if !bytes.Equal(buf[:n], want[total-10:]) {
		t.Error("the short read returned the wrong bytes")
	}
}

func TestDIDXReaderAtPastEndReturnsEOF(t *testing.T) {
	payloads := payloadsOf(2, 64)
	a := makeDIDX(t, nil, payloads...)
	total := int64(len(payloads[0]) + len(payloads[1]))

	f := newFakePBSWithDIDX(t, "a.didx", a, nil)
	r, _, err := f.NewDIDXReaderAt("a.didx", 4, nil)
	if err != nil {
		t.Fatalf("NewDIDXReaderAt: %v", err)
	}

	for _, off := range []int64{total, total + 1, total + 4096} {
		buf := make([]byte, 16)
		n, err := r.ReadAt(buf, off)
		if !errors.Is(err, io.EOF) {
			t.Errorf("off=%d: err = %v, want io.EOF", off, err)
		}
		if n != 0 {
			t.Errorf("off=%d: read %d bytes, want 0", off, n)
		}
	}
}

func TestDIDXReaderAtRejectsNegativeOffset(t *testing.T) {
	a := makeDIDX(t, nil, payloadsOf(2, 32)...)
	f := newFakePBSWithDIDX(t, "a.didx", a, nil)
	r, _, err := f.NewDIDXReaderAt("a.didx", 4, nil)
	if err != nil {
		t.Fatalf("NewDIDXReaderAt: %v", err)
	}
	if _, err := r.ReadAt(make([]byte, 8), -1); err == nil {
		t.Fatal("expected an error for a negative offset")
	}
}

func TestDIDXReaderAtEmptyArchive(t *testing.T) {
	a := makeDIDX(t, nil)
	f := newFakePBSWithDIDX(t, "empty.didx", a, nil)
	r, total, err := f.NewDIDXReaderAt("empty.didx", 4, nil)
	if err != nil {
		t.Fatalf("NewDIDXReaderAt: %v", err)
	}
	if total != 0 {
		t.Errorf("total = %d, want 0", total)
	}
	buf := make([]byte, 8)
	if n, err := r.ReadAt(buf, 0); !errors.Is(err, io.EOF) || n != 0 {
		t.Errorf("ReadAt on empty archive = (%d, %v), want (0, io.EOF)", n, err)
	}
}

// ---------------------------------------------------------------------------
// Caching: the cache is load-bearing, not an optimisation.
// ---------------------------------------------------------------------------

// Walking reads many tiny headers that land in the same multi-MB chunk. Without
// caching, every header read would re-download a whole chunk.
func TestDIDXReaderAtCachesChunks(t *testing.T) {
	payloads := payloadsOf(4, 512)
	a := makeDIDX(t, nil, payloads...)
	want := bytes.Join(payloads, nil)

	f := newFakePBSWithDIDX(t, "a.didx", a, nil)

	var progressCalls []int
	r, _, err := f.NewDIDXReaderAt("a.didx", 8, func(fetched, total int) {
		progressCalls = append(progressCalls, fetched)
	})
	if err != nil {
		t.Fatalf("NewDIDXReaderAt: %v", err)
	}

	// 40 tiny reads spread over the whole stream: every chunk is touched many
	// times but must only be fetched once.
	for i := 0; i < len(want); i += 8 {
		buf := make([]byte, 4)
		if _, err := r.ReadAt(buf, int64(i)); err != nil && !errors.Is(err, io.EOF) {
			t.Fatalf("ReadAt(%d): %v", i, err)
		}
	}

	if got := f.totalFetches(); got != 4 {
		t.Errorf("fetched %d chunks, want 4 (cache is not being used)", got)
	}
	if len(progressCalls) != 4 {
		t.Errorf("progress callback fired %d times, want 4 (only on new fetches)", len(progressCalls))
	}
	for i, got := range progressCalls {
		if got != i+1 {
			t.Errorf("progress call %d reported fetched=%d, want %d", i, got, i+1)
		}
	}
}

func TestDIDXReaderAtCacheEvictsLeastRecentlyUsed(t *testing.T) {
	payloads := payloadsOf(3, 64)
	a := makeDIDX(t, nil, payloads...)

	f := newFakePBSWithDIDX(t, "a.didx", a, nil)
	// Capacity 1: reading chunk 0, then chunk 1, then chunk 0 again must
	// re-fetch chunk 0.
	r, _, err := f.NewDIDXReaderAt("a.didx", 1, nil)
	if err != nil {
		t.Fatalf("NewDIDXReaderAt: %v", err)
	}

	read := func(off int64) {
		t.Helper()
		if _, err := r.ReadAt(make([]byte, 8), off); err != nil && !errors.Is(err, io.EOF) {
			t.Fatalf("ReadAt(%d): %v", off, err)
		}
	}
	read(0)  // chunk 0
	read(64) // chunk 1, evicts chunk 0
	read(0)  // chunk 0 again -> must be re-fetched
	if got := f.fetchCount(a.order[0]); got != 2 {
		t.Errorf("chunk 0 fetched %d times, want 2", got)
	}
	if got := f.fetchCount(a.order[1]); got != 1 {
		t.Errorf("chunk 1 fetched %d times, want 1", got)
	}
}

func TestChunkCacheLRUEviction(t *testing.T) {
	c := newChunkCache(2)

	c.put(1, []byte("one"))
	c.put(2, []byte("two"))
	if _, ok := c.get(1); !ok {
		t.Fatal("1 should be cached")
	}
	if _, ok := c.get(2); !ok {
		t.Fatal("2 should be cached")
	}

	// Touch 1 so that 2 becomes least-recently-used.
	if _, ok := c.get(1); !ok {
		t.Fatal("1 should still be cached")
	}
	c.put(3, []byte("three"))

	if _, ok := c.get(2); ok {
		t.Error("2 should have been evicted as least recently used")
	}
	if _, ok := c.get(1); !ok {
		t.Error("1 should have survived (it was used more recently than 2)")
	}
	if v, ok := c.get(3); !ok || string(v) != "three" {
		t.Error("3 should be cached with its payload intact")
	}
}

func TestChunkCacheOverwritesExistingKey(t *testing.T) {
	c := newChunkCache(2)
	c.put(7, []byte("old"))
	c.put(7, []byte("new"))
	v, ok := c.get(7)
	if !ok || string(v) != "new" {
		t.Errorf("get(7) = (%q, %v), want (\"new\", true)", v, ok)
	}
	if c.ll.Len() != 1 {
		t.Errorf("cache holds %d entries after an overwrite, want 1", c.ll.Len())
	}
}

// ---------------------------------------------------------------------------
// Integrity
// ---------------------------------------------------------------------------

// A chunk whose bytes do not hash to the digest recorded in the index must be
// rejected rather than served.
func TestDIDXReaderAtRejectsContentMismatch(t *testing.T) {
	payloads := payloadsOf(2, 64)
	a := makeDIDX(t, nil, payloads...)
	f := newFakePBSWithDIDX(t, "a.didx", a, nil)
	// Same length as the real chunk (so the size check passes) but different bytes.
	f.corrupt[a.order[0]] = bytes.Repeat([]byte{0xAB}, len(payloads[0]))

	r, _, err := f.NewDIDXReaderAt("a.didx", 4, nil)
	if err != nil {
		t.Fatalf("NewDIDXReaderAt: %v", err)
	}
	_, err = r.ReadAt(make([]byte, 8), 0)
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "content hash mismatch") {
		t.Errorf("error %q should mention a content hash mismatch", err)
	}
}

func TestDIDXReaderAtRejectsSizeMismatch(t *testing.T) {
	payloads := payloadsOf(2, 64)
	a := makeDIDX(t, nil, payloads...)
	f := newFakePBSWithDIDX(t, "a.didx", a, nil)
	// Different length: caught by the size check before the hash check.
	f.corrupt[a.order[0]] = bytes.Repeat([]byte{0xAB}, len(payloads[0])+1)

	r, _, err := f.NewDIDXReaderAt("a.didx", 4, nil)
	if err != nil {
		t.Fatalf("NewDIDXReaderAt: %v", err)
	}
	_, err = r.ReadAt(make([]byte, 8), 0)
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "decompressed size") {
		t.Errorf("error %q should mention the size mismatch", err)
	}
}

func TestDIDXReaderAtPropagatesChunkFetchError(t *testing.T) {
	payloads := payloadsOf(2, 64)
	a := makeDIDX(t, nil, payloads...)
	f := newFakePBSWithDIDX(t, "a.didx", a, nil)

	r, _, err := f.NewDIDXReaderAt("a.didx", 4, nil)
	if err != nil {
		t.Fatalf("NewDIDXReaderAt: %v", err)
	}
	// Drop the second chunk from the archive so the fetch 404s.
	delete(a.chunks, a.order[1])
	// Indices were already parsed from the didx, so the digest is still known.
	_, err = r.ReadAt(make([]byte, 8), int64(len(payloads[0])))
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), a.order[1]) {
		t.Errorf("error %q should name the offending digest", err)
	}
}

// ---------------------------------------------------------------------------
// Cancellation
// ---------------------------------------------------------------------------

func TestDIDXReaderAtCancelBeforeRead(t *testing.T) {
	payloads := payloadsOf(2, 64)
	a := makeDIDX(t, nil, payloads...)
	f := newFakePBSWithDIDX(t, "a.didx", a, nil)
	r, _, err := f.NewDIDXReaderAt("a.didx", 4, nil)
	if err != nil {
		t.Fatalf("NewDIDXReaderAt: %v", err)
	}

	r.SetCancelCheck(func() bool { return true })
	if _, err := r.ReadAt(make([]byte, 8), 0); !errors.Is(err, ErrReadCancelled) {
		t.Errorf("err = %v, want ErrReadCancelled", err)
	}
	if got := f.totalFetches(); got != 0 {
		t.Errorf("fetched %d chunks after cancellation, want 0", got)
	}

	// Clearing the predicate must make reads work again.
	r.SetCancelCheck(nil)
	if _, err := r.ReadAt(make([]byte, 8), 0); err != nil {
		t.Errorf("read after clearing the cancel predicate: %v", err)
	}
}

// A long multi-chunk read can be aborted between chunk fetches.
func TestDIDXReaderAtCancelBetweenChunks(t *testing.T) {
	payloads := payloadsOf(6, 64)
	a := makeDIDX(t, nil, payloads...)
	f := newFakePBSWithDIDX(t, "a.didx", a, nil)
	r, _, err := f.NewDIDXReaderAt("a.didx", 8, nil)
	if err != nil {
		t.Fatalf("NewDIDXReaderAt: %v", err)
	}

	calls := 0
	r.SetCancelCheck(func() bool {
		calls++
		// Let the first couple of chunk fetches through, then abort.
		return calls > 4
	})

	// One read spanning every chunk.
	buf := make([]byte, len(payloads)*len(payloads[0]))
	n, err := r.ReadAt(buf, 0)
	if !errors.Is(err, ErrReadCancelled) {
		t.Fatalf("err = %v, want ErrReadCancelled", err)
	}
	if n >= len(buf) {
		t.Errorf("read %d bytes, expected a partial read", n)
	}
}

func TestDIDXReaderAtCancelDoesNotBreakSubsequentReads(t *testing.T) {
	payloads := payloadsOf(3, 64)
	a := makeDIDX(t, nil, payloads...)
	want := bytes.Join(payloads, nil)
	f := newFakePBSWithDIDX(t, "a.didx", a, nil)
	r, _, err := f.NewDIDXReaderAt("a.didx", 8, nil)
	if err != nil {
		t.Fatalf("NewDIDXReaderAt: %v", err)
	}

	cancel := true
	r.SetCancelCheck(func() bool { return cancel })
	if _, err := r.ReadAt(make([]byte, 8), 0); !errors.Is(err, ErrReadCancelled) {
		t.Fatalf("err = %v, want ErrReadCancelled", err)
	}
	cancel = false
	got := make([]byte, len(want))
	if _, err := r.ReadAt(got, 0); err != nil && !errors.Is(err, io.EOF) {
		t.Fatalf("ReadAt: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Error("data read after a cancelled read is wrong")
	}
}

// ---------------------------------------------------------------------------
// Encrypted archives
// ---------------------------------------------------------------------------

// An encrypted snapshot stores id_key-salted digests and AES-256-GCM blobs; the
// reader must decrypt transparently and hash-check the plaintext.
func TestDIDXReaderAtEncryptedArchive(t *testing.T) {
	crypt := testCryptConfig(t)
	payloads := payloadsOf(4, 200)
	a := makeDIDX(t, crypt, payloads...)
	want := bytes.Join(payloads, nil)

	f := newFakePBSWithDIDX(t, "enc.didx", a, crypt)
	r, total, err := f.NewDIDXReaderAt("enc.didx", 8, nil)
	if err != nil {
		t.Fatalf("NewDIDXReaderAt: %v", err)
	}
	if total != int64(len(want)) {
		t.Errorf("total = %d, want %d", total, len(want))
	}

	got := make([]byte, len(want))
	if _, err := r.ReadAt(got, 0); err != nil && !errors.Is(err, io.EOF) {
		t.Fatalf("ReadAt: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Error("decrypted stream differs from the original chunks")
	}

	// The digests in an encrypted index are NOT plain SHA-256.
	if a.order[0] == chunkDigest(nil, payloads[0]) {
		t.Error("encrypted index should not use plain SHA-256 digests")
	}
}

// Without the key, an encrypted archive must fail loudly rather than serve
// ciphertext.
func TestDIDXReaderAtEncryptedArchiveWithoutKey(t *testing.T) {
	crypt := testCryptConfig(t)
	payloads := payloadsOf(2, 64)
	a := makeDIDX(t, crypt, payloads...)
	f := newFakePBSWithDIDX(t, "enc.didx", a, crypt)
	f.Crypt = nil // client without the key

	r, _, err := f.NewDIDXReaderAt("enc.didx", 4, nil)
	if err != nil {
		t.Fatalf("NewDIDXReaderAt: %v", err)
	}
	if _, err := r.ReadAt(make([]byte, 8), 0); err == nil {
		t.Fatal("expected an error when reading an encrypted archive without the key")
	}
}

// The wrong key must fail the integrity check, not silently return garbage.
func TestDIDXReaderAtEncryptedArchiveWrongKey(t *testing.T) {
	crypt := testCryptConfig(t)
	other, err := NewCryptConfig(bytes.Repeat([]byte{0xAB}, 32))
	if err != nil {
		t.Fatalf("NewCryptConfig: %v", err)
	}

	payloads := payloadsOf(2, 64)
	a := makeDIDX(t, crypt, payloads...)
	f := newFakePBSWithDIDX(t, "enc.didx", a, crypt)
	f.Crypt = other

	r, _, err := f.NewDIDXReaderAt("enc.didx", 4, nil)
	if err != nil {
		t.Fatalf("NewDIDXReaderAt: %v", err)
	}
	if _, err := r.ReadAt(make([]byte, 8), 0); err == nil {
		t.Fatal("expected an error when decrypting with the wrong key")
	}
}

// ---------------------------------------------------------------------------
// Cache size defaults
// ---------------------------------------------------------------------------

func TestNewDIDXReaderAtCacheSizeDefault(t *testing.T) {
	a := makeDIDX(t, nil, payloadsOf(2, 32)...)
	f := newFakePBSWithDIDX(t, "a.didx", a, nil)

	for _, requested := range []int{0, -1, -100} {
		r, _, err := f.NewDIDXReaderAt("a.didx", requested, nil)
		if err != nil {
			t.Fatalf("NewDIDXReaderAt(%d): %v", requested, err)
		}
		if r.cache.cap != 32 {
			t.Errorf("cacheChunks=%d gave capacity %d, want the default 32", requested, r.cache.cap)
		}
	}
}
