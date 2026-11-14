package tui

import "testing"

// A set's Result, "Why" and "Last OK" come from its own runs, by the set's
// ID: a new set named as a deleted one does not show the deleted set's
// failure, and a renamed set keeps its runs (issue #143).
func TestRunsAreTheSetsOwnByID(t *testing.T) {
	hist := []HistoryEntry{ // newest first, as the service gives them
		toHistory(map[string]interface{}{"id": "r5", "name": "Test", "jobId": "old", "status": "failed", "message": "refused", "trigger": "manual"}),
		toHistory(map[string]interface{}{"id": "r4", "name": "Test", "status": "failed", "trigger": "oneoff"}),
		toHistory(map[string]interface{}{"id": "r3", "name": "Test", "status": "failed", "kind": "restore"}),
		toHistory(map[string]interface{}{"id": "r2", "name": "Before rename", "jobId": "new", "status": "success", "trigger": "scheduled"}),
		toHistory(map[string]interface{}{"id": "r1", "name": "Test", "jobId": "-", "status": "success"}),
	}
	if hist[0].SetID != "old" || hist[1].Trigger != "oneoff" || hist[2].Kind != "restore" {
		t.Fatalf("toHistory dropped the set ID, trigger or kind: %+v", hist[:3])
	}
	a := &App{hist: hist}

	fresh := Set{ID: "new", Name: "Test"}
	last, ok := a.lastResult(fresh)
	if !ok || last.ID != "r2" {
		t.Errorf("the new set's last result: %+v (found %v), want its own run r2", last, ok)
	}
	if okRun, ok := a.lastSuccess(fresh); !ok || okRun.ID != "r2" {
		t.Errorf("the new set's last success: %+v (found %v), want r2", okRun, ok)
	}

	never := Set{ID: "newer", Name: "Test"}
	if h, ok := a.lastResult(never); ok {
		t.Errorf("a set that never ran shows %+v", h)
	}
	if h, ok := a.lastSuccess(never); ok {
		t.Errorf("a set that never ran shows a success: %+v", h)
	}

	// An entry the service has not tied to a set yet goes by its name, but
	// never a one-off or a restore.
	a.hist = []HistoryEntry{
		{ID: "o3", Name: "Documents", Status: "failed", Trigger: "oneoff"},
		{ID: "o2", Name: "Documents", Status: "failed", Kind: "restore"},
		{ID: "o1", Name: "Documents", Status: "success"},
	}
	if h, ok := a.lastResult(Set{ID: "d", Name: "Documents"}); !ok || h.ID != "o1" {
		t.Errorf("an entry with no set ID: %+v (found %v), want o1", h, ok)
	}
}
