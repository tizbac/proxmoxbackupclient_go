package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// A run is its set's by the set's ID: a new set named as a deleted one does
// not take the deleted set's runs, and a renamed set keeps its own (#143).
func TestRunOfSet(t *testing.T) {
	set := ScheduledJob{ID: "new", Name: "Test"}
	for _, tc := range []struct {
		name string
		h    JobHistory
		want bool
	}{
		{"its own run", JobHistory{Name: "Test", JobID: "new"}, true},
		{"its own run under an earlier name", JobHistory{Name: "Before rename", JobID: "new"}, true},
		{"a deleted set of the same name", JobHistory{Name: "Test", JobID: "old"}, false},
		{"an older run of no set there is now", JobHistory{Name: "Test", JobID: historyNoSet}, false},
		{"not tied to a set yet, same name", JobHistory{Name: "Test"}, true},
		{"not tied to a set yet, another name", JobHistory{Name: "Other"}, false},
		{"a one-off backup with the set's name", JobHistory{Name: "Test", Trigger: "oneoff"}, false},
		{"a restore with the set's name", JobHistory{Name: "Test", Kind: "restore"}, false},
		{"a restore carrying the set's ID", JobHistory{Name: "Test", Kind: "restore", JobID: "new"}, false},
	} {
		if got := runOfSet(tc.h, set); got != tc.want {
			t.Errorf("%s: runOfSet = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestStampHistorySetIDs(t *testing.T) {
	jobs := []ScheduledJob{{ID: "d1", Name: "Documents"}, {ID: "p1", Name: "Photos"}, {Name: "No ID"}}
	history := []JobHistory{
		{ID: "1", Name: "Documents", Trigger: "scheduled"},
		{ID: "2", Name: "Photos"}, // from before runs said how they started
		{ID: "3", Name: "Deleted set", Trigger: "manual"},
		{ID: "4", Name: "Documents", Trigger: "oneoff"},
		{ID: "5", Name: "Restore of Documents", Kind: "restore"},
		{ID: "6", Name: "Documents", JobID: "earlier", Trigger: "manual"},
		{ID: "7", Name: "documents", Trigger: "manual"}, // the name exactly, or it is not the set's
		{ID: "8", Name: "No ID", Trigger: "manual"},
	}
	if !stampHistorySetIDs(history, jobs) {
		t.Fatal("stampHistorySetIDs reported no change")
	}
	want := []string{"d1", "p1", historyNoSet, "", "", "earlier", historyNoSet, historyNoSet}
	for i, h := range history {
		if h.JobID != want[i] {
			t.Errorf("entry %s (%s): JobID %q, want %q", h.ID, h.Name, h.JobID, want[i])
		}
	}
	// Once only: nothing is left to do, and nothing moves.
	if stampHistorySetIDs(history, []ScheduledJob{{ID: "x", Name: "Deleted set"}, {ID: "y", Name: "Documents"}}) {
		t.Error("a second pass changed entries that were already tied")
	}
	for i, h := range history {
		if h.JobID != want[i] {
			t.Errorf("after a second pass, entry %s: JobID %q, want %q", h.ID, h.JobID, want[i])
		}
	}
	if stampHistorySetIDs(nil, jobs) {
		t.Error("an empty history reported a change")
	}
}

// Sean's steps on #143: a set fails, is deleted, and a new set gets its name.
// The tray and the public status must not hold the old failure against it.
func TestLastRunOutcomeBySetID(t *testing.T) {
	old := JobHistory{Name: "Test", JobID: "old", Status: "failed", Timestamp: "t1"}
	jobs := []ScheduledJob{{ID: "new", Name: "Test"}}
	if failed, ended := lastRunOutcome(jobs, []JobHistory{old}); failed || ended != "t1" {
		t.Errorf("a deleted set's failure counted for the new set: failed=%v ended=%q", failed, ended)
	}
	// The new set's own runs do count, under whatever name it had then.
	own := JobHistory{Name: "Earlier name", JobID: "new", Status: "failed", Timestamp: "t2"}
	if failed, _ := lastRunOutcome(jobs, []JobHistory{own, old}); !failed {
		t.Error("the set's own failed run under an earlier name did not count")
	}
	fine := JobHistory{Name: "Test", JobID: "new", Status: "success", Timestamp: "t3"}
	if failed, ended := lastRunOutcome(jobs, []JobHistory{fine, own, old}); failed || ended != "t3" {
		t.Errorf("fine since: failed=%v ended=%q", failed, ended)
	}
	// Two sets, the most recent run of each: one of them failed.
	two := []ScheduledJob{{ID: "a", Name: "A"}, {ID: "b", Name: "B"}}
	hist := []JobHistory{
		{Name: "A", JobID: "a", Status: "success", Timestamp: "t4"},
		{Name: "B", JobID: "b", Status: "failed", Timestamp: "t3"},
		{Name: "B", JobID: "b", Status: "success", Timestamp: "t2"},
	}
	if failed, _ := lastRunOutcome(two, hist); !failed {
		t.Error("set B's most recent run failed and did not count")
	}
}

func TestPendingEntrySetID(t *testing.T) {
	var none *pendingPostActionsEntry
	if got := none.setID(); got != "" {
		t.Errorf("no entry: %q", got)
	}
	set := &pendingPostActionsEntry{job: &ScheduledJob{ID: "s1"}, trigger: "manual"}
	if got := set.setID(); got != "s1" {
		t.Errorf("a set's run: %q", got)
	}
	// A request the app handed to the service has no set behind it.
	req := &pendingPostActionsEntry{job: &ScheduledJob{ID: "service-request"}}
	if got := req.setID(); got != "" {
		t.Errorf("a one-off request: %q", got)
	}
	if got := (&pendingPostActionsEntry{trigger: "manual"}).setID(); got != "" {
		t.Errorf("no job: %q", got)
	}
}

func writeJSONFile(t *testing.T, path string, v interface{}) {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}

// At a start, runs an earlier version recorded are tied to their sets in the
// history file, and a run left "running" is still marked abandoned.
func TestCleanupAbandonedJobsTiesHistoryToSets(t *testing.T) {
	useTempConfigDir(t)
	jobsPath, err := getScheduledJobsPath()
	if err != nil {
		t.Fatal(err)
	}
	histPath, err := getJobHistoryPath()
	if err != nil {
		t.Fatal(err)
	}
	writeJSONFile(t, jobsPath, []ScheduledJob{{ID: "d1", Name: "Documents", BackupType: "directory"}})
	writeJSONFile(t, histPath, []JobHistory{
		{ID: "3", Name: "Documents", Status: "running", Trigger: "scheduled"},
		{ID: "2", Name: "Gone", Status: "failed", Trigger: "manual"},
		{ID: "1", Name: "Documents", Status: "success"},
	})
	a := &App{}
	a.CleanupAbandonedJobs()
	got, err := a.GetJobHistory()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("%d entries, want 3", len(got))
	}
	if got[0].JobID != "d1" || got[0].Status != "failed" || got[0].MessageKey != MsgJobAbandoned {
		t.Errorf("the abandoned run: %+v", got[0])
	}
	if got[1].JobID != historyNoSet || got[2].JobID != "d1" {
		t.Errorf("JobIDs %q, %q; want %q, d1", got[1].JobID, got[2].JobID, historyNoSet)
	}

	// The set is deleted and a new one takes its name: the runs stay the
	// deleted set's.
	writeJSONFile(t, jobsPath, []ScheduledJob{{ID: "d2", Name: "Documents", BackupType: "directory"}})
	a.CleanupAbandonedJobs()
	got, err = a.GetJobHistory()
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range got {
		if runOfSet(h, ScheduledJob{ID: "d2", Name: "Documents"}) {
			t.Errorf("entry %s went to the new set of the same name", h.ID)
		}
	}
}

// Sets that cannot be read are no reason to mark every run as no set's: the
// history is left for the next start.
func TestCleanupAbandonedJobsLeavesHistoryWhenSetsUnreadable(t *testing.T) {
	useTempConfigDir(t)
	jobsPath, err := getScheduledJobsPath()
	if err != nil {
		t.Fatal(err)
	}
	histPath, err := getJobHistoryPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(jobsPath), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(jobsPath, []byte("{not json"), 0600); err != nil {
		t.Fatal(err)
	}
	writeJSONFile(t, histPath, []JobHistory{{ID: "1", Name: "Documents", Status: "success"}})
	a := &App{}
	a.CleanupAbandonedJobs()
	got, err := a.GetJobHistory()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].JobID != "" {
		t.Errorf("history changed with the sets unreadable: %+v", got)
	}
}
