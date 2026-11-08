package machinebackup

import (
	"crypto/rand"
	"errors"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// cuttingProxy forwards TCP connections to target and cuts the first one
// that has sent more than cutAfter bytes, both ways at once, as a dropped
// link would. It returns its address and the number of connections made.
// (The same helper is in directorybackup's tests.)
func cuttingProxy(t *testing.T, target string, cutAfter int64) (string, *atomic.Int32) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	var conns atomic.Int32
	var cut atomic.Bool
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			conns.Add(1)
			go func() {
				s, err := net.Dial("tcp", target)
				if err != nil {
					_ = c.Close()
					return
				}
				var once sync.Once
				closeBoth := func() { once.Do(func() { _ = c.Close(); _ = s.Close() }) }
				go func() { _, _ = io.Copy(c, s); closeBoth() }()
				buf := make([]byte, 32*1024)
				var sent int64
				for {
					n, rerr := c.Read(buf)
					if n > 0 {
						if _, werr := s.Write(buf[:n]); werr != nil {
							break
						}
						sent += int64(n)
						if sent > cutAfter && cut.CompareAndSwap(false, true) {
							break
						}
					}
					if rerr != nil {
						break
					}
				}
				closeBoth()
			}()
		}
	}()
	return ln.Addr().String(), &conns
}

// A connection to PBS lost part way through a machine backup: the run says
// so, starts again with a new session and succeeds, as the app does. The
// waits are cut to 3 s here. The device is a 16 MB file of random data (no
// administrator rights needed). Needs a PBS: PBS_LIVE_URL, PBS_LIVE_AUTHID,
// PBS_LIVE_SECRET, PBS_LIVE_FINGERPRINT and PBS_LIVE_DATASTORE.
func TestLostConnectionRetryLive(t *testing.T) {
	pbsURL := os.Getenv("PBS_LIVE_URL")
	if pbsURL == "" {
		t.Skip("set PBS_LIVE_URL, PBS_LIVE_AUTHID, PBS_LIVE_SECRET, PBS_LIVE_FINGERPRINT, PBS_LIVE_DATASTORE")
	}
	u, err := url.Parse(pbsURL)
	if err != nil {
		t.Fatal(err)
	}
	proxy, conns := cuttingProxy(t, u.Host, 3*1000*1000)

	dev := filepath.Join(t.TempDir(), "disk.img")
	data := make([]byte, 16*1000*1000)
	if _, err := rand.Read(data); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dev, data, 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0],
		"-baseurl", "https://"+proxy,
		"-certfingerprint", os.Getenv("PBS_LIVE_FINGERPRINT"),
		"-authid", os.Getenv("PBS_LIVE_AUTHID"),
		"-secret", os.Getenv("PBS_LIVE_SECRET"),
		"-datastore", os.Getenv("PBS_LIVE_DATASTORE"),
		"-backupdev", dev,
		"-backup-id", "machine-reconnect-test",
	)
	cmd.Env = append(os.Environ(), "MACHINEBACKUP_RUN_MAIN=1", "MACHINEBACKUP_TEST_RECONNECT_WAIT=3s")
	out, err := cmd.CombinedOutput()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 2 {
		t.Skipf("another backup job holds the process lock on this machine\n%s", out)
	}
	text := string(out)
	if err != nil {
		t.Fatalf("exit %v, want 0 after the new attempt\n%s", err, text)
	}
	if !strings.Contains(text, "Connection to the server lost on attempt 1 of 5") ||
		!strings.Contains(text, "Starting again at") {
		t.Errorf("no word of the lost connection and the new attempt\n%s", text)
	}
	if n := conns.Load(); n < 2 {
		t.Errorf("%d connection(s) to PBS, want a second one for the new attempt", n)
	}
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, "lost") || strings.Contains(line, "again") || strings.Contains(line, "Done in") {
			t.Log(line)
		}
	}
}
