//go:build windows
// +build windows

package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// A folder named by a network path, here this computer's own C$ share, with
// VSS on (the default): VSS cannot snapshot a share, so the run reads it
// live and says so, where it used to fail. Needs an administrator, the C$
// share and a PBS: PBS_LIVE_URL, PBS_LIVE_AUTHID, PBS_LIVE_SECRET,
// PBS_LIVE_FINGERPRINT and PBS_LIVE_DATASTORE.
func TestNetworkPathReadLiveLive(t *testing.T) {
	url := os.Getenv("PBS_LIVE_URL")
	if url == "" {
		t.Skip("set PBS_LIVE_URL, PBS_LIVE_AUTHID, PBS_LIVE_SECRET, PBS_LIVE_FINGERPRINT, PBS_LIVE_DATASTORE")
	}
	dir, err := filepath.Abs(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("network path test\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(strings.ToUpper(dir), `C:\`) {
		t.Skipf("temp folder %s is not on C:", dir)
	}
	share := `\\localhost\C$\` + dir[3:]
	if _, err := os.Stat(share); err != nil {
		t.Skipf("%s not reachable: %v", share, err)
	}
	cmd := exec.Command(os.Args[0],
		"-baseurl", url,
		"-certfingerprint", os.Getenv("PBS_LIVE_FINGERPRINT"),
		"-authid", os.Getenv("PBS_LIVE_AUTHID"),
		"-secret", os.Getenv("PBS_LIVE_SECRET"),
		"-datastore", os.Getenv("PBS_LIVE_DATASTORE"),
		"-backupdir", share,
		"-backup-id", "dir-networkpath-test",
	)
	cmd.Env = append(os.Environ(), "DIRECTORYBACKUP_RUN_MAIN=1")
	out, err := cmd.CombinedOutput()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 2 {
		t.Skipf("another backup job holds the process lock on this machine\n%s", out)
	}
	if err != nil {
		t.Fatalf("exit %v, want 0\n%s", err, out)
	}
	if !strings.Contains(string(out), "Not taking a snapshot of "+share+": a network path") {
		t.Errorf("no word of reading the share live\n%s", out)
	}
	if strings.Contains(string(out), "Creating VSS Snapshot") {
		t.Errorf("VSS was asked for a share\n%s", out)
	}
}
