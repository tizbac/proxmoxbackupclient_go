package pbscommon

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

// Speed limits (Preferences > Backup Options). Each kind has one budget for
// the whole process, shared by every backup and restore running in it at the
// same time: three backups under a 10 MB/s upload limit share 10 MB/s, they
// do not get 10 MB/s each. A limit can change at any time and applies at once,
// also to runs already going (the wrappers below always charge the current
// budget, and a goroutine waiting under the old limit starts again under the
// new one).
//
// The budgets are charged at the narrowest points the bytes pass through:
// upload and download on the HTTP/2 connection to PBS (limitedConn, made in
// Connect for backup and reader sessions alike), disk reads where a folder
// backup reads a file (PXARArchive.openSource, the read-ahead workers) and
// where a machine backup reads a disk (LimitReaderAt, LimitReader).

// LimitKind names one of the budgets.
type LimitKind int

const (
	// LimitUpload is bytes sent to PBS.
	LimitUpload LimitKind = iota
	// LimitDownload is bytes read from PBS.
	LimitDownload
	// LimitDiskRead is bytes a backup reads from local storage.
	LimitDiskRead
	limitKinds
)

var limiters = [limitKinds]*RateLimiter{
	LimitUpload:   NewRateLimiter(0),
	LimitDownload: NewRateLimiter(0),
	LimitDiskRead: NewRateLimiter(0),
}

// SetSpeedLimit sets the budget for kind in bytes per second; 0 (or less)
// means no limit.
func SetSpeedLimit(kind LimitKind, bytesPerSec int64) {
	if kind >= 0 && kind < limitKinds {
		limiters[kind].SetLimit(bytesPerSec)
	}
}

// SpeedLimit returns the budget for kind in bytes per second, 0 for none.
func SpeedLimit(kind LimitKind) int64 {
	if kind >= 0 && kind < limitKinds {
		return limiters[kind].Limit()
	}
	return 0
}

// WaitSpeedLimit charges n bytes to kind's budget, waiting while it is spent.
func WaitSpeedLimit(ctx context.Context, kind LimitKind, n int) error {
	if kind >= 0 && kind < limitKinds {
		return limiters[kind].Wait(ctx, n)
	}
	return nil
}

// RateLimiter is a token bucket shared by any number of goroutines. The zero
// limit is "unlimited", and then Wait costs one atomic load.
type RateLimiter struct {
	rate atomic.Int64 // bytes per second, 0 = unlimited

	mu      sync.Mutex
	tokens  float64 // may go below zero: bytes taken ahead of time
	last    time.Time
	changed chan struct{} // closed and replaced when the limit changes
}

// NewRateLimiter returns a limiter of bytesPerSec (0: unlimited).
func NewRateLimiter(bytesPerSec int64) *RateLimiter {
	l := &RateLimiter{changed: make(chan struct{})}
	l.SetLimit(bytesPerSec)
	return l
}

// burstFor is how many bytes may pass at once at rate r: a fifth of a
// second's worth, so a long transfer is split into short waits (Stop is never
// held up by more than about that much per waiter) and an idle limiter does
// not let a large burst through afterwards.
func burstFor(r int64) int64 {
	const minBurst = 32 * 1024
	if b := r / 5; b > minBurst {
		return b
	}
	return minBurst
}

// SetLimit changes the limit. The bucket starts empty under the new one, and
// goroutines waiting under the old one take their bytes again at the new
// rate, so a lower limit holds at once and a higher one (or none) frees them.
// Setting the same limit again changes nothing: the service sets it from its
// settings before every run.
func (l *RateLimiter) SetLimit(bytesPerSec int64) {
	if bytesPerSec < 0 {
		bytesPerSec = 0
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.rate.Load() == bytesPerSec {
		return
	}
	l.rate.Store(bytesPerSec)
	l.tokens = 0
	l.last = time.Now()
	close(l.changed)
	l.changed = make(chan struct{})
}

// Limit returns the limit in bytes per second, 0 for none.
func (l *RateLimiter) Limit() int64 {
	return l.rate.Load()
}

// take takes n bytes from the bucket and says how long the caller must wait
// before using them (0: at once), and the channel that is closed if the limit
// changes meanwhile.
func (l *RateLimiter) take(n int64) (time.Duration, chan struct{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	r := l.rate.Load()
	if r <= 0 {
		return 0, nil
	}
	now := time.Now()
	l.tokens += now.Sub(l.last).Seconds() * float64(r)
	l.last = now
	if b := float64(burstFor(r)); l.tokens > b {
		l.tokens = b
	}
	l.tokens -= float64(n)
	if l.tokens >= 0 {
		return 0, nil
	}
	return time.Duration(-l.tokens / float64(r) * float64(time.Second)), l.changed
}

// giveBack returns n bytes a cancelled waiter did not use, unless the limit
// changed since it took them (the bucket was emptied then anyway).
func (l *RateLimiter) giveBack(n int64, taken chan struct{}) {
	l.mu.Lock()
	if l.changed == taken {
		l.tokens += float64(n)
	}
	l.mu.Unlock()
}

// Wait charges n bytes and returns when they may be used. A large n is taken
// in burst-sized pieces, so several goroutines share the budget fairly. It
// returns ctx's error at once when ctx is cancelled (a nil ctx never is).
func (l *RateLimiter) Wait(ctx context.Context, n int) error {
	if l == nil || n <= 0 || l.rate.Load() <= 0 {
		return nil
	}
	var done <-chan struct{}
	if ctx != nil {
		done = ctx.Done()
	}
	left := int64(n)
	for left > 0 {
		r := l.rate.Load()
		if r <= 0 {
			return nil
		}
		piece := min(left, burstFor(r))
		if err := l.waitPiece(ctx, done, piece); err != nil {
			return err
		}
		left -= piece
	}
	return nil
}

func (l *RateLimiter) waitPiece(ctx context.Context, done <-chan struct{}, piece int64) error {
	for {
		d, changed := l.take(piece)
		if d <= 0 {
			return nil
		}
		t := time.NewTimer(d)
		select {
		case <-t.C:
			return nil
		case <-done:
			t.Stop()
			l.giveBack(piece, changed)
			return ctx.Err()
		case <-changed:
			t.Stop() // take the piece again under the new limit
		}
	}
}

// limitedConn charges what goes over a connection to PBS: writes to the
// upload budget, reads to the download budget. Close ends any wait at once.
type limitedConn struct {
	net.Conn
	ctx    context.Context
	cancel context.CancelFunc
}

func newLimitedConn(c net.Conn) *limitedConn {
	ctx, cancel := context.WithCancel(context.Background())
	return &limitedConn{Conn: c, ctx: ctx, cancel: cancel}
}

func (c *limitedConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	if n > 0 {
		_ = limiters[LimitDownload].Wait(c.ctx, n)
	}
	return n, err
}

// Write waits before sending, so the bytes leave at the limited pace.
func (c *limitedConn) Write(p []byte) (int, error) {
	if err := limiters[LimitUpload].Wait(c.ctx, len(p)); err != nil {
		return 0, net.ErrClosed
	}
	return c.Conn.Write(p)
}

func (c *limitedConn) Close() error {
	c.cancel()
	return c.Conn.Close()
}

// ConnectionState passes the TLS state through: the HTTP/2 transport reads it
// from the connection it is given to fill in each response's TLS field.
func (c *limitedConn) ConnectionState() tls.ConnectionState {
	if cs, ok := c.Conn.(interface{ ConnectionState() tls.ConnectionState }); ok {
		return cs.ConnectionState()
	}
	return tls.ConnectionState{}
}

// limitedReader charges what it reads to one budget after each read. A
// cancelled ctx only ends the wait early; the caller notices the cancel
// itself, so the read's own result is passed on unchanged.
type limitedReader struct {
	r    io.Reader
	ctx  context.Context
	kind LimitKind
}

// LimitReader returns r with its reads charged to kind's budget.
func LimitReader(ctx context.Context, kind LimitKind, r io.Reader) io.Reader {
	return &limitedReader{r: r, ctx: ctx, kind: kind}
}

func (l *limitedReader) Read(p []byte) (int, error) {
	n, err := l.r.Read(p)
	if n > 0 {
		_ = WaitSpeedLimit(l.ctx, l.kind, n)
	}
	return n, err
}

// limitedReadSeekCloser is limitedReader for a file the archiver may seek.
type limitedReadSeekCloser struct {
	io.ReadSeekCloser
	ctx  context.Context
	kind LimitKind
}

func (l *limitedReadSeekCloser) Read(p []byte) (int, error) {
	n, err := l.ReadSeekCloser.Read(p)
	if n > 0 {
		_ = WaitSpeedLimit(l.ctx, l.kind, n)
	}
	return n, err
}

// limitedReaderAt charges what it reads to one budget after each read.
type limitedReaderAt struct {
	r    io.ReaderAt
	ctx  context.Context
	kind LimitKind
}

// LimitReaderAt returns r with its reads charged to kind's budget.
func LimitReaderAt(ctx context.Context, kind LimitKind, r io.ReaderAt) io.ReaderAt {
	return &limitedReaderAt{r: r, ctx: ctx, kind: kind}
}

func (l *limitedReaderAt) ReadAt(p []byte, off int64) (int, error) {
	n, err := l.r.ReadAt(p, off)
	if n > 0 {
		_ = WaitSpeedLimit(l.ctx, l.kind, n)
	}
	return n, err
}

// openSource opens a file the archiver is about to read, with its reads
// charged to the disk read budget. Every read of a file's contents in
// WriteFile goes through a handle from here (the read-ahead workers use
// readFileLimited), so the folder backup's disk reads are counted in one place.
func (a *PXARArchive) openSource(path string) (io.ReadSeekCloser, error) {
	f, err := openForRead(path)
	if err != nil {
		return nil, err
	}
	return &limitedReadSeekCloser{ReadSeekCloser: f, ctx: a.Ctx, kind: LimitDiskRead}, nil
}

// readFileLimited is os.ReadFile charged to the disk read budget as it goes,
// for the folder backup's read-ahead workers. With no limit it is
// os.ReadFile itself.
func readFileLimited(ctx context.Context, path string) ([]byte, error) {
	if SpeedLimit(LimitDiskRead) <= 0 {
		return os.ReadFile(path)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	size := 0
	if fi, err := f.Stat(); err == nil {
		size = int(fi.Size())
	}
	data := make([]byte, 0, size+512)
	r := LimitReader(ctx, LimitDiskRead, f)
	// Read in pieces: the budget is charged after each read, so one read of
	// a whole large file would go through at full speed and wait afterwards.
	const piece = 256 * 1024
	for {
		if len(data) == cap(data) {
			data = append(data, 0)[:len(data)]
		}
		n, err := r.Read(data[len(data):min(cap(data), len(data)+piece)])
		data = data[:len(data)+n]
		if err == io.EOF {
			return data, nil
		}
		if err != nil {
			return data, err
		}
	}
}
