//go:build linux && !service

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseSystemctlShow(t *testing.T) {
	out := "LoadState=loaded\nActiveState=active\nUnitFileState=enabled\n" +
		"FragmentPath=/usr/lib/systemd/system/salvanza.service\n" +
		"ExecStart={ path=/usr/libexec/salvanza/salvanza-svc ; argv[]=/usr/libexec/salvanza/salvanza-svc ; ignore_errors=no ; start_time=[n/a] ; stop_time=[n/a] ; pid=0 ; code=(null) ; status=0/0 }\n"
	u := parseSystemctlShow(out)
	if !u.installed() || u.state() != "running" || u.startType() != "boot" {
		t.Fatalf("got %+v state %s start %s", u, u.state(), u.startType())
	}
	if u.ExecPath != packagedServiceExe {
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

// The packaged app's service program is the one in /usr/libexec; a copy of the
// app run from a folder uses the one beside it.
func TestServiceExeFor(t *testing.T) {
	for _, c := range []struct{ app, want string }{
		{"/usr/lib/salvanza/salvanza-gui", "/usr/libexec/salvanza/salvanza-svc"},
		{"/opt/pbc/salvanza-gui", "/opt/pbc/salvanza-svc"},
		{"/home/u/build/bin/salvanza-gui", "/home/u/build/bin/salvanza-svc"},
	} {
		if got := serviceExeFor(c.app); got != c.want {
			t.Errorf("serviceExeFor(%q) = %q, want %q", c.app, got, c.want)
		}
	}
}

// Installing copies the user's settings into an empty shared folder, and
// never over settings already there.
func TestMoveSettingsToSharedCopiesOnce(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	src := filepath.Join(home, ".salvanza")
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
	lib := t.TempDir()
	shared := filepath.Join(lib, "salvanza")
	old, oldLegacy, oldPbsgo := sharedConfigDir, legacyStateDir, pbsgoStateDir
	// No earlier install here: the real /var/lib folders stay out of it.
	sharedConfigDir, legacyStateDir, pbsgoStateDir = shared, filepath.Join(lib, "proxmoxbackupclient"), filepath.Join(lib, "pbsgo")
	t.Cleanup(func() { sharedConfigDir, legacyStateDir, pbsgoStateDir = old, oldLegacy, oldPbsgo })

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
