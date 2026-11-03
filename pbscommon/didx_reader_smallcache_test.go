package pbscommon

import (
	"fmt"
	"testing"
	"time"
)

// A reader whose cache is smaller than the read-ahead window reads ahead
// only as far as the cache holds beside the chunk just read, so a linear
// walk fetches every chunk exactly once. With the full 32-chunk window a
// 4-chunk cache evicted chunks it had just prefetched, and they were fetched
// again; TestDIDXReaderAtCacheEvictsLeastRecentlyUsed (a 1-chunk cache)
// failed about 1 run in 50 for that reason. The prefetches are let finish
// after every read, so the outcome does not depend on timing.
func TestDIDXReaderAt_SmallCacheNoRefetch(t *testing.T) {
	var chunks [][]byte
	for i := 0; i < 40; i++ {
		chunks = append(chunks, []byte(fmt.Sprintf("chunk-%03d-%s", i, "yyyyyyyyyyyyyyyy")))
	}
	srv, _ := newFakeChunkServer(t, chunks, 0)
	defer srv.Close()
	pbs := &PBSClient{BaseURL: srv.URL, Client: readerTestClient(srv)}

	for _, capacity := range []int{1, 2, 4} {
		r := newTestReaderAt(pbs, buildFakeIndex(chunks))
		r.cache = newChunkCache(capacity)
		settle := func() {
			for i := 0; i < 500; i++ {
				r.inflightMu.Lock()
				n := len(r.inflight)
				r.inflightMu.Unlock()
				if n == 0 && len(r.prefetchSem) == 0 {
					return
				}
				time.Sleep(time.Millisecond)
			}
		}
		for ci := range chunks {
			if _, err := r.chunkAt(ci); err != nil {
				t.Fatal(err)
			}
			settle()
		}
		st := r.Stats()
		if st.ChunksRefetched != 0 || st.ChunksFetched != len(chunks) {
			t.Errorf("cache %d: %d fetched, %d refetched; want %d fetched, none twice",
				capacity, st.ChunksFetched, st.ChunksRefetched, len(chunks))
		}
	}
}
