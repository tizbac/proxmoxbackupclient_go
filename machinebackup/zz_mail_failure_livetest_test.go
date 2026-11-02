package main

import (
	"clientcommon"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"pbscommon"
	"strings"
	"testing"
)

// TestMain runs main() itself when a test starts this test binary again with
// MACHINEBACKUP_RUN_MAIN set, so a test can run the program as a whole.
func TestMain(m *testing.M) {
	if os.Getenv("MACHINEBACKUP_RUN_MAIN") == "1" {
		main()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// liveComment returns the PBS comment of the newest snapshot of backupID.
func liveComment(t *testing.T, backupID string) string {
	t.Helper()
	c := &pbscommon.PBSClient{
		BaseURL:         os.Getenv("PBS_LIVE_URL"),
		CertFingerPrint: os.Getenv("PBS_LIVE_FINGERPRINT"),
		AuthID:          os.Getenv("PBS_LIVE_AUTHID"),
		Secret:          os.Getenv("PBS_LIVE_SECRET"),
		Datastore:       os.Getenv("PBS_LIVE_DATASTORE"),
	}
	snaps, err := c.ListSnapshots()
	if err != nil {
		t.Fatalf("list snapshots: %v", err)
	}
	for _, s := range snaps { // newest first
		if s.BackupID == backupID {
			c.Manifest = s
			comment, err := c.GetComment()
			if err != nil {
				t.Fatalf("read the comment of %s/%s/%d: %v", s.BackupType, s.BackupID, s.BackupTime, err)
			}
			return comment
		}
	}
	t.Fatalf("no snapshot of %s on the server", backupID)
	return ""
}

// A backup that worked exits 0 even when its notification e-mail cannot be
// sent: the mail problem goes to stderr and into the snapshot's PBS comment.
// It backs up a small file as the device (no administrator rights needed) and
// the mail server is a port with nothing listening on it. Needs a PBS:
// PBS_LIVE_URL, PBS_LIVE_AUTHID, PBS_LIVE_SECRET, PBS_LIVE_FINGERPRINT and
// PBS_LIVE_DATASTORE.
func TestMailFailureKeepsBackupExitCode(t *testing.T) {
	url := os.Getenv("PBS_LIVE_URL")
	if url == "" {
		t.Skip("set PBS_LIVE_URL, PBS_LIVE_AUTHID, PBS_LIVE_SECRET, PBS_LIVE_FINGERPRINT, PBS_LIVE_DATASTORE")
	}
	dev := filepath.Join(t.TempDir(), "disk.img")
	if err := os.WriteFile(dev, []byte(strings.Repeat("machine backup mail test\n", 400000)), 0o600); err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	closedPort := fmt.Sprint(ln.Addr().(*net.TCPAddr).Port)
	ln.Close()

	const backupID = "machine-mail-failure-test"
	cmd := exec.Command(os.Args[0],
		"-baseurl", url,
		"-certfingerprint", os.Getenv("PBS_LIVE_FINGERPRINT"),
		"-authid", os.Getenv("PBS_LIVE_AUTHID"),
		"-secret", os.Getenv("PBS_LIVE_SECRET"),
		"-datastore", os.Getenv("PBS_LIVE_DATASTORE"),
		"-backupdev", dev,
		"-backup-id", backupID,
		"-mail-host", "127.0.0.1",
		"-mail-port", closedPort,
		"-mail-username", "user",
		"-mail-password", "password",
		"-mail-from", "backup@example.com",
		"-mail-to", "admin@example.com",
	)
	cmd.Env = append(os.Environ(), "MACHINEBACKUP_RUN_MAIN=1")
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
	if comment := liveComment(t, backupID); comment != clientcommon.MailFailedNote {
		t.Errorf("snapshot comment %q, want %q", comment, clientcommon.MailFailedNote)
	}
	t.Logf("%s", out)
}
