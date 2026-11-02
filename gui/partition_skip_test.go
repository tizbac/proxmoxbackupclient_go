package main

import (
	"reflect"
	"testing"

	"machinebackuplib"
)

// A run leaves a partition out of the disk the set saved it on, wherever the
// system has put that disk now, and of no other disk.
func TestSkipsByDevice(t *testing.T) {
	efi := PartitionSkip{DiskID: "serial:AAA", Device: `\\.\PhysicalDrive0`, Start: 1 << 20, Size: 100 << 20, Number: 1, Label: "partition 1"}
	data := PartitionSkip{Device: `\\.\PhysicalDrive1`, Start: 16 << 20, Size: 1 << 30, Number: 2, Label: "D: (NTFS)"}
	other := PartitionSkip{DiskID: "serial:GONE", Device: `\\.\PhysicalDrive5`, Start: 1 << 20, Size: 1 << 20, Label: "old"}
	devices := []string{`\\.\PhysicalDrive0`, `\\.\PhysicalDrive1`}
	ids := []string{"serial:AAA", ""}

	// Disk AAA is PhysicalDrive3 now.
	paths := []string{`\\.\PhysicalDrive3`, `\\.\PhysicalDrive1`}
	got, dropped := skipsByDevice([]PartitionSkip{efi, data, other}, devices, ids, paths)
	want := map[string][]machinebackuplib.PartitionSkip{
		`\\.\PhysicalDrive3`: {{Start: 1 << 20, Size: 100 << 20, Number: 1, Label: "partition 1"}},
		`\\.\PhysicalDrive1`: {{Start: 16 << 20, Size: 1 << 30, Number: 2, Label: "D: (NTFS)"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("skipsByDevice = %+v, want %+v", got, want)
	}
	if len(dropped) != 1 {
		t.Errorf("dropped %q, want the skip of a disk not in the set", dropped)
	}

	// A skip saved without an identity never lands on a disk that has one:
	// that disk may be another one at the same path.
	got, _ = skipsByDevice([]PartitionSkip{{Device: `\\.\PhysicalDrive0`, Start: 1 << 20, Size: 1 << 20}}, devices, ids, devices)
	if got != nil {
		t.Errorf("skip without identity on a disk with one: %+v", got)
	}

	// Linux, no identities at all: by device path.
	got, _ = skipsByDevice([]PartitionSkip{{Device: "/dev/sda", Start: 1 << 20, Size: 1 << 20}}, []string{"/dev/sda"}, nil, []string{"/dev/sda"})
	if len(got["/dev/sda"]) != 1 {
		t.Errorf("Linux by path: %+v", got)
	}

	// No skips: no map.
	if got, dropped := skipsByDevice(nil, devices, ids, paths); got != nil || dropped != nil {
		t.Errorf("no skips: %+v, %q", got, dropped)
	}
}

// Saving a set keeps its skips in step with its disks.
func TestPinSetSkips(t *testing.T) {
	keep := PartitionSkip{DiskID: "serial:AAA", Device: `PhysicalDrive0`, Start: 1 << 20, Size: 1 << 20}
	adopt := PartitionSkip{Device: `\\.\PhysicalDrive1`, Start: 2 << 20, Size: 1 << 20}
	gone := PartitionSkip{DiskID: "serial:CCC", Device: `\\.\PhysicalDrive2`, Start: 1 << 20, Size: 1 << 20}
	noStart := PartitionSkip{DiskID: "serial:AAA", Device: `\\.\PhysicalDrive0`, Number: 3}
	job := ScheduledJob{BackupType: "machine", DriveLetters: []string{`\\.\PhysicalDrive0`, `\\.\PhysicalDrive1`},
		DiskIDs: []string{"serial:AAA", "guid:bbb"}, SkipPartitions: []PartitionSkip{keep, adopt, gone, noStart}}
	pinSetSkips(&job)
	if len(job.SkipPartitions) != 2 {
		t.Fatalf("SkipPartitions = %+v, want the two of disks in the set", job.SkipPartitions)
	}
	if s := job.SkipPartitions[0]; s.DiskID != "serial:AAA" || s.Device != `\\.\PhysicalDrive0` {
		t.Errorf("kept %+v, want its device as the set names it", s)
	}
	if s := job.SkipPartitions[1]; s.DiskID != "guid:bbb" {
		t.Errorf("saved without an identity: %+v, want the disk's identity", s)
	}

	job = ScheduledJob{BackupType: "directory", SkipPartitions: []PartitionSkip{keep}}
	pinSetSkips(&job)
	if job.SkipPartitions != nil {
		t.Errorf("folder set: %+v", job.SkipPartitions)
	}
}

// A Backup Set run carries the set's skips: in the scheduler's job copy, and
// in a GUI's request to the service.
func TestRunSkipPartitions(t *testing.T) {
	a := &App{}
	skip := PartitionSkip{Device: "/dev/sda", Start: 1 << 20, Size: 1 << 20}
	key := a.registerPendingPostActions(ScheduledJob{ID: "m", BackupType: "machine", DriveLetters: []string{"/dev/sda"},
		SkipPartitions: []PartitionSkip{skip}}, "scheduled")
	devs, skips, err := a.resolveRunDisks([]string{"/dev/sda"}, key)
	if err != nil || !sameList(devs, []string{"/dev/sda"}) || len(skips["/dev/sda"]) != 1 || skips["/dev/sda"][0].Start != 1<<20 {
		t.Errorf("scheduler run: %v, %+v, %v", devs, skips, err)
	}

	skey := a.RegisterRunOptions("machine", "", false)
	if got := a.runSkipPartitions(skey); got != nil {
		t.Errorf("request without skips: %+v", got)
	}
	a.SetRunSkipPartitions(skey, []PartitionSkip{skip})
	if got := a.runSkipPartitions(skey); len(got) != 1 || got[0] != skip {
		t.Errorf("service request: %+v", got)
	}
}
