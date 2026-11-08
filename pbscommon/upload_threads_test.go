package pbscommon

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Auto allows one upload until there is a measurement, then one more for
// every 4 Mbit/s, up to MaxUploadThreads.
func TestUploadGateAutoThreads(t *testing.T) {
	SetUploadThreads(0)
	for _, c := range []struct {
		mbit float64
		want int
	}{{0, 1}, {1, 1}, {3.9, 1}, {4, 2}, {7.9, 2}, {12, 4}, {28, 8}, {1000, 8}} {
		g := newUploadGate()
		g.bytesPerSec = c.mbit * 1e6 / 8
		if got := g.threads(); got != c.want {
			t.Errorf("%v Mbit/s: %d threads, want %d", c.mbit, got, c.want)
		}
	}
}

// A fixed number overrides auto, whatever was measured; out of range is auto.
func TestUploadGateFixedThreads(t *testing.T) {
	t.Cleanup(func() { SetUploadThreads(0) })
	g := newUploadGate()
	g.bytesPerSec = 1e9
	SetUploadThreads(2)
	if got := g.threads(); got != 2 {
		t.Errorf("fixed 2: %d", got)
	}
	SetUploadThreads(MaxUploadThreads + 1)
	if UploadThreads() != 0 || g.threads() != MaxUploadThreads {
		t.Errorf("out of range: setting %d, threads %d", UploadThreads(), g.threads())
	}
}

// No more uploads run at once than allowed.
func TestUploadGateHoldsUploads(t *testing.T) {
	t.Cleanup(func() { SetUploadThreads(0) })
	SetUploadThreads(3)
	g := newUploadGate()
	var now, most atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			g.acquire()
			n := now.Add(1)
			for m := most.Load(); n > m && !most.CompareAndSwap(m, n); m = most.Load() {
			}
			time.Sleep(5 * time.Millisecond)
			now.Add(-1)
			g.release(1000)
		}()
	}
	wg.Wait()
	if most.Load() != 3 {
		t.Errorf("at most %d at once, want 3", most.Load())
	}
}

// The speed is measured over the time an upload was under way: a quick 8 MB
// on a fast link opens all the threads, and a slow one keeps one.
func TestUploadGateMeasures(t *testing.T) {
	SetUploadThreads(0)
	fast := newUploadGate()
	fast.acquire()
	time.Sleep(20 * time.Millisecond)
	fast.release(gateSampleBytes) // 8 MB in ~20 ms
	if got := fast.threads(); got != MaxUploadThreads {
		t.Errorf("fast link: %d threads (%.0f bytes/s)", got, fast.bytesPerSec)
	}

	slow := newUploadGate()
	slow.acquire()
	// Started 3 s ago, and sent 0.5 MB: 1.3 Mbit/s.
	slow.busySince = time.Now().Add(-gateSampleTime)
	slow.release(500_000)
	if got := slow.threads(); got != 1 {
		t.Errorf("slow link: %d threads (%.0f bytes/s)", got, slow.bytesPerSec)
	}

	// Idle time between uploads does not count.
	idle := newUploadGate()
	idle.acquire()
	idle.release(1 << 20)
	time.Sleep(50 * time.Millisecond)
	idle.acquire()
	idle.busySince = idle.busySince.Add(-gateSampleTime)
	idle.release(3 << 20) // 4 MB in ~3 s busy: about 11 Mbit/s, 3 threads
	if got := idle.threads(); got != 3 {
		t.Errorf("with idle time: %d threads (%.0f bytes/s)", got, idle.bytesPerSec)
	}
}
