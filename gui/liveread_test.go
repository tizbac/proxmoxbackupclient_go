package main

import (
	"errors"
	"runtime"
	"testing"
)

// A Linux machine set with Use snapshots off reads live only with the
// override accepted; without it the run stops before it starts. Elsewhere,
// and for sets with snapshots on, nothing changes.
func TestRunLiveRead(t *testing.T) {
	a := &App{}
	reg := func(off, accepted bool) string {
		return a.registerPendingPostActions(ScheduledJob{ID: "m", BackupType: "machine",
			SnapshotsOff: off, LiveRead: accepted}, "scheduled")
	}
	linux := runtime.GOOS == "linux"

	if live, err := a.runLiveRead(reg(false, false)); live || err != nil {
		t.Errorf("snapshots on: live %v, err %v", live, err)
	}
	live, err := a.runLiveRead(reg(true, true))
	if live != linux || err != nil {
		t.Errorf("off and accepted: live %v, err %v (want live on Linux only)", live, err)
	}
	live, err = a.runLiveRead(reg(true, false))
	if linux && (live || !errors.Is(err, errSnapshotsOffNotAccepted)) {
		t.Errorf("off, not accepted on Linux: live %v, err %v; want the run stopped", live, err)
	}
	if !linux && (live || err != nil) {
		t.Errorf("off, not accepted on %s: live %v, err %v; want ignored", runtime.GOOS, live, err)
	}
	if live, err := a.runLiveRead(""); live || err != nil {
		t.Errorf("one-off backup: live %v, err %v", live, err)
	}

	// A GUI's request to the service carries the accepted live read.
	key := a.RegisterRunOptions("machine", "", false)
	a.SetRunLiveRead(key, true)
	if live, err := a.runLiveRead(key); live != linux || err != nil {
		t.Errorf("service request: live %v, err %v", live, err)
	}
}

// Only machine sets keep the choice.
func TestPinJobDisksDropsLiveReadOnFolderSets(t *testing.T) {
	job := ScheduledJob{BackupType: "directory", SnapshotsOff: true, LiveRead: true}
	pinJobDisks(&job)
	if job.SnapshotsOff || job.LiveRead {
		t.Errorf("folder set kept the machine snapshot choice: %+v", job)
	}
}
