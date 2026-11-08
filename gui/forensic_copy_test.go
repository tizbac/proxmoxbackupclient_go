package main

import "testing"

// A machine set's forensic copy setting travels with its run, both for a run
// the scheduler starts and for a request the GUI hands to the service.
func TestReadFreeSpaceTravelsWithTheRun(t *testing.T) {
	a := &App{}
	if a.runReadFreeSpace("") {
		t.Fatal("a one-off run is a forensic copy")
	}
	on := a.registerPendingPostActions(ScheduledJob{ID: "m1", BackupType: "machine", ReadFreeSpace: true}, "scheduled")
	if !a.runReadFreeSpace(on) {
		t.Fatal("the set's forensic copy did not reach its run")
	}
	off := a.registerPendingPostActions(ScheduledJob{ID: "m2", BackupType: "machine"}, "scheduled")
	if a.runReadFreeSpace(off) {
		t.Fatal("a normal machine set ran as a forensic copy")
	}
	if !a.runReadFreeSpace(a.RegisterRunOptions("machine", "", true)) {
		t.Fatal("a GUI request's forensic copy was lost on the service side")
	}
}
