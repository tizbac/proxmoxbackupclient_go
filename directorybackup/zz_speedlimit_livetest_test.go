package directorybackup

import (
	"crypto/rand"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// -disk-read-limit and -upload-limit slow a real backup to their rate. Each
// run backs up 6 MB of fresh random data, which cannot be deduplicated or
// compressed, so at 2 MB/s it needs about 3 s (2.6 s at least, after the
// first fifth of a second's burst); unlimited on a LAN it takes well under
// one. Needs a PBS: PBS_LIVE_URL, PBS_LIVE_AUTHID, PBS_LIVE_SECRET,
// PBS_LIVE_FINGERPRINT and PBS_LIVE_DATASTORE.
func TestSpeedLimitsLive(t *testing.T) {
	url := os.Getenv("PBS_LIVE_URL")
	if url == "" {
		t.Skip("set PBS_LIVE_URL, PBS_LIVE_AUTHID, PBS_LIVE_SECRET, PBS_LIVE_FINGERPRINT, PBS_LIVE_DATASTORE")
	}
	// Each run has a group of its own: PBS refuses a second snapshot of a
	// group in the same second, and an unlimited run takes less.
	run := func(t *testing.T, id string, limits ...string) (string, time.Duration) {
		dir := t.TempDir()
		data := make([]byte, 6*1000*1000)
		if _, err := rand.Read(data); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "random.bin"), data, 0o600); err != nil {
			t.Fatal(err)
		}
		args := append([]string{
			"-baseurl", url,
			"-certfingerprint", os.Getenv("PBS_LIVE_FINGERPRINT"),
			"-authid", os.Getenv("PBS_LIVE_AUTHID"),
			"-secret", os.Getenv("PBS_LIVE_SECRET"),
			"-datastore", os.Getenv("PBS_LIVE_DATASTORE"),
			"-backupdir", dir,
			"-backup-id", "dir-speedlimit-test-" + id,
		}, limits...)
		if runtime.GOOS != "windows" {
			args = append(args, "-novss")
		}
		cmd := exec.Command(os.Args[0], args...)
		cmd.Env = append(os.Environ(), "DIRECTORYBACKUP_RUN_MAIN=1")
		start := time.Now()
		out, err := cmd.CombinedOutput()
		took := time.Since(start)
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 2 {
			t.Skipf("another backup job holds the process lock on this machine\n%s", out)
		}
		if err != nil {
			t.Fatalf("exit %v\n%s", err, out)
		}
		return string(out), took
	}

	_, free := run(t, "free")
	t.Logf("no limit: %s", free.Round(time.Millisecond))
	for _, c := range []struct {
		id, flag, line string
	}{
		{"disk", "-disk-read-limit", "Speed limits: upload none, download none, disk read 2 MB/s"},
		{"upload", "-upload-limit", "Speed limits: upload 2 MB/s, download none, disk read none"},
	} {
		out, took := run(t, c.id, c.flag, "2")
		t.Logf("%s 2: %s", c.flag, took.Round(time.Millisecond))
		if !strings.Contains(out, c.line+"\n") {
			t.Errorf("%s: no %q in the output\n%s", c.flag, c.line, out)
		}
		if took < 2600*time.Millisecond {
			t.Errorf("%s 2: 6 MB took %s, want at least 2.6 s", c.flag, took)
		}
	}
}
