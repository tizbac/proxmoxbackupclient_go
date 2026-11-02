//go:build windows

package main

import (
	"os"
	"slices"
	"testing"
)

// Opt-in (PBS_DISKLIST=1, elevated): this computer's disks carry identities
// and drive letters, and a machine Backup Set run finds its disk by identity
// and refuses one that is not connected.
func TestLiveDiskListAndSetRun(t *testing.T) {
	if os.Getenv("PBS_DISKLIST") != "1" {
		t.Skip("set PBS_DISKLIST=1 (elevated)")
	}
	disks, err := listPhysicalDisks()
	if err != nil || len(disks) == 0 {
		t.Fatalf("listPhysicalDisks = %v, %v", disks, err)
	}
	var boot *PhysicalDiskInfo
	for i, d := range disks {
		t.Logf("%s: %q id %q other %q letters %v boot %v", d.DevicePath, d.Model, d.UniqueID, d.OtherIDs, d.DriveLetters, d.IsBootDisk)
		if d.IsBootDisk {
			boot = &disks[i]
		}
	}
	if boot == nil {
		t.Fatal("no boot disk")
	}
	if !isStableDiskID(boot.UniqueID) || !slices.Contains(boot.DriveLetters, "C:") {
		t.Fatalf("boot disk %s: id %q letters %v", boot.DevicePath, boot.UniqueID, boot.DriveLetters)
	}

	a := &App{}
	key := a.registerPendingPostActions(ScheduledJob{ID: "live", BackupType: "machine",
		DriveLetters: []string{boot.DevicePath}, DiskIDs: []string{boot.UniqueID}}, "manual")
	got, err := a.resolveRunDisks([]string{boot.DevicePath}, key)
	if err != nil || len(got) != 1 || got[0] != boot.DevicePath {
		t.Fatalf("resolveRunDisks = %v, %v", got, err)
	}
	gone := a.registerPendingPostActions(ScheduledJob{ID: "gone", BackupType: "machine",
		DriveLetters: []string{boot.DevicePath}, DiskIDs: []string{"serial:NOT-CONNECTED"}}, "manual")
	if _, err := a.resolveRunDisks([]string{boot.DevicePath}, gone); err == nil {
		t.Fatal("a set whose disk is not connected must not run")
	} else {
		t.Logf("not connected: %v", err)
	}
}
