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

// The app uses the user's folder until the shared one exists, then the
// shared one; settings files in it become group-readable, nowhere else.
func TestSharedConfigDirChoice(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	shared := filepath.Join(t.TempDir(), "proxmoxbackupclient")
	withSharedDir(t, shared)

	dir, err := getConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, ".proxmox-backup-guardian"); dir != want {
		t.Fatalf("before the service: %s, want %s", dir, want)
	}
	if err := os.MkdirAll(shared, 0o750); err != nil {
		t.Fatal(err)
	}
	if dir, _ = getConfigDir(); dir != shared {
		t.Fatalf("after the service: %s, want %s", dir, shared)
	}
	if p, _ := getScheduledJobsPath(); p != filepath.Join(shared, "scheduled_jobs.json") {
		t.Fatalf("jobs path %s", p)
	}

	if err := atomicWriteFile(filepath.Join(shared, "config.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := atomicWriteFile(filepath.Join(home, "x.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	for p, want := range map[string]os.FileMode{
		filepath.Join(shared, "config.json"): 0o640,
		filepath.Join(home, "x.json"):        0o600,
	} {
		info, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != want {
			t.Errorf("%s: mode %o, want %o", p, got, want)
		}
	}
}

func TestSharedFileMode(t *testing.T) {
	withSharedDir(t, "/var/lib/proxmoxbackupclient")
	cases := []struct {
		path string
		in   os.FileMode
		want os.FileMode
	}{
		{"/var/lib/proxmoxbackupclient/api-token", 0o600, 0o640},
		{"/var/lib/proxmoxbackupclient/keys/encryption-key.json", 0o600, 0o640},
		{"/var/lib/proxmoxbackupclient/restore_cache/a.gz", 0o644, 0o640},
		{"/var/lib/proxmoxbackupclient-other/x", 0o600, 0o600},
		{"/home/u/.proxmox-backup-guardian/config.json", 0o600, 0o600},
	}
	for _, c := range cases {
		if got := sharedFileMode(c.path, c.in); got != c.want {
			t.Errorf("%s %o: got %o, want %o", c.path, c.in, got, c.want)
		}
	}
}

// A user's own folder is made private; hardening the shared folder as
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
		t.Skip("shared-folder case is for a non-root run")
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
		t.Fatalf("non-root changed the shared folder: %o", info.Mode().Perm())
	}
}
