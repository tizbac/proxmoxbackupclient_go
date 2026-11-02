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

// After an upgrade from 1.2.0-rc, its config.json disk table reaches the
// machine sets when they are loaded, and a save of config.json keeps it.
func TestUpstreamDiskPinsAfterUpgrade(t *testing.T) {
	t.Setenv("ProgramData", t.TempDir())
	dir, err := getConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"config.json":         `{"pbs_servers":{},"pinned_disks":{"serial:S4XNX0A123":"\\\\.\\PhysicalDrive0"}}`,
		"scheduled_jobs.json": `[{"id":"m","name":"Machine","scheduleTime":"02:00","backupType":"machine","driveLetters":["\\\\.\\PhysicalDrive0"],"backupDirs":null,"excludeList":null,"enabled":true}]`,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	a := &App{config: LoadConfig()}
	jobs, err := a.GetScheduledJobs()
	if err != nil || len(jobs) != 1 {
		t.Fatalf("GetScheduledJobs = %v, %v", jobs, err)
	}
	if len(jobs[0].DiskIDs) != 1 || jobs[0].DiskIDs[0] != "serial:S4XNX0A123" {
		t.Errorf("DiskIDs = %q, want [serial:S4XNX0A123]", jobs[0].DiskIDs)
	}

	if err := a.config.Save(); err != nil {
		t.Fatal(err)
	}
	if got := LoadConfig().UpstreamPinnedDisks["serial:S4XNX0A123"]; got != `\\.\PhysicalDrive0` {
		t.Errorf("pinned_disks after a save: %q", got)
	}
}
