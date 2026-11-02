//go:build windows

package main

import (
	"os"
	"slices"
	"testing"

	"machinebackuplib"
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

	// The disk picker's partitions: C: is the system partition.
	parts, err := machinebackuplib.ListPartitions(boot.DevicePath)
	if err != nil || len(parts) == 0 {
		t.Fatalf("ListPartitions(%s) = %v, %v", boot.DevicePath, parts, err)
	}
	var system, last *machinebackuplib.PartitionInfo
	for i, p := range parts {
		t.Logf("  partition %d at %d, %d bytes: mount %q fs %q type %q role %q", p.Number, p.Start, p.Size, p.Mount, p.FileSystem, p.Type, p.Role)
		if p.Role == "system" {
			system = &parts[i]
		}
		last = &parts[i]
	}
	if system == nil || system.Mount != "C:" {
		t.Fatalf("no system partition on C: among %+v", parts)
	}

	a := &App{}
	skip := PartitionSkip{DiskID: boot.UniqueID, Device: boot.DevicePath, Start: last.Start, Size: last.Size, Number: last.Number, Label: "last"}
	key := a.registerPendingPostActions(ScheduledJob{ID: "live", BackupType: "machine",
		DriveLetters: []string{boot.DevicePath}, DiskIDs: []string{boot.UniqueID}, SkipPartitions: []PartitionSkip{skip}}, "manual")
	got, skips, err := a.resolveRunDisks([]string{boot.DevicePath}, key)
	if err != nil || len(got) != 1 || got[0] != boot.DevicePath {
		t.Fatalf("resolveRunDisks = %v, %v", got, err)
	}
	if s := skips[boot.DevicePath]; len(s) != 1 || s[0].Start != last.Start || s[0].Size != last.Size {
		t.Fatalf("partitions left out = %+v, want the last partition of %s", skips, boot.DevicePath)
	}
	gone := a.registerPendingPostActions(ScheduledJob{ID: "gone", BackupType: "machine",
		DriveLetters: []string{boot.DevicePath}, DiskIDs: []string{"serial:NOT-CONNECTED"}}, "manual")
	if _, _, err := a.resolveRunDisks([]string{boot.DevicePath}, gone); err == nil {
		t.Fatal("a set whose disk is not connected must not run")
	} else {
		t.Logf("not connected: %v", err)
	}
}
