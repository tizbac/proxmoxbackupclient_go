package main

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

// Disabling a set keeps it and its settings but takes it out of the schedule;
// enabling it again works out the next run from now, so a set disabled for a
// while does not fire at once for the runs it missed. Other sets are left as
// they were.
func TestSetScheduledJobEnabled(t *testing.T) {
	deployTestDir(t)
	past := time.Now().Add(-72 * time.Hour).UTC().Format(time.RFC3339)
	jobs := []ScheduledJob{
		{ID: "a", Name: "Hourly", BackupID: "pc", BackupDirs: []string{`C:\Data`}, BackupType: "directory", Enabled: true,
			ScheduleMode: "scheduled", Schedules: []Schedule{{ID: "s1", Enabled: true, Unit: "hours", Every: 1}}, NextRun: past, LastRun: past},
		{ID: "b", Name: "Other", BackupID: "pc2", BackupDirs: []string{`C:\Other`}, BackupType: "directory", Enabled: true, NextRun: past},
	}
	path, err := getScheduledJobsPath()
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(jobs)
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	a := &App{}

	if err := a.SetScheduledJobEnabled("a", false); err != nil {
		t.Fatal(err)
	}
	got, err := a.GetScheduledJobs()
	if err != nil || len(got) != 2 {
		t.Fatalf("jobs after disable: %v %v", got, err)
	}
	if got[0].Enabled || got[0].Name != "Hourly" || len(got[0].Schedules) != 1 || got[0].LastRun != past {
		t.Errorf("disabled set changed beyond Enabled: %+v", got[0])
	}
	if !got[1].Enabled || got[1].NextRun != past {
		t.Errorf("the other set was touched: %+v", got[1])
	}

	if err := a.SetScheduledJobEnabled("a", true); err != nil {
		t.Fatal(err)
	}
	got, _ = a.GetScheduledJobs()
	if !got[0].Enabled {
		t.Fatal("set not enabled again")
	}
	next, err := time.Parse(time.RFC3339, got[0].NextRun)
	if err != nil || !next.After(time.Now()) {
		t.Errorf("next run after enabling = %q, want a time after now (no catch-up of missed runs)", got[0].NextRun)
	}

	if err := a.SetScheduledJobEnabled("missing", true); err == nil {
		t.Error("enabling a set that does not exist gave no error")
	}
}
