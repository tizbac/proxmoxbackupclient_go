package machinebackup

import (
	"crypto/rand"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The config file's "disk-read-limit" slows a machine backup to its rate,
// and a flag overrides it. The device is a file of 8 MB of fresh random data
// (no administrator rights needed): at 2 MB/s it needs about 4 s (3.6 s at
// least, after the first burst). Needs a PBS: PBS_LIVE_URL, PBS_LIVE_AUTHID,
// PBS_LIVE_SECRET, PBS_LIVE_FINGERPRINT and PBS_LIVE_DATASTORE.
func TestSpeedLimitsLive(t *testing.T) {
	url := os.Getenv("PBS_LIVE_URL")
	if url == "" {
		t.Skip("set PBS_LIVE_URL, PBS_LIVE_AUTHID, PBS_LIVE_SECRET, PBS_LIVE_FINGERPRINT, PBS_LIVE_DATASTORE")
	}
	cfg := filepath.Join(t.TempDir(), "cfg.json")
	if err := os.WriteFile(cfg, []byte(`{"disk-read-limit": 2, "upload-limit": 50}`), 0o600); err != nil {
		t.Fatal(err)
	}
	// Each run has a group of its own: PBS refuses a second snapshot of a
	// group in the same second.
	run := func(t *testing.T, id string, extra ...string) (string, time.Duration) {
		dev := filepath.Join(t.TempDir(), "disk.img")
		data := make([]byte, 8*1000*1000)
		if _, err := rand.Read(data); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(dev, data, 0o600); err != nil {
			t.Fatal(err)
		}
		args := append([]string{
			"-config", cfg,
			"-baseurl", url,
			"-certfingerprint", os.Getenv("PBS_LIVE_FINGERPRINT"),
			"-authid", os.Getenv("PBS_LIVE_AUTHID"),
			"-secret", os.Getenv("PBS_LIVE_SECRET"),
			"-datastore", os.Getenv("PBS_LIVE_DATASTORE"),
			"-backupdev", dev,
			"-backup-id", "machine-speedlimit-test-" + id,
		}, extra...)
		cmd := exec.Command(os.Args[0], args...)
		cmd.Env = append(os.Environ(), "MACHINEBACKUP_RUN_MAIN=1")
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

	out, took := run(t, "file")
	t.Logf("config file, disk read 2: %s", took.Round(time.Millisecond))
	if !strings.Contains(out, "Speed limits: upload 50 MB/s, download none, disk read 2 MB/s\n") {
		t.Errorf("limits from the config file not applied\n%s", out)
	}
	if took < 3600*time.Millisecond {
		t.Errorf("8 MB at 2 MB/s took %s, want at least 3.6 s", took)
	}

	out, took = run(t, "flag", "-disk-read-limit", "0")
	t.Logf("flag -disk-read-limit 0 over the file: %s", took.Round(time.Millisecond))
	if !strings.Contains(out, "Speed limits: upload 50 MB/s, download none, disk read none\n") {
		t.Errorf("the flag did not override the config file\n%s", out)
	}
	if took > 3*time.Second {
		t.Logf("note: unlimited run took %s", took)
	}
}
