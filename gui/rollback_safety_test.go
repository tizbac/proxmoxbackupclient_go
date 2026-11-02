package main

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPathWithin(t *testing.T) {
	root := filepath.Join(t.TempDir(), "set")
	for _, c := range []struct {
		p    string
		want bool
	}{
		{root, true},
		{filepath.Join(root, "a", "b.txt"), true},
		{filepath.Join(root, "..", "other"), false},
		{filepath.Join(root, "a", "..", "..", "x"), false},
		{root + "2", false}, // a sibling sharing the prefix
	} {
		if got := pathWithin(c.p, root); got != c.want {
			t.Errorf("pathWithin(%q) = %v, want %v", c.p, got, c.want)
		}
	}
}

// A record is acted on only if everything in it belongs to this set's roll
// back: anyone who could edit it would otherwise choose what Undo deletes.
func TestCheckRollbackRecord(t *testing.T) {
	base := t.TempDir()
	set := filepath.Join(base, "data")
	old := rollbackSafetyRoot
	rollbackSafetyRoot = func(string) string { return filepath.Join(base, ".pbs-rollback") }
	t.Cleanup(func() { rollbackSafetyRoot = old })

	job := ScheduledJob{ID: "j", Name: "Set", BackupType: "directory", BackupDirs: []string{set}}
	safety := filepath.Join(base, ".pbs-rollback", "Set", "20260101-000000")
	manifest := filepath.Join(safety, "rollback.json")
	good := func() *rollbackManifest {
		return &rollbackManifest{
			JobID: "j", SetName: "Set", Created: time.Now(), Status: "done",
			Moved:       []rollbackMove{{From: filepath.Join(set, "a.txt"), To: filepath.Join(safety, "data", "a.txt"), Kind: "replaced"}},
			Restored:    []string{filepath.Join(set, "a.txt"), filepath.Join(set, "sub", "b.txt")},
			CreatedDirs: []string{filepath.Join(set, "sub")},
			RemovedDirs: []string{filepath.Join(set, "gone")},
			SafetyRoots: []string{safety},
		}
	}
	if err := checkRollbackRecord(job, manifest, good()); err != nil {
		t.Fatalf("a genuine record was refused: %v", err)
	}

	outside := filepath.Join(base, "Windows", "system32", "important.dll")
	for name, spoil := range map[string]func(m *rollbackManifest){
		"restored file outside the set": func(m *rollbackManifest) { m.Restored = append(m.Restored, outside) },
		"moved from outside the set":    func(m *rollbackManifest) { m.Moved[0].From = outside },
		"kept copy outside safety":      func(m *rollbackManifest) { m.Moved[0].To = outside },
		"created folder outside":        func(m *rollbackManifest) { m.CreatedDirs = append(m.CreatedDirs, filepath.Dir(outside)) },
		"removed folder outside":        func(m *rollbackManifest) { m.RemovedDirs = append(m.RemovedDirs, filepath.Dir(outside)) },
		"climbing out with ..":          func(m *rollbackManifest) { m.Restored[0] = filepath.Join(set, "..", "Windows", "x") },
		"safety folder elsewhere":       func(m *rollbackManifest) { m.SafetyRoots = append(m.SafetyRoots, filepath.Join(base, "Windows")) },
		"another set's record":          func(m *rollbackManifest) { m.JobID = "other" },
		"no safety folder":              func(m *rollbackManifest) { m.SafetyRoots = nil },
	} {
		m := good()
		spoil(m)
		if err := checkRollbackRecord(job, manifest, m); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	// The record must sit in one of its own safety folders.
	if err := checkRollbackRecord(job, filepath.Join(base, "rollback.json"), good()); err == nil || !strings.Contains(err.Error(), "own safety") {
		t.Errorf("record outside its safety folder: %v", err)
	}
}
