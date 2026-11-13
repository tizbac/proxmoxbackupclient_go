//go:build windows

package main

import (
	"os"
	"path/filepath"
	"testing"
	"unsafe"

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

// Nimbus Backup 0.4.1 could leave config.json with a protected DACL without
// any entry, so that nobody may read it. A folder taken over with such a file
// has it readable at once, and the copy kept for a rollback holds it.
func TestTakeoverUnlocksSettingsNobodyMayRead(t *testing.T) {
	programData := t.TempDir()
	earlier := filepath.Join(programData, legacyFolder)
	dir := filepath.Join(programData, folderName())
	if err := os.MkdirAll(earlier, 0o700); err != nil {
		t.Fatal(err)
	}
	locked := filepath.Join(earlier, "config.json")
	if err := os.WriteFile(locked, []byte(`{"pbs_servers":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	emptyBuf := [8]byte{2, 0, 8, 0, 0, 0, 0, 0} // ACL_REVISION, size 8, 0 ACEs
	empty := (*windows.ACL)(unsafe.Pointer(&emptyBuf[0]))
	// The test owns the file, so it may still change the DACL afterwards, as
	// the service (SYSTEM) may on a real install.
	if err := windows.SetNamedSecurityInfo(locked, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION, nil, nil, empty, nil); err != nil {
		t.Fatal(err)
	}
	// An account with the backup privilege switched on (a remote session's
	// administrator) reads the file all the same, so the takeover is judged
	// by the DACL as well: it is no longer a protected one.
	if _, err := os.ReadFile(locked); err == nil {
		t.Log("this account reads a file that lets nobody in; judging by the DACL only")
	}

	adoptWindowsState(programData, dir)

	taken := filepath.Join(dir, "config.json")
	if _, err := os.ReadFile(taken); err != nil {
		t.Fatalf("config.json not readable after the takeover: %v", err)
	}
	sd, err := windows.GetNamedSecurityInfo(taken, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	if c, _, err := sd.Control(); err != nil || c&windows.SE_DACL_PROTECTED != 0 {
		t.Fatalf("config.json still has its protected DACL after the takeover (control %#x, %v)", c, err)
	}
	kept, err := filepath.Glob(filepath.Join(dir, "pre-*", "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(kept) != 1 {
		t.Fatalf("the copy kept for a rollback has %d config.json, want 1", len(kept))
	}
	if data, err := os.ReadFile(kept[0]); err != nil || len(data) == 0 {
		t.Errorf("the kept config.json is empty or unreadable: %v", err)
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
