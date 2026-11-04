package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"pbscommon"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alphadose/haxmap"
)

// TestWalkSpeed measures what one archive stream costs before anything is
// sent, on the folder in PBS_WALK_DIR (read from the page cache after the
// first pass), in layers: reading and encoding the archive; plus finding chunk
// boundaries; plus SHA-256 of each chunk; and the backup's own ChunkState with
// the sends stubbed out. The differences show where the single walk thread's
// time goes. Run with -cpuprofile for the detail. Opt-in: PBS_WALK_DIR.
func TestWalkSpeed(t *testing.T) {
	dir := os.Getenv("PBS_WALK_DIR")
	if dir == "" {
		t.Skip("set PBS_WALK_DIR to a folder to measure")
	}
	walk := func(cb func([]byte) error) (int64, time.Duration) {
		var n int64
		a := &pbscommon.PXARArchive{ArchiveName: "backup.pxar.didx"}
		a.WriteCB = func(b []byte) error { n += int64(len(b)); return cb(b) }
		a.CatalogWriteCB = func([]byte) error { return nil }
		a.Create()
		start := time.Now()
		if _, err := a.WriteDir(dir, "", true); err != nil {
			t.Fatal(err)
		}
		return n, time.Since(start)
	}
	report := func(label string, n int64, d time.Duration) {
		t.Logf("%-34s %7.1f MB in %6.2f s = %7.1f MB/s", label, float64(n)/1e6, d.Seconds(), float64(n)/1e6/d.Seconds())
	}

	walk(func([]byte) error { return nil }) // warm the page cache
	n, d := walk(func([]byte) error { return nil })
	report("read + encode", n, d)

	var ch pbscommon.Chunker
	ch.New(4 << 20)
	breaks := 0
	n, d = walk(func(b []byte) error {
		for {
			p := ch.Scan(b)
			if p == 0 {
				return nil
			}
			breaks++
			b = b[p:]
		}
	})
	report(fmt.Sprintf("+ boundaries (%d chunks)", breaks), n, d)

	ch.New(4 << 20)
	h := sha256.New()
	n, d = walk(func(b []byte) error {
		for {
			p := ch.Scan(b)
			if p == 0 {
				h.Write(b)
				return nil
			}
			h.Write(b[:p])
			h.Sum(nil)
			h.Reset()
			b = b[p:]
		}
	})
	report("+ boundaries + SHA-256", n, d)

	// The same with SHA-256 moved off the walk thread: the walk finds the
	// boundaries and hands each chunk (a copy, as ChunkState makes one) to 4
	// hashing goroutines. Grown: the chunk is built by appending, as now;
	// sized: its buffer is made once at the average chunk size and reused for
	// the next chunk only after it is hashed (here: never, a fresh one).
	for _, sized := range []bool{false, true} {
		ch.New(4 << 20)
		jobs := make(chan []byte, 8)
		done := make(chan struct{})
		for i := 0; i < 4; i++ {
			go func() {
				hh := sha256.New()
				for c := range jobs {
					hh.Reset()
					hh.Write(c)
					hh.Sum(nil)
				}
				done <- struct{}{}
			}()
		}
		newBuf := func() []byte {
			if sized {
				return make([]byte, 0, 4<<20)
			}
			return make([]byte, 0)
		}
		cur := newBuf()
		n, d = walk(func(b []byte) error {
			for {
				p := ch.Scan(b)
				if p == 0 {
					cur = append(cur, b...)
					return nil
				}
				cur = append(cur, b[:p]...)
				jobs <- cur
				cur = newBuf()
				b = b[p:]
			}
		})
		jobs <- cur
		close(jobs)
		for i := 0; i < 4; i++ {
			<-done
		}
		label := "boundaries, SHA-256 x4 (grown)"
		if sized {
			label = "boundaries, SHA-256 x4 (sized)"
		}
		report(label, n, d)
	}

	newc, reused := new(atomic.Uint64), new(atomic.Uint64)
	c := &ChunkState{}
	c.reset(newc, reused, haxmap.New[string, bool]())
	c.startUploads(func(b []byte) [32]byte { return sha256.Sum256(b) }, func(uint64, string, []byte) error { return nil })
	client := &pbscommon.PBSClient{}
	n, d = walk(func(b []byte) error { return c.HandleData(b, client) })
	start := time.Now()
	c.stopUploads(false) // the hashing still under way
	report("ChunkState, sends stubbed", n, d+time.Since(start))
}
