package main

import (
	"testing"

	"pbscommon"
)

func TestChangeDetectionForRunFullReadEvery(t *testing.T) {
	job := ScheduledJob{BackupType: "directory", ChangeDetectionMode: pbscommon.ChangeDetectionMetadata, FullReadEvery: 4}
	var got []string
	for i := 0; i < 9; i++ {
		mode, next := changeDetectionForRun(job)
		got = append(got, mode)
		job.MetadataRunsSinceFullRead = next
	}
	want := []string{"metadata", "metadata", "metadata", "data", "metadata", "metadata", "metadata", "data", "metadata"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("runs %v, want %v", got, want)
		}
	}
}

func TestChangeDetectionForRunModes(t *testing.T) {
	for _, c := range []struct {
		job  ScheduledJob
		want string
	}{
		{ScheduledJob{BackupType: "directory"}, ""},
		{ScheduledJob{BackupType: "directory", ChangeDetectionMode: "legacy"}, ""},
		{ScheduledJob{BackupType: "directory", ChangeDetectionMode: "data", FullReadEvery: 3}, "data"},
		{ScheduledJob{BackupType: "directory", ChangeDetectionMode: "metadata"}, "metadata"},
		// Machine sets back up disk images: never any change detection.
		{ScheduledJob{BackupType: "machine", ChangeDetectionMode: "metadata", FullReadEvery: 2}, ""},
	} {
		if got, _ := changeDetectionForRun(c.job); got != c.want {
			t.Errorf("%+v: got %q, want %q", c.job, got, c.want)
		}
	}
	// metadata without a full-read interval never switches to data.
	job := ScheduledJob{BackupType: "directory", ChangeDetectionMode: "metadata"}
	for i := 0; i < 50; i++ {
		mode, next := changeDetectionForRun(job)
		if mode != "metadata" {
			t.Fatalf("run %d: %s", i, mode)
		}
		job.MetadataRunsSinceFullRead = next
	}
}

func TestValidateJobChangeDetection(t *testing.T) {
	ok := []ScheduledJob{
		{}, {ChangeDetectionMode: "legacy"}, {ChangeDetectionMode: "data"},
		{ChangeDetectionMode: "metadata", FullReadEvery: 7}, {ChangeDetectionMode: "metadata", FullReadEvery: 2},
	}
	for _, j := range ok {
		if err := validateScheduledJob(j); err != nil {
			t.Errorf("%+v rejected: %v", j, err)
		}
	}
	bad := []ScheduledJob{
		{ChangeDetectionMode: "fast"}, {ChangeDetectionMode: "metadata", FullReadEvery: 1},
		{ChangeDetectionMode: "metadata", FullReadEvery: -3}, {ChangeDetectionMode: "metadata", FullReadEvery: 400},
	}
	for _, j := range bad {
		if err := validateScheduledJob(j); err == nil {
			t.Errorf("%+v accepted", j)
		}
	}
}

func TestRunChangeDetectionModeTravelsWithTheRun(t *testing.T) {
	a := &App{}
	if a.runChangeDetectionMode("") != "" {
		t.Fatal("one-off run got a mode")
	}
	key := a.registerPendingPostActions(ScheduledJob{ID: "j1", BackupType: "directory", ChangeDetectionMode: "metadata"}, "scheduled")
	if got := a.runChangeDetectionMode(key); got != "metadata" {
		t.Fatalf("got %q", got)
	}
	mkey := a.registerPendingPostActions(ScheduledJob{ID: "j2", BackupType: "machine", ChangeDetectionMode: "metadata"}, "scheduled")
	if got := a.runChangeDetectionMode(mkey); got != "" {
		t.Fatalf("machine run got %q", got)
	}
	// A GUI request handed to the service.
	skey := a.RegisterRunOptions("directory", "data")
	if got := a.runChangeDetectionMode(skey); got != "data" {
		t.Fatalf("service request got %q", got)
	}
	if got := a.runChangeDetectionMode(a.RegisterRunOptions("directory", "bogus")); got != "" {
		t.Fatalf("invalid mode passed through as %q", got)
	}

	// The structured result rides back on the same key.
	a.setRunResult(key, &BackupStatus{ReusedFiles: 12})
	if r := a.takeRunResult(key); r == nil || r.ReusedFiles != 12 {
		t.Fatalf("result %+v", r)
	}
	if a.takePendingPostActions(key) == nil {
		t.Fatal("reading the result consumed the entry")
	}
}

func TestParamUint(t *testing.T) {
	p := msgParams{"a": uint64(5), "b": 7, "c": float64(9), "d": "x", "e": -2}
	for k, want := range map[string]uint64{"a": 5, "b": 7, "c": 9, "d": 0, "e": 0, "missing": 0} {
		if got := paramUint(p, k); got != want {
			t.Errorf("%s: %d, want %d", k, got, want)
		}
	}
}
