package machinebackuplib

import (
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestStoppedBackupReleasesGroup backs up a small file, stops the backup
// after its first block, then backs up the same group again in the same
// process. Before Backup closed its session on every return, the stopped
// run left PBS's backup task running with the group locked, and the second
// run failed until the process exited.
//
// Needs a PBS: PBS_LIVE_URL, PBS_LIVE_AUTHID, PBS_LIVE_SECRET,
// PBS_LIVE_FINGERPRINT, PBS_LIVE_DATASTORE (and optionally PBS_LIVE_NS).
func TestStoppedBackupReleasesGroup(t *testing.T) {
	url := os.Getenv("PBS_LIVE_URL")
	if url == "" {
		t.Skip("set PBS_LIVE_URL, PBS_LIVE_AUTHID, PBS_LIVE_SECRET, PBS_LIVE_FINGERPRINT, PBS_LIVE_DATASTORE")
	}
	dev := filepath.Join(t.TempDir(), "disk.img")
	data := make([]byte, 12<<20)
	if _, err := rand.Read(data); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dev, data, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{
		BaseURL:         url,
		AuthID:          os.Getenv("PBS_LIVE_AUTHID"),
		Secret:          os.Getenv("PBS_LIVE_SECRET"),
		CertFingerprint: os.Getenv("PBS_LIVE_FINGERPRINT"),
		Datastore:       os.Getenv("PBS_LIVE_DATASTORE"),
		Namespace:       os.Getenv("PBS_LIVE_NS"),
		BackupID:        "machine-session-close-test",
		BackupDevices:   []string{dev},
		BackupType:      "host",
	}

	stopAtFirstBlock := func(fraction float64, _ string) bool { return fraction > 0 }
	if _, err := Backup(cfg, stopAtFirstBlock); !errors.Is(err, ErrCancelled) {
		t.Fatalf("first backup: want ErrCancelled, got %v", err)
	}
	if _, err := Backup(cfg, nil); err != nil {
		t.Fatalf("second backup of the same group after a stopped one: %v", err)
	}
}
