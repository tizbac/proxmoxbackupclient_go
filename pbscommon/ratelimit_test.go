package pbscommon

import (
	"bytes"
	"context"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// pump charges total bytes to l in pieces of size step and returns how long
// it took.
func pump(t *testing.T, l *RateLimiter, total, step int) time.Duration {
	t.Helper()
	start := time.Now()
	for sent := 0; sent < total; sent += step {
		if err := l.Wait(context.Background(), step); err != nil {
			t.Fatalf("Wait: %v", err)
		}
	}
	return time.Since(start)
}

func TestRateLimiterRate(t *testing.T) {
	const rate = 2 << 20
	l := NewRateLimiter(rate)
	// The bucket starts empty, so 2 MiB at 2 MiB/s takes about a second.
	took := pump(t, l, rate, 16*1024)
	if took < 850*time.Millisecond || took > 1400*time.Millisecond {
		t.Fatalf("2 MiB at 2 MiB/s took %v, want about 1s", took)
	}
}

func TestRateLimiterLargeWaitIsSplit(t *testing.T) {
	const rate = 1 << 20
	l := NewRateLimiter(rate)
	start := time.Now()
	// One call for a whole second's worth still keeps to the rate.
	if err := l.Wait(context.Background(), rate); err != nil {
		t.Fatal(err)
	}
	if took := time.Since(start); took < 850*time.Millisecond || took > 1400*time.Millisecond {
		t.Fatalf("1 MiB in one Wait at 1 MiB/s took %v, want about 1s", took)
	}
}

func TestRateLimiterSharedAcrossGoroutines(t *testing.T) {
	const rate = 2 << 20
	l := NewRateLimiter(rate)
	// Two readers of 1 MiB each share one 2 MiB/s budget: about a second for
	// both together, not half a second each in parallel.
	data := bytes.Repeat([]byte{1}, 1<<20)
	start := time.Now()
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := bytes.NewReader(data)
			buf := make([]byte, 32*1024)
			for {
				n, err := r.Read(buf)
				if n > 0 {
					if werr := l.Wait(context.Background(), n); werr != nil {
						t.Error(werr)
						return
					}
				}
				if err == io.EOF {
					return
				}
			}
		}()
	}
	wg.Wait()
	if took := time.Since(start); took < 850*time.Millisecond || took > 1500*time.Millisecond {
		t.Fatalf("two readers of 1 MiB under 2 MiB/s took %v, want about 1s", took)
	}
}

func TestRateLimiterUnlimited(t *testing.T) {
	l := NewRateLimiter(0)
	start := time.Now()
	for i := 0; i < 100000; i++ {
		if err := l.Wait(context.Background(), 1<<20); err != nil {
			t.Fatal(err)
		}
	}
	if took := time.Since(start); took > 200*time.Millisecond {
		t.Fatalf("100000 unlimited waits took %v", took)
	}
	var nilLimiter *RateLimiter
	if err := nilLimiter.Wait(context.Background(), 10); err != nil {
		t.Fatal(err)
	}
}

func TestRateLimiterUnlimitedDoesNotAllocate(t *testing.T) {
	l := NewRateLimiter(0)
	ctx := context.Background()
	if n := testing.AllocsPerRun(1000, func() { _ = l.Wait(ctx, 4096) }); n != 0 {
		t.Fatalf("unlimited Wait allocates %v times", n)
	}
}

func TestRateLimiterLiveChange(t *testing.T) {
	// A waiter far behind under a tiny limit is freed at once when the
	// limit is lifted.
	l := NewRateLimiter(64 * 1024)
	done := make(chan time.Duration, 1)
	go func() {
		start := time.Now()
		_ = l.Wait(context.Background(), 4<<20) // about a minute at 64 KiB/s
		done <- time.Since(start)
	}()
	time.Sleep(100 * time.Millisecond)
	l.SetLimit(0)
	select {
	case took := <-done:
		if took > time.Second {
			t.Fatalf("lifting the limit freed the waiter after %v", took)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("waiter still blocked after the limit was lifted")
	}

	// Raising the limit mid-transfer speeds the rest up.
	l.SetLimit(256 * 1024)
	go func() {
		start := time.Now()
		_ = l.Wait(context.Background(), 8<<20) // 32 s at 256 KiB/s
		done <- time.Since(start)
	}()
	time.Sleep(200 * time.Millisecond)
	l.SetLimit(64 << 20) // the rest in well under a second
	select {
	case took := <-done:
		if took > 2*time.Second {
			t.Fatalf("raising the limit: the transfer took %v", took)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("transfer did not speed up after the limit was raised")
	}

	// Setting the same limit again changes nothing.
	ch := l.changed
	l.SetLimit(64 << 20)
	if l.changed != ch {
		t.Fatal("setting the same limit again woke the waiters")
	}
	if l.Limit() != 64<<20 {
		t.Fatalf("Limit() = %d", l.Limit())
	}
}

func TestRateLimiterLowerLimitHoldsAtOnce(t *testing.T) {
	l := NewRateLimiter(0)
	l.SetLimit(1 << 20)
	took := pump(t, l, 512*1024, 16*1024)
	if took < 400*time.Millisecond {
		t.Fatalf("512 KiB at a new 1 MiB/s limit took only %v", took)
	}
}

func TestRateLimiterCancel(t *testing.T) {
	l := NewRateLimiter(32 * 1024)
	ctx, cancel := context.WithCancel(context.Background())
	errc := make(chan error, 1)
	go func() { errc <- l.Wait(ctx, 10<<20) }() // minutes at 32 KiB/s
	time.Sleep(50 * time.Millisecond)
	start := time.Now()
	cancel()
	select {
	case err := <-errc:
		if err != context.Canceled {
			t.Fatalf("Wait after cancel = %v, want context.Canceled", err)
		}
		if took := time.Since(start); took > 200*time.Millisecond {
			t.Fatalf("cancel took %v to end the wait", took)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Wait did not return after its context was cancelled")
	}
}

func TestSpeedLimitPackageBudgets(t *testing.T) {
	t.Cleanup(func() {
		for k := LimitKind(0); k < limitKinds; k++ {
			SetSpeedLimit(k, 0)
		}
	})
	SetSpeedLimit(LimitUpload, 5_000_000)
	SetSpeedLimit(LimitDownload, 6_000_000)
	SetSpeedLimit(LimitDiskRead, -1)
	if SpeedLimit(LimitUpload) != 5_000_000 || SpeedLimit(LimitDownload) != 6_000_000 || SpeedLimit(LimitDiskRead) != 0 {
		t.Fatalf("limits = %d %d %d", SpeedLimit(LimitUpload), SpeedLimit(LimitDownload), SpeedLimit(LimitDiskRead))
	}
	if SpeedLimit(limitKinds) != 0 || WaitSpeedLimit(context.Background(), limitKinds, 10) != nil {
		t.Fatal("an unknown kind must be unlimited")
	}
}

func TestLimitedConnChargesBothWays(t *testing.T) {
	t.Cleanup(func() {
		SetSpeedLimit(LimitUpload, 0)
		SetSpeedLimit(LimitDownload, 0)
	})
	a, b := net.Pipe()
	defer b.Close()
	c := newLimitedConn(a)
	defer c.Close()

	payload := make([]byte, 256*1024)
	go func() { _, _ = io.Copy(io.Discard, b) }()
	SetSpeedLimit(LimitUpload, 512*1024)
	start := time.Now()
	if _, err := c.Write(payload); err != nil {
		t.Fatal(err)
	}
	if took := time.Since(start); took < 350*time.Millisecond {
		t.Fatalf("256 KiB written at 512 KiB/s in %v", took)
	}

	// Close ends a long write wait at once.
	SetSpeedLimit(LimitUpload, 16*1024)
	errc := make(chan error, 1)
	go func() { _, err := c.Write(make([]byte, 4<<20)); errc <- err }()
	time.Sleep(50 * time.Millisecond)
	c.Close()
	select {
	case err := <-errc:
		if err == nil {
			t.Fatal("write on a closed connection succeeded")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Close did not end the write's wait")
	}
}

func TestReadFileLimited(t *testing.T) {
	t.Cleanup(func() { SetSpeedLimit(LimitDiskRead, 0) })
	path := filepath.Join(t.TempDir(), "f")
	want := bytes.Repeat([]byte("0123456789abcdef"), 40000) // 640000 bytes
	if err := os.WriteFile(path, want, 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := readFileLimited(context.Background(), path)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("unlimited: %d bytes, %v", len(got), err)
	}
	SetSpeedLimit(LimitDiskRead, 1<<20)
	start := time.Now()
	got, err = readFileLimited(context.Background(), path)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("limited: %d bytes, %v", len(got), err)
	}
	if took := time.Since(start); took < 450*time.Millisecond {
		t.Fatalf("640000 bytes read at 1 MiB/s in %v", took)
	}
}

func TestLimitReaderAtChargesDiskRead(t *testing.T) {
	t.Cleanup(func() { SetSpeedLimit(LimitDiskRead, 0) })
	SetSpeedLimit(LimitDiskRead, 1<<20)
	r := LimitReaderAt(context.Background(), LimitDiskRead, bytes.NewReader(make([]byte, 512*1024)))
	buf := make([]byte, 64*1024)
	start := time.Now()
	for off := int64(0); off < 512*1024; off += int64(len(buf)) {
		if _, err := r.ReadAt(buf, off); err != nil {
			t.Fatal(err)
		}
	}
	if took := time.Since(start); took < 350*time.Millisecond {
		t.Fatalf("512 KiB read at 1 MiB/s in %v", took)
	}
}

func TestOpenSourceChargesDiskRead(t *testing.T) {
	t.Cleanup(func() { SetSpeedLimit(LimitDiskRead, 0) })
	path := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(path, make([]byte, 512*1024), 0o600); err != nil {
		t.Fatal(err)
	}
	a := &PXARArchive{Ctx: context.Background()}
	f, err := a.openSource(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	SetSpeedLimit(LimitDiskRead, 1<<20)
	start := time.Now()
	if _, err := io.Copy(io.Discard, f); err != nil {
		t.Fatal(err)
	}
	if took := time.Since(start); took < 350*time.Millisecond {
		t.Fatalf("512 KiB read at 1 MiB/s in %v", took)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("Seek through the limited handle: %v", err)
	}
}
