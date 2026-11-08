package pbscommon

import (
	"context"
	"sync"
	"time"
)

// PauseGate lets the user pause a running backup and carry on later in the
// same PBS session, so nothing already read has to be read again (a backup
// that is stopped and started over reads everything from the beginning).
//
// The backup calls Wait at its safe points: before each file or folder, every
// read block of a large file, and every block of a disk image. While the gate
// is closed Wait blocks, so the reading stops and the upload queue drains. The
// HTTP/2 connection to PBS stays open meanwhile: the transport pings an idle
// connection (see ReadIdleTimeout in pbsapi.go).
//
// A nil *PauseGate is valid and never pauses, so callers that do not offer
// Pause need not set one.
type PauseGate struct {
	mu      sync.Mutex
	resumed chan struct{} // nil while running; closed on Resume
	since   time.Time
}

// Pause closes the gate. It reports false if it was already paused.
func (g *PauseGate) Pause() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.resumed != nil {
		return false
	}
	g.resumed = make(chan struct{})
	g.since = time.Now()
	return true
}

// Resume opens the gate and returns how long it was paused. It reports false
// if it was not paused.
func (g *PauseGate) Resume() (time.Duration, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.resumed == nil {
		return 0, false
	}
	close(g.resumed)
	g.resumed = nil
	return time.Since(g.since), true
}

// Paused reports whether the gate is closed, and since when.
func (g *PauseGate) Paused() (bool, time.Time) {
	if g == nil {
		return false, time.Time{}
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.resumed != nil, g.since
}

// Wait returns at once while the gate is open. While it is closed it blocks
// until Resume, or until ctx is cancelled (Stop pressed during a pause), in
// which case it returns ctx's error.
func (g *PauseGate) Wait(ctx context.Context) error {
	if g == nil {
		return nil
	}
	g.mu.Lock()
	ch := g.resumed
	g.mu.Unlock()
	if ch == nil {
		return nil
	}
	if ctx == nil {
		<-ch
		return nil
	}
	select {
	case <-ch:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
