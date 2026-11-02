//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func withSharedDir(t *testing.T, dir string) {
	t.Helper()
	old := sharedConfigDir
	sharedConfigDir = dir
	t.Cleanup(func() { sharedConfigDir = old })
}

// The app keeps its files in the user's folder whether or not the service's
// folder exists (it reaches the service's settings through the API); the
// token path is in the service's folder; the Linux command line, run as root
// on the service's Backup Sets, uses the service's folder.
func TestConfigDirChoice(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	shared := filepath.Join(t.TempDir(), "proxmoxbackupclient")
	withSharedDir(t, shared)
	if err := os.MkdirAll(shared, 0o700); err != nil {
		t.Fatal(err)
	}

	dir, err := getConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, ".proxmox-backup-guardian"); dir != want {
		t.Fatalf("app folder %s, want %s", dir, want)
	}
	if p := getAPITokenPath(); p != filepath.Join(shared, "api-token") {
		t.Fatalf("token path %s, want the service's folder", p)
	}

	useServiceStateDir.Store(true)
	t.Cleanup(func() { useServiceStateDir.Store(false) })
	if p, _ := getScheduledJobsPath(); p != filepath.Join(shared, "scheduled_jobs.json") {
		t.Fatalf("command line jobs path %s", p)
	}

	// Settings files are written owner-only everywhere.
	if err := atomicWriteFile(filepath.Join(shared, "config.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(shared, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("config.json mode %o, want 600", got)
	}
}

// A user's own folder is made private; hardening the service's folder as
// anyone but root changes nothing.
func TestHardenConfigDirUnix(t *testing.T) {
	own := t.TempDir()
	if err := os.Chmod(own, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := hardenConfigDir(own); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(own); info.Mode().Perm() != 0o700 {
		t.Fatalf("own folder mode %o", info.Mode().Perm())
	}
	if os.Geteuid() == 0 {
		t.Skip("the service-folder case is for a non-root run")
	}
	shared := t.TempDir()
	withSharedDir(t, shared)
	if err := os.Chmod(shared, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := hardenConfigDir(shared); err != nil {
		t.Fatal(err)
	}
	if info, _ := os.Stat(shared); info.Mode().Perm() != 0o755 {
		t.Fatalf("non-root changed the service's folder: %o", info.Mode().Perm())
	}
}

// A computer that had the earlier pbsgo package keeps its service's settings:
// they move to the shared folder when it is not there yet, and never over
// settings already in it.
func TestAdoptPbsgoState(t *testing.T) {
	root := t.TempDir()
	shared := filepath.Join(root, "proxmoxbackupclient")
	withSharedDir(t, shared)
	old := pbsgoStateDir
	pbsgoStateDir = filepath.Join(root, "pbsgo")
	t.Cleanup(func() { pbsgoStateDir = old })

	// No pbsgo settings: nothing happens.
	if err := adoptPbsgoState(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(shared); !os.IsNotExist(err) {
		t.Fatalf("shared folder made without pbsgo settings: %v", err)
	}

	if err := os.MkdirAll(pbsgoStateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"config.json", "scheduled_jobs.json"} {
		if err := os.WriteFile(filepath.Join(pbsgoStateDir, n), []byte(`{}`), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := adoptPbsgoState(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(shared, "scheduled_jobs.json")); err != nil {
		t.Fatalf("pbsgo settings not moved: %v", err)
	}
	if _, err := os.Stat(pbsgoStateDir); !os.IsNotExist(err) {
		t.Fatalf("pbsgo folder still there after the move: %v", err)
	}

	// With settings in the shared folder, another pbsgo folder is left alone.
	if err := os.MkdirAll(pbsgoStateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pbsgoStateDir, "config.json"), []byte(`{"x":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := adoptPbsgoState(); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(filepath.Join(shared, "config.json")); string(data) != `{}` {
		t.Errorf("shared settings replaced: %s", data)
	}
}
