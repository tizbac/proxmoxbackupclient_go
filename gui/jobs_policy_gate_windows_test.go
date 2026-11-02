//go:build windows

package main

import (
	"os"
	"strings"
	"testing"
)

// The "Require administrator for Backup Sets" policy is not offered any more
// (upstream's strict model): only an app holding the service's token, which
// it gets through UAC, reaches the service's Backup Sets, so the token is the
// gate. These run the real add / update / delete / API paths against a
// scratch settings folder, elevated or not.

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

// Backup Set changes are not gated by the policy, whether the process is
// elevated or not, and the API's job routes are open (the token guards them).
func TestGate_NoPolicy_ChangesAllowed(t *testing.T) {
	useTempPolicyKey(t) // a missing value used to mean ON
	scratchProgramData(t)
	a := &App{}
	if err := a.SaveScheduledJob(validJob("c")); err != nil {
		t.Fatalf("add: %v", err)
	}
	if !jobsFileExists(t) {
		t.Fatal("job was not saved")
	}
	if err := a.UpdateScheduledJob(validJob("c")); err != nil {
		t.Fatalf("update: %v", err)
	}
	if err := a.DeleteScheduledJob("c"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := a.SaveScheduledJobFromMap(validJobMap("d")); err != nil {
		t.Fatalf("API route: %v", err)
	}
	p := a.GetJobPolicy()
	if p.Supported || !p.CanModify {
		t.Fatalf("policy report %+v, want not supported and changes allowed", p)
	}
}

// Switching the policy is refused with a clear message: there is nothing to
// switch.
func TestGate_PolicySwitchRefused(t *testing.T) {
	useTempPolicyKey(t)
	a := &App{}
	err := a.SetRequireAdminForJobs(false)
	if err == nil || !strings.Contains(err.Error(), "not available") {
		t.Fatalf("SetRequireAdminForJobs = %v, want not available", err)
	}
}

func validJobMap(id string) map[string]interface{} {
	j := validJob(id)
	return map[string]interface{}{
		"id": j.ID, "name": j.Name, "scheduleTime": j.ScheduleTime, "backupDirs": j.BackupDirs,
		"backupId": j.BackupID, "backupType": j.BackupType, "triggerMode": j.TriggerMode, "compression": j.Compression,
	}
}
