package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The devices to back up can come from the -config file alone. The run goes
// as far as the backup itself, which fails on the non-numeric VM ID before
// any connection to PBS; without the file's devices it stopped earlier, at
// "Missing or invalid options".
func TestConfigFileBackupDevices(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), "config.json")
	err := os.WriteFile(cfg, []byte(`{
		"baseurl": "https://127.0.0.1:1",
		"certfingerprint": "00:11:22",
		"authid": "test@pbs!test",
		"secret": "secret",
		"datastore": "store",
		"backupdev": ["nosuchdisk.img"],
		"backuptype": "vm",
		"backup-id": "not-a-number"
	}`), 0o600)
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(os.Args[0], "-config", cfg)
	cmd.Env = append(os.Environ(), "MACHINEBACKUP_RUN_MAIN=1")
	out, err := cmd.CombinedOutput()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected a non-zero exit, got %v\n%s", err, out)
	}
	if exitErr.ExitCode() == 2 {
		t.Skipf("another backup job holds the process lock on this machine\n%s", out)
	}
	if strings.Contains(string(out), "Missing or invalid options") {
		t.Fatalf("the config file's backupdev was not used\n%s", out)
	}
	if !strings.Contains(string(out), "needs a numeric VM ID") {
		t.Fatalf("expected the backup to fail on the VM ID\n%s", out)
	}
}
