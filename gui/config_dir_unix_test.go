//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func withSharedDir(t *testing.T, dir string) {
	t.Helper()
	old := sharedConfigDir
	sharedConfigDir = dir
	t.Cleanup(func() { sharedConfigDir = old })
}

// isolateServiceDir gives one test a service settings folder of its own,
// inside dir. Every test otherwise shares the one TestMain set, and the
// service program keeps all its files there (the restore cache among them):
// built with -tags service, one test found what another had saved.
func isolateServiceDir(t *testing.T, dir string) {
	t.Helper()
	withSharedDir(t, filepath.Join(dir, "shared"))
}

// The app keeps its files in the user's folder whether or not the service's
// folder exists (it reaches the service's settings through the API); the
// token path is in the service's folder; the Linux command line, run as root
// on the service's Backup Sets, uses the service's folder.
func TestConfigDirChoice(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	shared := filepath.Join(t.TempDir(), "salvanza")
	withSharedDir(t, shared)
	if err := os.MkdirAll(shared, 0o700); err != nil {
		t.Fatal(err)
	}

	dir, err := getConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	// The service program (built with -tags service) is the other way round:
	// its own folder is the service's.
	want := filepath.Join(home, ".salvanza")
	if serviceBuild {
		want = shared
	}
	if dir != want {
		t.Fatalf("settings folder %s, want %s", dir, want)
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

// A computer that had Proxmox Backup Client (before the rename) or the earlier
// pbsgo package keeps its service's settings: the newer of the two moves to
// the shared folder when it has none yet, and never over settings already in
// it.
func TestAdoptLinuxState(t *testing.T) {
	root := t.TempDir()
	shared := filepath.Join(root, "salvanza")
	withSharedDir(t, shared)
	oldLegacy, oldPbsgo := legacyStateDir, pbsgoStateDir
	legacyStateDir = filepath.Join(root, "proxmoxbackupclient")
	pbsgoStateDir = filepath.Join(root, "pbsgo")
	t.Cleanup(func() { legacyStateDir, pbsgoStateDir = oldLegacy, oldPbsgo })

	// No earlier settings: nothing happens.
	if err := adoptLinuxState(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(shared); !os.IsNotExist(err) {
		t.Fatalf("shared folder made without earlier settings: %v", err)
	}

	write := func(dir, name, content string, age time.Duration) {
		t.Helper()
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		when := time.Now().Add(-age)
		if err := os.Chtimes(p, when, when); err != nil {
			t.Fatal(err)
		}
	}
	// Both earlier packages left settings; Proxmox Backup Client's are newer.
	write(pbsgoStateDir, "config.json", `{"from":"pbsgo"}`, 48*time.Hour)
	write(legacyStateDir, "config.json", `{"from":"pbc"}`, time.Hour)
	write(legacyStateDir, "scheduled_jobs.json", `[]`, time.Hour)
	if err := adoptLinuxState(); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(filepath.Join(shared, "config.json")); string(data) != `{"from":"pbc"}` {
		t.Fatalf("took over %s, want the newer settings", data)
	}
	if _, err := os.Stat(filepath.Join(shared, "scheduled_jobs.json")); err != nil {
		t.Fatalf("Backup Sets not moved: %v", err)
	}
	if _, err := os.Stat(legacyStateDir); !os.IsNotExist(err) {
		t.Fatalf("earlier folder still there after the move: %v", err)
	}
	if copies, _ := filepath.Glob(filepath.Join(shared, "pre-salvanza-*", "config.json")); len(copies) != 1 {
		t.Errorf("no copy of the settings kept for a rollback: %v", copies)
	}

	// With settings in the shared folder, the other earlier folder is left alone.
	if err := adoptLinuxState(); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(filepath.Join(shared, "config.json")); string(data) != `{"from":"pbc"}` {
		t.Errorf("shared settings replaced: %s", data)
	}
	if _, err := os.Stat(filepath.Join(pbsgoStateDir, "config.json")); err != nil {
		t.Errorf("pbsgo settings touched: %v", err)
	}
}
