package main

import (
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/salvanza/salvanza_client/gui/api"
)

// The summary a service run's completion gave replaces the scheduler's
// generic "Backup completed" in Reports, with its message code and figures.
func TestUseRunSummary(t *testing.T) {
	generic := func(backupType, status string) JobHistory {
		return JobHistory{Name: "Set", Status: status, Message: "Backup completed",
			MessageKey: MsgBackupCompletedGeneric, BackupType: backupType}
	}
	machine := &serviceRunOutcome{success: true,
		message: "Machine backup completed in 5m 23s: 128.0 GB (119.2 GiB) read",
		key:     MsgMachineBackupCompletedStats,
		params:  msgParams{"duration": "5m 23s", "read": "128.0 GB (119.2 GiB)"}}

	e := generic("machine", "success")
	useRunSummary(&e, machine, true)
	if e.Message != machine.message || e.MessageKey != MsgMachineBackupCompletedStats || e.MessageParams["duration"] != "5m 23s" {
		t.Fatalf("machine run: %+v", e)
	}

	// A folder run's reused files come from its figures, as without the
	// service (they arrive from the API as JSON numbers).
	folder := &serviceRunOutcome{success: true, message: "Backup completed in 1m", key: MsgBackupCompleted,
		params: msgParams{"reusedFiles": float64(40), "reusedBytes": float64(4096)}}
	e = generic("directory", "success")
	useRunSummary(&e, folder, true)
	if e.ReusedFiles != 40 || e.ReusedBytes != 4096 || e.MessageKey != MsgBackupCompleted {
		t.Fatalf("folder run: %+v", e)
	}

	// The cases that keep the scheduler's own entry.
	for name, c := range map[string]struct {
		outcome   *serviceRunOutcome
		succeeded bool
		status    string
	}{
		"no outcome":            {nil, true, "success"},
		"empty summary":         {&serviceRunOutcome{success: true}, true, "success"},
		"outcome disagrees":     {machine, false, "failed"},
		"cancelled keeps words": {&serviceRunOutcome{message: "stopped"}, false, "cancelled"},
	} {
		e := generic("machine", c.status)
		useRunSummary(&e, c.outcome, c.succeeded)
		if e.Message != "Backup completed" || e.MessageKey != MsgBackupCompletedGeneric {
			t.Errorf("%s: entry changed to %+v", name, e)
		}
	}

	// A failed run reads as its completion said, as without the service.
	failed := &serviceRunOutcome{message: "Machine backup failed: disk not found", key: MsgScheduledJobError}
	e = generic("machine", "failed")
	useRunSummary(&e, failed, false)
	if e.Message != failed.message || e.Status != "failed" {
		t.Fatalf("failed run: %+v", e)
	}
}

// In the service, a backup's completion is kept on its run for the
// scheduler's history write, and its API entry carries the message code and
// figures to the app that asked for the run (Run Now).
func TestServiceRunCompletionCarriesItsSummary(t *testing.T) {
	tokenPath := filepath.Join(t.TempDir(), "api-token")
	tok, err := api.EnsureToken(tokenPath)
	if err != nil {
		t.Fatal(err)
	}
	server := api.NewServer("127.0.0.1:0", &fakeService{}, tok, "test")
	hs := httptest.NewServer(server.Handler())
	defer hs.Close()

	a := &App{isServiceProcess: true, runReporter: server}
	key := a.registerPendingPostActions(ScheduledJob{ID: "j1", Name: "DMX", BackupType: "machine"}, "manual")
	runID := server.BeginRun("DMX")

	// What the service's OnComplete does (app_service_stubs.go).
	params := msgParams{"duration": "5m 23s"}
	a.setRunOutcome(key, serviceRunOutcome{success: true, message: "Machine backup completed in 5m 23s", key: MsgMachineBackupCompletedStats, params: params})
	a.finishRunKeyed(runID, true, "Machine backup completed in 5m 23s", MsgMachineBackupCompletedStats, params)
	a.finishRun(runID, true, "Backup completed") // the scheduler's late close changes nothing

	if o := a.runOutcome(key); o == nil || o.key != MsgMachineBackupCompletedStats || o.params["duration"] != "5m 23s" {
		t.Fatalf("outcome kept on the run = %+v", o)
	}
	if a.runOutcome("") != nil || a.runOutcome("unknown") != nil {
		t.Fatal("an outcome for a run that has none")
	}

	st, err := api.NewClientAt(hs.URL, tokenPath).GetBackupStatus(runID)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Complete || !st.Success || st.Message != "Machine backup completed in 5m 23s" ||
		st.MessageKey != string(MsgMachineBackupCompletedStats) || st.MessageParams["duration"] != "5m 23s" {
		t.Fatalf("API entry = %+v", st)
	}
}
