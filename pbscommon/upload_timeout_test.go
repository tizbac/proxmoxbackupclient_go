package pbscommon

import (
	"context"
	"crypto/rand"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// stallTimes shortens the stall time, the answer floor and the most an upload
// may take, for one test.
func stallTimes(t *testing.T, stall time.Duration, floor int, maxTime time.Duration) {
	t.Helper()
	savedStall, savedFloor, savedMax := chunkStallTime, chunkAnswerFloor, chunkUploadMaxTime
	chunkStallTime = func() time.Duration { return stall }
	chunkAnswerFloor = floor
	chunkUploadMaxTime = maxTime
	t.Cleanup(func() { chunkStallTime, chunkAnswerFloor, chunkUploadMaxTime = savedStall, savedFloor, savedMax })
}

func randomChunk(t *testing.T, n int) []byte {
	t.Helper()
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return b
}

func wantStall(t *testing.T, err error, answer, total bool) {
	t.Helper()
	var stall *UploadStallError
	if !errors.As(err, &stall) {
		t.Fatalf("error = %v; want an *UploadStallError", err)
	}
	if stall.Answer != answer || stall.Total != total {
		t.Errorf("stall = %+v; want Answer %v, Total %v", *stall, answer, total)
	}
	// The callers' retries (retry.DefaultRetryable) send it again.
	if !errors.Is(err, context.DeadlineExceeded) || !strings.Contains(err.Error(), "timeout") {
		t.Errorf("error = %v; want it to count as a timeout", err)
	}
}

// TestUploadChunkNoAnswer: a server that takes the chunk but never answers (a
// stuck stream, issue #59) fails the upload once the answer time is over,
// instead of holding the backup for ever.
func TestUploadChunkNoAnswer(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.ReadAll(r.Body)
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(release)
	// A floor so high the answer gets only the stall time.
	stallTimes(t, 200*time.Millisecond, 1<<40, time.Minute)

	client := newTestClient(srv.URL, nil)
	pt := randomChunk(t, 4096)
	start := time.Now()
	err := client.UploadChunk(1, client.ChunkDigestHex(pt), pt, true, false)
	wantStall(t, err, true, false)
	if d := time.Since(start); d < 200*time.Millisecond || d > 5*time.Second {
		t.Errorf("UploadChunk took %v; want about 200ms", d)
	}
}

// TestUploadChunkStalledSend: a server that stops taking the chunk part way
// (a link that has stopped) fails the upload once nothing has moved for the
// stall time. The chunk is larger than the loopback socket buffers, so the
// sending itself stops.
func TestUploadChunkStalledSend(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.CopyN(io.Discard, r.Body, 1<<20) // takes 1 MiB, then nothing
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(release)
	stallTimes(t, 300*time.Millisecond, 32*1024, time.Minute)

	client := newTestClient(srv.URL, nil)
	pt := randomChunk(t, 48<<20)
	start := time.Now()
	err := client.UploadChunk(1, client.ChunkDigestHex(pt), pt, true, false)
	wantStall(t, err, false, false)
	if d := time.Since(start); d > 10*time.Second {
		t.Errorf("UploadChunk took %v; want about the 300ms stall time", d)
	}
}

// TestUploadChunkSlowButMoving: a chunk that takes far longer than the stall
// time to arrive, but keeps moving (a slow or shared link), is not given up.
func TestUploadChunkSlowButMoving(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 64*1024)
		for {
			if _, err := io.ReadFull(r.Body, buf); err != nil {
				break
			}
			time.Sleep(5 * time.Millisecond) // about 12 MB/s
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	stallTimes(t, 300*time.Millisecond, 32*1024, time.Minute)

	client := newTestClient(srv.URL, nil)
	pt := randomChunk(t, 32<<20)
	start := time.Now()
	if err := client.UploadChunk(1, client.ChunkDigestHex(pt), pt, true, false); err != nil {
		t.Fatalf("UploadChunk over a slow link: %v", err)
	}
	if d := time.Since(start); d < 1*time.Second {
		t.Errorf("UploadChunk took %v; the test needs it to take several stall times", d)
	}
}

// TestUploadChunkMaxTime: an upload that keeps trickling is still given up
// after chunkUploadMaxTime.
func TestUploadChunkMaxTime(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 1024)
		for {
			if _, err := io.ReadFull(r.Body, buf); err != nil {
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
	}))
	defer srv.Close()
	stallTimes(t, time.Second, 32*1024, 400*time.Millisecond)

	client := newTestClient(srv.URL, nil)
	pt := randomChunk(t, 48<<20)
	err := client.UploadChunk(1, client.ChunkDigestHex(pt), pt, true, false)
	wantStall(t, err, false, true)
}

// TestSetChunkStallTimeout: the setting takes 30 s to 30 min; 0 and anything
// else is the default.
func TestSetChunkStallTimeout(t *testing.T) {
	t.Cleanup(func() { SetChunkStallTimeout(0) })
	for _, c := range []struct{ set, want time.Duration }{
		{0, DefaultChunkStallTimeout},
		{2 * time.Minute, 2 * time.Minute},
		{MinChunkStallTimeout, MinChunkStallTimeout},
		{MaxChunkStallTimeout, MaxChunkStallTimeout},
		{10 * time.Second, DefaultChunkStallTimeout},
		{time.Hour, DefaultChunkStallTimeout},
		{-time.Second, DefaultChunkStallTimeout},
	} {
		SetChunkStallTimeout(c.set)
		if got := chunkStallTime(); got != c.want {
			t.Errorf("SetChunkStallTimeout(%v): stall time %v; want %v", c.set, got, c.want)
		}
	}
}

// TestCloseIfQuiet: a stalled upload closes the session's connection only
// when nothing has come from PBS on it for the stall time; a connection that
// still hears from PBS is left for the stuck upload to be retried on.
func TestCloseIfQuiet(t *testing.T) {
	a, b := net.Pipe()
	defer b.Close()
	lc := newLimitedConn(a)
	pbs := &PBSClient{activeConn: lc}

	if _, closed := pbs.closeIfQuiet(time.Second); closed {
		t.Fatal("closed a connection that has just been read from")
	}
	go func() { _, _ = b.Write([]byte("x")) }()
	if _, err := lc.Read(make([]byte, 1)); err != nil {
		t.Fatal(err)
	}
	lc.lastRead.Store(time.Now().Add(-2 * time.Second).UnixNano())
	if _, closed := pbs.closeIfQuiet(time.Second); closed {
		t.Fatal("closed a connection whose socket has just taken data to send")
	}
	lc.lastWrite.Store(time.Now().Add(-2 * time.Second).UnixNano())
	quiet, closed := pbs.closeIfQuiet(time.Second)
	if !closed || quiet < 2*time.Second {
		t.Fatalf("quiet %v, closed %v; want it closed after 2s of nothing", quiet, closed)
	}
	if _, err := lc.Write([]byte("y")); err == nil {
		t.Error("the connection still takes writes after closeIfQuiet closed it")
	}
	if _, closed := (&PBSClient{}).closeIfQuiet(time.Second); closed {
		t.Error("closeIfQuiet with no connection reported closing one")
	}
}

// TestWatchUploadSharedConnection: an upload waiting its turn while the
// session's connection sends other uploads' data is not given up; once the
// connection stops sending data it is. Small writes (the transport's pings)
// do not count as sending.
func TestWatchUploadSharedConnection(t *testing.T) {
	stallTimes(t, 300*time.Millisecond, 1<<40, time.Minute)
	a, b := net.Pipe()
	defer b.Close()
	go func() { _, _ = io.Copy(io.Discard, b) }()
	lc := newLimitedConn(a)
	defer lc.Close()

	// write sends n bytes every 50 ms until stop is closed.
	write := func(n int, stop chan struct{}) {
		for {
			select {
			case <-stop:
				return
			case <-time.After(50 * time.Millisecond):
				_, _ = lc.Write(make([]byte, n))
			}
		}
	}
	watch := func() (context.Context, func()) {
		ctx, cancel := context.WithCancelCause(context.Background())
		body := newUploadProgress(1 << 20) // never read: this upload waits
		stop := watchUpload(cancel, body, lc)
		return ctx, stop
	}

	others := make(chan struct{})
	go write(16*1024, others)
	ctx, stop := watch()
	time.Sleep(time.Second)
	if context.Cause(ctx) != nil {
		t.Fatalf("given up while the connection sent other data: %v", context.Cause(ctx))
	}
	close(others)
	time.Sleep(700 * time.Millisecond)
	var stall *UploadStallError
	if !errors.As(context.Cause(ctx), &stall) {
		t.Errorf("not given up after the connection stopped sending: %v", context.Cause(ctx))
	}
	stop()

	pings := make(chan struct{})
	go write(40, pings)
	defer close(pings)
	ctx, stop = watch()
	defer stop()
	time.Sleep(700 * time.Millisecond)
	if !errors.As(context.Cause(ctx), &stall) {
		t.Errorf("small writes kept a stuck upload alive: %v", context.Cause(ctx))
	}
}

// TestProgressBodyAgain: a body asked for again (GetBody, after a refused
// stream) reads the whole chunk from the start and starts the progress afresh.
func TestProgressBodyAgain(t *testing.T) {
	data := []byte("chunk data")
	p := newUploadProgress(len(data))
	first, _ := io.ReadAll(p.body(data))
	if string(first) != string(data) || !p.done.Load() {
		t.Fatalf("first read %q, done %v", first, p.done.Load())
	}
	again := p.body(data)
	if p.done.Load() {
		t.Error("a new body starts as not done")
	}
	second, _ := io.ReadAll(again)
	if string(second) != string(data) || !p.done.Load() {
		t.Errorf("second read %q, done %v", second, p.done.Load())
	}
}
