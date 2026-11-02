package pbscommon

import (
	"fmt"
	"testing"
	"time"
)

// Five readers (four extraction workers and the walker) moving through
// different parts of one archive, sharing one 64-chunk cache: with the
// default 32-chunk read-ahead they push each other's chunks out before they
// are read (refetches); with the window shared out (11 each) nothing is
// fetched twice. The prefetches are let finish after every read so the
// outcome does not depend on timing.
func TestDIDXReaderAt_PrefetchWindowSharedCache(t *testing.T) {
	var chunks [][]byte
	for i := 0; i < 400; i++ {
		chunks = append(chunks, []byte(fmt.Sprintf("chunk-%03d-%s", i, "xxxxxxxxxxxxxxxx")))
	}
	srv, _ := newFakeChunkServer(t, chunks, 0)
	defer srv.Close()
	pbs := &PBSClient{BaseURL: srv.URL, Client: readerTestClient(srv)}

	run := func(window int) DIDXReaderAtStats {
		idx := buildFakeIndex(chunks)
		r := newTestReaderAt(pbs, idx)
		r.SetPrefetchWindow(window)
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
		starts := []int{0, 80, 160, 240, 320}
		for step := 0; step < 60; step++ {
			for _, s := range starts {
				if _, err := r.chunkAt(s + step); err != nil {
					t.Fatal(err)
				}
				settle()
			}
		}
		return r.Stats()
	}

	wide := run(0) // default window
	if wide.ChunksRefetched == 0 {
		t.Fatalf("expected refetches with the default window: %+v", wide)
	}
	narrow := run(64/5 - 1)
	if narrow.ChunksRefetched != 0 {
		t.Errorf("window 11 x 5 readers should fit a 64-chunk cache: %+v", narrow)
	}
	t.Logf("default window: %d fetched, %d refetched; window 11: %d fetched, %d refetched",
		wide.ChunksFetched, wide.ChunksRefetched, narrow.ChunksFetched, narrow.ChunksRefetched)
}
