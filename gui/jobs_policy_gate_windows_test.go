//go:build windows

package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// These exercise the real add / update / delete / import-style paths against a
// scratch ProgramData. They behave differently depending on whether the test
// process itself is elevated, so run them both ways (elevated shell, and a
// de-elevated one via `runas /trustlevel:0x20000`); the other case skips.

func scratchProgramData(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("ProgramData", dir)
	t.Setenv("SystemDrive", "")
}

func validJob(id string) ScheduledJob {
	return ScheduledJob{
		ID: id, Name: "gate test " + id, ScheduleTime: "02:00", BackupDirs: []string{`C:\Temp`},
		BackupID: "gate-" + id, BackupType: "directory", TriggerMode: "daily", Compression: "fastest",
	}
}

func jobsFileExists(t *testing.T) bool {
	t.Helper()
	p, err := getScheduledJobsPath()
	if err != nil {
		t.Fatal(err)
	}
	_, err = os.Stat(p)
	return err == nil
}

func TestGate_StandardUser_PolicyOn_Blocked(t *testing.T) {
	if isAdmin() {
		t.Skip("needs a non-elevated process")
	}
	useTempPolicyKey(t) // missing value => ON
	scratchProgramData(t)
	a := &App{}

	if err := a.SaveScheduledJob(validJob("a")); !errors.Is(err, errJobsAdminRequired) {
		t.Fatalf("Save: got %v want JOBS_ADMIN_REQUIRED", err)
	}
	if err := a.UpdateScheduledJob(validJob("a")); !errors.Is(err, errJobsAdminRequired) {
		t.Fatalf("Update: got %v", err)
	}
	if err := a.DeleteScheduledJob("a"); !errors.Is(err, errJobsAdminRequired) {
		t.Fatalf("Delete: got %v", err)
	}
	if err := a.SaveScheduledJobFromMap(map[string]interface{}{"id": "a"}); !errors.Is(err, errJobsAdminRequired) {
		t.Fatalf("API save: got %v", err)
	}
	if jobsFileExists(t) {
		t.Fatal("a blocked change must not create the jobs file")
	}
	if err := a.SetRequireAdminForJobs(false); !errors.Is(err, errJobsAdminRequired) {
		t.Fatalf("a standard user must not be able to switch the policy off: got %v", err)
	}
	if !readRequireAdminForJobs() {
		t.Fatal("policy changed by a standard user")
	}
	p := a.GetJobPolicy()
	if !p.Supported || !p.RequireAdmin || p.IsAdmin || p.CanModify {
		t.Fatalf("unexpected policy report: %+v", p)
	}
}

func TestGate_StandardUser_PolicyOff_Allowed(t *testing.T) {
	if isAdmin() {
		t.Skip("needs a non-elevated process")
	}
	useTempPolicyKey(t)
	// Writing the throwaway HKCU key is allowed for a standard user (it is
	// their own hive), so this simulates an administrator having switched the
	// policy off.
	if err := writeRequireAdminForJobs(false); err != nil {
		t.Fatal(err)
	}
	scratchProgramData(t)
	a := &App{}
	if err := a.SaveScheduledJob(validJob("b")); err != nil {
		t.Fatalf("with the policy off a standard user may add jobs: %v", err)
	}
	if !jobsFileExists(t) {
		t.Fatal("job was not saved")
	}
	if err := a.DeleteScheduledJob("b"); err != nil {
		t.Fatalf("delete with policy off: %v", err)
	}
}

func TestGate_Admin_PolicyOn_Allowed(t *testing.T) {
	if !isAdmin() {
		t.Skip("needs an elevated process")
	}
	useTempPolicyKey(t)
	scratchProgramData(t)
	a := &App{}
	if err := a.SaveScheduledJob(validJob("c")); err != nil {
		t.Fatalf("an administrator may add jobs with the policy on: %v", err)
	}
	if err := a.UpdateScheduledJob(validJob("c")); err != nil {
		t.Fatalf("update: %v", err)
	}
	if err := a.DeleteScheduledJob("c"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	// The service's HTTP job routes stay closed while the policy is on, even
	// though the caller (here, an admin test process) is elevated.
	if err := a.SaveScheduledJobFromMap(map[string]interface{}{"id": "c"}); !errors.Is(err, errJobsAdminRequired) {
		t.Fatalf("API route must be refused while the policy is on: got %v", err)
	}
	p := a.GetJobPolicy()
	if !p.RequireAdmin || !p.IsAdmin || !p.CanModify {
		t.Fatalf("unexpected policy report: %+v", p)
	}
}

func TestGate_Admin_CanSwitchPolicy(t *testing.T) {
	if !isAdmin() {
		t.Skip("needs an elevated process")
	}
	useTempPolicyKey(t)
	a := &App{}
	if err := a.SetRequireAdminForJobs(false); err != nil {
		t.Fatal(err)
	}
	if readRequireAdminForJobs() {
		t.Fatal("expected OFF")
	}
	scratchProgramData(t)
	if err := a.SaveScheduledJobFromMap(validJobMap("d")); err != nil {
		t.Fatalf("API route should be open with the policy off: %v", err)
	}
	if err := a.SetRequireAdminForJobs(true); err != nil {
		t.Fatal(err)
	}
	if !readRequireAdminForJobs() {
		t.Fatal("expected ON")
	}
}

func validJobMap(id string) map[string]interface{} {
	j := validJob(id)
	return map[string]interface{}{
		"id": j.ID, "name": j.Name, "scheduleTime": j.ScheduleTime, "backupDirs": j.BackupDirs,
		"backupId": j.BackupID, "backupType": j.BackupType, "triggerMode": j.TriggerMode, "compression": j.Compression,
	}
}

var _ = filepath.Join
