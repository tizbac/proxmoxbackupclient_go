package main

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestMain runs main() itself when a test starts this test binary again with
// DIRECTORYBACKUP_RUN_MAIN set, so a test can run the program as a whole.
func TestMain(m *testing.M) {
	if os.Getenv("DIRECTORYBACKUP_RUN_MAIN") == "1" {
		main()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// A backup that worked exits 0 even when its notification e-mail cannot be
// sent: the mail problem goes to stderr only. The mail server is a port with
// nothing listening on it. Needs a PBS: PBS_LIVE_URL, PBS_LIVE_AUTHID,
// PBS_LIVE_SECRET, PBS_LIVE_FINGERPRINT and PBS_LIVE_DATASTORE.
func TestMailFailureKeepsBackupExitCode(t *testing.T) {
	url := os.Getenv("PBS_LIVE_URL")
	if url == "" {
		t.Skip("set PBS_LIVE_URL, PBS_LIVE_AUTHID, PBS_LIVE_SECRET, PBS_LIVE_FINGERPRINT, PBS_LIVE_DATASTORE")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("mail failure test\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	closedPort := fmt.Sprint(ln.Addr().(*net.TCPAddr).Port)
	ln.Close()

	cmd := exec.Command(os.Args[0],
		"-baseurl", url,
		"-certfingerprint", os.Getenv("PBS_LIVE_FINGERPRINT"),
		"-authid", os.Getenv("PBS_LIVE_AUTHID"),
		"-secret", os.Getenv("PBS_LIVE_SECRET"),
		"-datastore", os.Getenv("PBS_LIVE_DATASTORE"),
		"-backupdir", dir,
		"-backup-id", "dir-mail-failure-test",
		"-novss",
		"-mail-host", "127.0.0.1",
		"-mail-port", closedPort,
		"-mail-username", "user",
		"-mail-password", "password",
		"-mail-from", "backup@example.com",
		"-mail-to", "admin@example.com",
	)
	cmd.Env = append(os.Environ(), "DIRECTORYBACKUP_RUN_MAIN=1")
	out, err := cmd.CombinedOutput()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 2 {
		t.Skipf("another backup job holds the process lock on this machine\n%s", out)
	}
	if err != nil {
		t.Fatalf("exit %v, want 0 (the backup worked; only the mail failed)\n%s", err, out)
	}
	if !strings.Contains(string(out), "Cannot connect to the mail server") {
		t.Errorf("no mail error in the output\n%s", out)
	}
	t.Logf("%s", out)
}
