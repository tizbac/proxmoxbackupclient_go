package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Quick backups of one group started back to back, as a script chaining
// them would: PBS refuses a second snapshot of a group in the same second,
// and each run waits for the next second instead of failing (before, four
// of five failed with "PBS authentication failed: HTTP 400"). Needs a PBS:
// PBS_LIVE_URL, PBS_LIVE_AUTHID, PBS_LIVE_SECRET, PBS_LIVE_FINGERPRINT and
// PBS_LIVE_DATASTORE.
func TestSameSecondBackToBackLive(t *testing.T) {
	url := os.Getenv("PBS_LIVE_URL")
	if url == "" {
		t.Skip("set PBS_LIVE_URL, PBS_LIVE_AUTHID, PBS_LIVE_SECRET, PBS_LIVE_FINGERPRINT, PBS_LIVE_DATASTORE")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("same second\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	waited := 0
	for i := 1; i <= 5; i++ {
		args := []string{
			"-baseurl", url,
			"-certfingerprint", os.Getenv("PBS_LIVE_FINGERPRINT"),
			"-authid", os.Getenv("PBS_LIVE_AUTHID"),
			"-secret", os.Getenv("PBS_LIVE_SECRET"),
			"-datastore", os.Getenv("PBS_LIVE_DATASTORE"),
			"-backupdir", dir,
			"-backup-id", "dir-samesecond-test",
		}
		if runtime.GOOS != "windows" {
			args = append(args, "-novss")
		}
		cmd := exec.Command(os.Args[0], args...)
		cmd.Env = append(os.Environ(), "DIRECTORYBACKUP_RUN_MAIN=1")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("run %d: exit %v\n%s", i, err, out)
		}
		if strings.Contains(string(out), "starting again in the next second") {
			waited++
		}
		if strings.Contains(string(out), "authentication failed") {
			t.Errorf("run %d: the refusal still reads as a login problem\n%s", i, out)
		}
	}
	t.Logf("5 runs, %d of them waited for the next second", waited)
}
