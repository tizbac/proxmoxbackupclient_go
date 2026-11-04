package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"pbscommon"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alphadose/haxmap"
)

// poolChunks returns a ChunkState whose uploads go to upload, and the
// counters it adds to. No PBS is needed: a chunk is only sent through upload,
// and the index is not closed in these tests.
func poolChunks(upload func(wrid uint64, digest string, data []byte) error) (*ChunkState, *atomic.Uint64, *atomic.Uint64) {
	newchunk, reusechunk := new(atomic.Uint64), new(atomic.Uint64)
	c := &ChunkState{}
	c.reset(newchunk, reusechunk, haxmap.New[string, bool]())
	c.startUploads(func(b []byte) [32]byte { return sha256.Sum256(b) }, upload)
	return c, newchunk, reusechunk
}

// emit hands one chunk with this content to c, as a folder's walk would.
func emit(t *testing.T, c *ChunkState, content string) error {
	t.Helper()
	client := &pbscommon.PBSClient{}
	if err := c.HandleData([]byte(content), client); err != nil {
		return err
	}
	return c.ForceBoundary(client)
}

func digestOf(content string) string {
	d := sha256.Sum256([]byte(content))
	return hex.EncodeToString(d[:])
}

// New chunks are sent several at a time, each once, while the index keeps
// the order of the stream; a chunk seen again is reused, not sent again.
func TestUploadPoolSendsInParallel(t *testing.T) {
	var mu sync.Mutex
	sent := map[string]int{}
	var running, most atomic.Int32
	c, newchunk, reusechunk := poolChunks(func(_ uint64, digest string, _ []byte) error {
		n := running.Add(1)
		for {
			m := most.Load()
			if n <= m || most.CompareAndSwap(m, n) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond) // a round trip to PBS
		running.Add(-1)
		mu.Lock()
		sent[digest]++
		mu.Unlock()
		return nil
	})
	var want []string
	for i := 0; i < 40; i++ {
		content := fmt.Sprintf("chunk %d", i)
		if err := emit(t, c, content); err != nil {
			t.Fatal(err)
		}
		want = append(want, digestOf(content))
	}
	if err := emit(t, c, "chunk 7"); err != nil { // already sent
		t.Fatal(err)
	}
	want = append(want, digestOf("chunk 7"))
	c.stopUploads(false)

	if err := c.uploadErr(); err != nil {
		t.Fatal(err)
	}
	if digests, _, _ := c.indexList(); strings.Join(digests, ",") != strings.Join(want, ",") {
		t.Errorf("index order differs from the stream's")
	}
	if len(sent) != 40 {
		t.Errorf("%d chunks sent, want 40", len(sent))
	}
	for d, n := range sent {
		if n != 1 {
			t.Errorf("chunk %s sent %d times", d[:12], n)
		}
	}
	if newchunk.Load() != 40 || reusechunk.Load() != 1 {
		t.Errorf("new %d, reused %d, want 40 and 1", newchunk.Load(), reusechunk.Load())
	}
	if m := most.Load(); m < 2 || m > chunkUploadWorkers {
		t.Errorf("at most %d sent at a time, want 2 to %d", m, chunkUploadWorkers)
	}
}

// A chunk that cannot be sent stops the walk with its error, sends nothing
// further, and keeps the index from being closed: Eof returns the error
// before closing it (closing would need a PBS, which the test has not got).
func TestUploadPoolFailureStopsTheBackup(t *testing.T) {
	denied := errors.New("HTTP 400 - chunk refused")
	bad := digestOf("chunk 3")
	var c *ChunkState
	var after atomic.Int32
	c, _, _ = poolChunks(func(_ uint64, digest string, _ []byte) error {
		if digest == bad {
			return denied
		}
		if c.uploadErr() != nil { // a send started after the failure was kept
			after.Add(1)
		}
		return nil
	})
	var walkErr error
	for i := 0; i < 200 && walkErr == nil; i++ {
		walkErr = emit(t, c, fmt.Sprintf("chunk %d", i))
		if i >= 3 {
			time.Sleep(time.Millisecond) // let the worker record it
		}
	}
	if walkErr == nil {
		walkErr = c.Eof(&pbscommon.PBSClient{})
	}
	if !errors.Is(walkErr, denied) || !strings.Contains(walkErr.Error(), bad) {
		t.Fatalf("got %v, want the refused chunk's error", walkErr)
	}
	if err := c.Eof(&pbscommon.PBSClient{}); !errors.Is(err, denied) {
		t.Errorf("Eof after the failure: %v, want the failure, not a closed index", err)
	}
	// Only a worker that had taken its chunk before the failure was kept may
	// still send it.
	if n := after.Load(); n > chunkUploadWorkers {
		t.Errorf("%d chunks sent after the failure was kept", n)
	}
}

// A backup given up part way (an error in the walk) stops its pool: chunks
// still queued are not sent, and stopUploads returns once the ones under
// way are done, so none is still sending when the session is closed.
func TestUploadPoolAbandon(t *testing.T) {
	release := make(chan struct{})
	var started, sent atomic.Int32
	c, _, _ := poolChunks(func(uint64, string, []byte) error {
		started.Add(1)
		<-release
		sent.Add(1)
		return nil
	})
	queued := chunkUploadWorkers * 3 // all workers busy, the queue full
	for i := 0; i < queued; i++ {
		if err := emit(t, c, fmt.Sprintf("chunk %d", i)); err != nil {
			t.Fatal(err)
		}
	}
	// Every worker under way before the backup is given up.
	for deadline := time.Now().Add(5 * time.Second); started.Load() < chunkUploadWorkers; {
		if time.Now().After(deadline) {
			t.Fatalf("%d of %d workers started", started.Load(), chunkUploadWorkers)
		}
		time.Sleep(time.Millisecond)
	}
	done := make(chan struct{})
	go func() { c.stopUploads(true); close(done) }()
	select {
	case <-done:
		t.Fatal("stopUploads returned while chunks were still being sent")
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("stopUploads did not return")
	}
	if n := sent.Load(); n != chunkUploadWorkers {
		t.Errorf("%d chunks sent, want only the %d under way", n, chunkUploadWorkers)
	}
}
