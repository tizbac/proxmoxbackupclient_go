package pbscommon

import (
	"context"
	"fmt"
	"io"
	"sync/atomic"
	"time"
)

// When a chunk upload is given up. A fixed limit for the whole upload
// (tizbac/proxmoxbackupclient_go#59) could not tell a slow link from a dead
// one: over several Wi-Fi hops shared with other backups a chunk took longer
// than 5 minutes while it was still moving, and was given up (2026-11-05),
// while a link that had stopped was only noticed after the same 5 minutes.
//
// So the upload is watched as it goes. The transport takes the chunk from us
// only as PBS makes room for it (HTTP/2 flow control, then the socket), so
// while bytes are being taken the link is moving. An upload is given up when
// none has been taken for the stall time (ChunkStallTimeout, 90 seconds by
// default). Once the last byte has been taken, PBS still has to receive what
// is buffered on the way, store the chunk and answer: that gets the stall time
// plus the time the chunk would take at chunkAnswerFloor, a rate no usable
// backup link is slower than. A dead connection is found sooner by the
// transport's pings (ReadIdleTimeout in pbsapi.go).

// DefaultChunkStallTimeout is how long an upload may make no progress before
// it is given up, unless SetChunkStallTimeout set another.
const DefaultChunkStallTimeout = 90 * time.Second

// The range SetChunkStallTimeout takes; anything else means the default.
const (
	MinChunkStallTimeout = 30 * time.Second
	MaxChunkStallTimeout = 30 * time.Minute
)

// chunkAnswerFloor is the slowest rate, in bytes per second, at which the
// part of a chunk still on its way to PBS is waited for once all of it has
// been handed over (32 KiB/s, 256 kbit/s: a 4 MiB chunk gets 128 s more). A
// variable so tests can change it.
var chunkAnswerFloor = 32 * 1024

// chunkUploadMaxTime bounds one upload however steadily it moves, so a link
// that keeps trickling a byte now and then cannot hold a backup for ever. At
// 37 kbit/s a 16 MiB chunk still fits. A variable so tests can shorten it.
var chunkUploadMaxTime = 60 * time.Minute

var chunkStallSetting atomic.Int64 // nanoseconds; 0 = default

// SetChunkStallTimeout sets how long a chunk upload may make no progress
// before it is given up (the CLI's -chunk-timeout, the app's Chunk timeout).
// 0, or anything outside MinChunkStallTimeout to MaxChunkStallTimeout, is the
// default. A change applies to the uploads that start after it.
func SetChunkStallTimeout(d time.Duration) {
	if d < MinChunkStallTimeout || d > MaxChunkStallTimeout {
		d = 0
	}
	chunkStallSetting.Store(int64(d))
}

// ChunkStallTimeout returns the stall time set with SetChunkStallTimeout, or
// 0 for the default.
func ChunkStallTimeout() time.Duration {
	return time.Duration(chunkStallSetting.Load())
}

// chunkStallTime is the stall time uploads use now. A variable so tests can
// shorten it below MinChunkStallTimeout.
var chunkStallTime = func() time.Duration {
	if d := ChunkStallTimeout(); d > 0 {
		return d
	}
	return DefaultChunkStallTimeout
}

// UploadStallError is why a chunk upload was given up. It counts as a
// timeout (errors.Is context.DeadlineExceeded), so the callers' retries send
// the chunk again.
type UploadStallError struct {
	Answer bool          // all of it was sent; PBS did not answer
	Idle   time.Duration // how long nothing happened
	Total  bool          // chunkUploadMaxTime ran out while it still moved
}

func (e *UploadStallError) Error() string {
	switch {
	case e.Total:
		return fmt.Sprintf("chunk upload timeout: still not done after %v", e.Idle.Round(time.Second))
	case e.Answer:
		return fmt.Sprintf("chunk upload timeout: sent, but no answer from PBS within %v", e.Idle.Round(time.Second))
	default:
		return fmt.Sprintf("chunk upload timeout: nothing sent for %v", e.Idle.Round(time.Second))
	}
}

func (e *UploadStallError) Unwrap() error { return context.DeadlineExceeded }

// uploadProgress is when the transport last took some of a chunk's body, and
// whether it has taken all of it.
type uploadProgress struct {
	size int
	last atomic.Int64 // unix nanoseconds of the last read that took bytes
	done atomic.Bool  // every byte has been taken
}

func newUploadProgress(size int) *uploadProgress {
	p := &uploadProgress{size: size}
	p.last.Store(time.Now().UnixNano())
	return p
}

// sessionConn is the session's connection to PBS, or nil (none yet, or a
// client made without one, as in tests).
func (pbs *PBSClient) sessionConn() *limitedConn {
	pbs.activeConnMu.Lock()
	defer pbs.activeConnMu.Unlock()
	c, _ := pbs.activeConn.(*limitedConn)
	return c
}

// closeIfQuiet closes the session's connection to PBS when for at least d
// nothing at all has come from PBS on it (not an answer, not a flow-control
// update, not the answer to the transport's ping) and its socket has taken
// nothing to send. An upload that stopped on such a connection is not one
// stuck stream but a link that has gone, and every upload on it would stall in
// turn; closed, the uploads fail as a lost connection (retry.SessionLost) and
// the backup starts again with a new session. On a slow link answers can be
// late by a minute or more, queued behind data, but then the socket is still
// taking data. It reports how long the connection was quiet and whether it
// closed it.
func (pbs *PBSClient) closeIfQuiet(d time.Duration) (time.Duration, bool) {
	c := pbs.sessionConn()
	if c == nil {
		return 0, false
	}
	quiet := c.quietFor()
	if quiet < d {
		return quiet, false
	}
	_ = c.Close()
	return quiet, true
}

// progressBody is a chunk's request body that reports to its uploadProgress.
type progressBody struct {
	data []byte
	off  int
	p    *uploadProgress
}

// body returns a new reader of data from the start. The transport asks for
// one again (GetBody) when it sends the request a second time, as after a
// refused stream; that starts the progress afresh.
func (p *uploadProgress) body(data []byte) io.ReadCloser {
	p.done.Store(false)
	p.last.Store(time.Now().UnixNano())
	return io.NopCloser(&progressBody{data: data, p: p})
}

func (b *progressBody) Read(buf []byte) (int, error) {
	if b.off >= len(b.data) {
		b.p.done.Store(true)
		return 0, io.EOF
	}
	n := copy(buf, b.data[b.off:])
	b.off += n
	b.p.last.Store(time.Now().UnixNano())
	if b.off >= len(b.data) {
		b.p.done.Store(true)
	}
	return n, nil
}

// watchUpload cancels the upload (through cancel, with an *UploadStallError
// as the cause) when it stops moving, or PBS does not answer in time once all
// of it is sent, or the upload as a whole takes chunkUploadMaxTime. Call the
// returned stop when the request is over.
//
// It is moving while its body is being taken, or while the session's
// connection (conn, nil in tests without one) takes anything to send: the
// uploads of a session share one connection, and on a slow link the
// transport may feed the others for minutes while this one waits its turn
// (found 2026-11-05: four uploads on 400 kbit/s, one given up after 90 s of
// waiting while the link was full). For the same reason the wait for PBS's
// answer starts again whenever the connection sends: what is ahead of this
// chunk on the way has to arrive first.
func watchUpload(cancel context.CancelCauseFunc, body *uploadProgress, conn *limitedConn) (stop func()) {
	stall := chunkStallTime()
	answer := stall + time.Duration(float64(body.size)/float64(chunkAnswerFloor)*float64(time.Second))
	maxTime := chunkUploadMaxTime
	tick := min(max(stall/10, 10*time.Millisecond), time.Second)
	start := time.Now()
	quit := make(chan struct{})
	go func() {
		t := time.NewTicker(tick)
		defer t.Stop()
		for {
			select {
			case <-quit:
				return
			case now := <-t.C:
				last := body.last.Load()
				if conn != nil {
					last = max(last, conn.lastWrite.Load())
				}
				idle := now.Sub(time.Unix(0, last))
				switch {
				case now.Sub(start) > maxTime:
					cancel(&UploadStallError{Total: true, Idle: now.Sub(start)})
					return
				case body.done.Load() && idle > answer:
					cancel(&UploadStallError{Answer: true, Idle: idle})
					return
				case !body.done.Load() && idle > stall:
					cancel(&UploadStallError{Idle: idle})
					return
				}
			}
		}
	}()
	return func() { close(quit) }
}
