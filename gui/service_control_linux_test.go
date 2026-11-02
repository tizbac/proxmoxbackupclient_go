//go:build linux && !service

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseSystemctlShow(t *testing.T) {
	out := "LoadState=loaded\nActiveState=active\nUnitFileState=enabled\n" +
		"FragmentPath=/usr/lib/systemd/system/proxmoxbackupclient.service\n" +
		"ExecStart={ path=/usr/lib/proxmoxbackupclient/ProxmoxBackupClientSVC ; argv[]=/usr/lib/proxmoxbackupclient/ProxmoxBackupClientSVC ; ignore_errors=no ; start_time=[n/a] ; stop_time=[n/a] ; pid=0 ; code=(null) ; status=0/0 }\n"
	u := parseSystemctlShow(out)
	if !u.installed() || u.state() != "running" || u.startType() != "boot" {
		t.Fatalf("got %+v state %s start %s", u, u.state(), u.startType())
	}
	if u.ExecPath != "/usr/lib/proxmoxbackupclient/ProxmoxBackupClientSVC" {
		t.Fatalf("exec path %q", u.ExecPath)
	}

	for _, c := range []struct{ show, installed, state, start string }{
		{"LoadState=not-found\nActiveState=inactive\nUnitFileState=\n", "no", "stopped", ""},
		{"LoadState=loaded\nActiveState=failed\nUnitFileState=disabled\n", "yes", "stopped", "manual"},
		{"LoadState=loaded\nActiveState=activating\nUnitFileState=enabled\n", "yes", "starting", "boot"},
		{"LoadState=masked\nActiveState=inactive\nUnitFileState=masked\n", "no", "stopped", "disabled"},
	} {
		u := parseSystemctlShow(c.show)
		inst := map[bool]string{true: "yes", false: "no"}[u.installed()]
		if inst != c.installed || u.state() != c.state || u.startType() != c.start {
			t.Errorf("%q: installed %s state %s start %s", c.show, inst, u.state(), u.startType())
		}
	}
}

// Installing copies the user's settings into an empty shared folder, and
// never over settings already there.
func TestMoveSettingsToSharedCopiesOnce(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	src := filepath.Join(home, ".proxmox-backup-guardian")
	for name, body := range map[string]string{
		"config.json":                "{\"a\":1}",
		"scheduled_jobs.json":        "[]",
		"keys/encryption-key.json":   "k",
		"restore_cache/tree.json.gz": "c",
	} {
		p := filepath.Join(src, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	shared := filepath.Join(t.TempDir(), "shared")
	old := sharedConfigDir
	sharedConfigDir = shared
	t.Cleanup(func() { sharedConfigDir = old })

	if err := moveSettingsToShared(); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"config.json", "scheduled_jobs.json", "keys/encryption-key.json", "restore_cache/tree.json.gz"} {
		if _, err := os.Stat(filepath.Join(shared, name)); err != nil {
			t.Errorf("%s not copied: %v", name, err)
		}
	}
	// A second install must not overwrite what the shared folder holds.
	if err := os.WriteFile(filepath.Join(shared, "config.json"), []byte("{\"b\":2}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := moveSettingsToShared(); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(filepath.Join(shared, "config.json")); string(b) != "{\"b\":2}" {
		t.Fatalf("shared settings overwritten: %s", b)
	}
}
