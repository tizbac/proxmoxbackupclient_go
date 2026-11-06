package pbscommon

import (
	"sync"
	"sync/atomic"
	"time"
)

// How many chunks one backup session sends to PBS at a time. The backups hash
// with up to eight workers and, on a fast link, send with all of them: eight
// uploads in flight nearly doubled the speed of sending one at a time. On a
// slow link they only share it, and each chunk takes eight times as long to
// arrive: at 1 Mbit/s a 4 MB chunk then took over four minutes, close to the
// 5 minute limit an upload had then, and PBS answered nothing for so long that the
// connection's liveness check gave up on it (found 2026-11-04 on a link shaped
// to 1 Mbit/s, which the one-at-a-time client of before managed in 203 s).
//
// So by default (auto) a session starts with one upload and allows one more
// for every 4 Mbit/s it measures, up to MaxUploadThreads: about one chunk
// arriving every eight seconds per upload. A fixed number set with
// SetUploadThreads (the CLI's -upload-threads, the app's Upload threads)
// overrides that. Only the sending is limited: hashing and the check against
// the chunks PBS already has still run on all the workers.

// MaxUploadThreads is the most uploads one session runs at a time, and the
// largest fixed number SetUploadThreads takes.
const MaxUploadThreads = 8

// autoMbitPerThread is the upload speed in Mbit/s that each further upload
// thread needs in auto.
const autoMbitPerThread = 4

var uploadThreads atomic.Int32

// SetUploadThreads sets how many chunks a session sends at a time, 1 to
// MaxUploadThreads; 0 (or anything out of range) is auto. A change applies at
// once, also to backups already running.
func SetUploadThreads(n int) {
	if n < 0 || n > MaxUploadThreads {
		n = 0
	}
	uploadThreads.Store(int32(n))
}

// UploadThreads returns the fixed number of upload threads, or 0 for auto.
func UploadThreads() int {
	return int(uploadThreads.Load())
}

// uploadGate holds a session's uploads to the number allowed, and measures
// the session's upload speed for auto: the bytes sent divided by the time at
// least one upload was under way, taken afresh once a measurement has 8 MB or
// 3 s in it, so a change in the link is followed within a few chunks.
type uploadGate struct {
	mu       sync.Mutex
	cond     *sync.Cond
	inFlight int

	bytesPerSec float64 // last measurement, 0 until there is one

	busySince time.Time     // when inFlight last went from 0 to 1
	busy      time.Duration // time with an upload under way, this measurement
	sent      int64         // bytes sent, this measurement
}

const (
	gateSampleBytes = 8 << 20
	gateSampleTime  = 3 * time.Second
)

func newUploadGate() *uploadGate {
	g := &uploadGate{}
	g.cond = sync.NewCond(&g.mu)
	return g
}

// allowedLocked is how many uploads may be under way now.
func (g *uploadGate) allowedLocked() int {
	if n := UploadThreads(); n > 0 {
		return n
	}
	n := 1 + int(g.bytesPerSec*8/1e6/autoMbitPerThread)
	return min(n, MaxUploadThreads)
}

// acquire waits until another upload may start.
func (g *uploadGate) acquire() {
	g.mu.Lock()
	defer g.mu.Unlock()
	for g.inFlight >= g.allowedLocked() {
		g.cond.Wait()
	}
	if g.inFlight == 0 {
		g.busySince = time.Now()
	}
	g.inFlight++
}

// release ends an upload that sent n bytes (0 if it failed), and lets the
// next one start.
func (g *uploadGate) release(n int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := time.Now()
	g.busy += now.Sub(g.busySince)
	g.busySince = now // acquire starts it again after an idle spell
	g.inFlight--
	g.sent += int64(n)
	if n > 0 && g.busy > 0 && (g.sent >= gateSampleBytes || g.busy >= gateSampleTime) {
		g.bytesPerSec = float64(g.sent) / g.busy.Seconds()
		g.sent, g.busy = 0, 0
	}
	g.cond.Broadcast()
}

// threads is how many uploads are allowed now, for the log.
func (g *uploadGate) threads() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.allowedLocked()
}
