//go:build windows

package main

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

// A 1.2.0-rc state file, locked to SYSTEM with inheritance removed as its
// service left it, goes back to inheriting the folder's permissions; a file
// with another name keeps its own protected DACL (key files do on purpose).
func TestInheritUpstreamStateFiles(t *testing.T) {
	dir := t.TempDir()
	system, err := windows.CreateWellKnownSid(windows.WinLocalSystemSid)
	if err != nil {
		t.Fatal(err)
	}
	acl, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{aclEntry(system, fileAllAccess, windows.NO_INHERITANCE)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	lock := func(name string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(`[]`), 0o600); err != nil {
			t.Fatal(err)
		}
		// The test runs elevated and owns the file, so it may still change
		// the DACL afterwards, as the service (SYSTEM) may on a real install.
		if err := windows.SetNamedSecurityInfo(p, windows.SE_FILE_OBJECT,
			windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, acl, nil); err != nil {
			t.Fatal(err)
		}
		return p
	}
	jobs := lock("scheduled_jobs.json")
	key := lock("my-key.json")

	if n := inheritUpstreamStateFiles(dir); n != 1 {
		t.Errorf("reset %d file(s), want 1", n)
	}
	protected := func(p string) bool {
		sd, err := windows.GetNamedSecurityInfo(p, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
		if err != nil {
			t.Fatal(err)
		}
		c, _, err := sd.Control()
		if err != nil {
			t.Fatal(err)
		}
		return c&windows.SE_DACL_PROTECTED != 0
	}
	if protected(jobs) {
		t.Error("scheduled_jobs.json is still protected")
	}
	if _, err := os.ReadFile(jobs); err != nil {
		t.Errorf("scheduled_jobs.json not readable after the reset: %v", err)
	}
	if !protected(key) {
		t.Error("a file not on the 1.2.0-rc list lost its protected DACL")
	}
	if n := inheritUpstreamStateFiles(dir); n != 0 {
		t.Errorf("second run reset %d file(s), want 0", n)
	}
}

// Settings of the 1.2.0-rc app run without its service come across from the
// user's profile once, and never over settings already in ProgramData.
func TestImportUpstreamUserSettings(t *testing.T) {
	home := t.TempDir()
	programData := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv("ProgramData", programData)
	old := filepath.Join(home, ".proxmox-backup-guardian")
	if err := os.MkdirAll(old, 0o700); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"config.json":         `{"pbs_servers":{}}`,
		"scheduled_jobs.json": `[{"id":"w","name":"Weekly","frequency":"weekly","weekdays":[2]}]`,
		"job_history.json":    `[]`,
	} {
		if err := os.WriteFile(filepath.Join(old, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	importUpstreamUserSettings()
	dst := filepath.Join(programData, "ProxmoxBackupClient")
	for _, name := range []string{"config.json", "scheduled_jobs.json", "job_history.json"} {
		if _, err := os.Stat(filepath.Join(dst, name)); err != nil {
			t.Errorf("%s not copied: %v", name, err)
		}
		if _, err := os.Stat(filepath.Join(old, name)); err != nil {
			t.Errorf("original %s gone: %v", name, err)
		}
	}

	// Settings already in ProgramData are never replaced.
	if err := os.WriteFile(filepath.Join(dst, "scheduled_jobs.json"), []byte(`[]`), 0o600); err != nil {
		t.Fatal(err)
	}
	importUpstreamUserSettings()
	if data, _ := os.ReadFile(filepath.Join(dst, "scheduled_jobs.json")); string(data) != `[]` {
		t.Errorf("existing settings were replaced: %s", data)
	}
}
