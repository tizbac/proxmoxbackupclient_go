package main

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"
)

func disk(num int64, path, id string, other ...string) PhysicalDiskInfo {
	return PhysicalDiskInfo{
		DiskNumber: num,
		DeviceID:   "PhysicalDrive" + strconv.FormatInt(num, 10),
		DevicePath: path,
		UniqueID:   id,
		OtherIDs:   other,
		Size:       512 * 1024 * 1024 * 1024,
		Model:      "TEST DISK",
	}
}

func sameList(a, b []string) bool {
	return strings.Join(a, "|") == strings.Join(b, "|")
}

func TestNormalizePhysicalDrivePath(t *testing.T) {
	ok := map[string]string{
		`\\.\PhysicalDrive0`:   `\\.\PhysicalDrive0`,
		`PhysicalDrive12`:      `\\.\PhysicalDrive12`,
		`\\?\PhysicalDrive3`:   `\\.\PhysicalDrive3`,
		`  \\.\PhysicalDrive7`: `\\.\PhysicalDrive7`,
		`\\.\physicaldrive4`:   `\\.\PhysicalDrive4`,
	}
	for in, want := range ok {
		got, ok := normalizePhysicalDrivePath(in)
		if !ok || got != want {
			t.Errorf("normalizePhysicalDrivePath(%q) = (%q, %v), want (%q, true)", in, got, ok, want)
		}
	}

	bad := []string{
		"", "PhysicalDrive", "PhysicalDriveA", `\\.\PhysicalDrive1x`,
		`\\.\C:`, "/dev/sda", "/dev/disk/by-id/ata-foo",
		"serial:ABC123", "config.json",
	}
	for _, in := range bad {
		if got, ok := normalizePhysicalDrivePath(in); ok {
			t.Errorf("normalizePhysicalDrivePath(%q) = (%q, true), want ok=false", in, got)
		}
	}
}

func TestIsStableDiskID(t *testing.T) {
	stable := []string{
		"serial:S4XNX0A123", "guid:12345678-1234-1234-1234-123456789abc",
		"sig:deadbeef", "byid:wwn-0x5000c500a1b2c3d4",
	}
	for _, id := range stable {
		if !isStableDiskID(id) {
			t.Errorf("isStableDiskID(%q) = false, want true", id)
		}
	}
	weak := []string{
		"", "size:512000000000", "serial:", "byid:", "no-prefix", `\\.\PhysicalDrive0`, "/dev/sda",
	}
	for _, id := range weak {
		if isStableDiskID(id) {
			t.Errorf("isStableDiskID(%q) = true, want false", id)
		}
	}
}

// A set saved before identities were recorded runs exactly as it did: its
// devices are passed on untouched, whatever the listing says.
func TestResolveSetDisksWithoutIDsUnchanged(t *testing.T) {
	present := []PhysicalDiskInfo{
		disk(0, `\\.\PhysicalDrive0`, "serial:AAA"),
		disk(1, `\\.\PhysicalDrive1`, "serial:BBB"),
	}
	for _, devs := range [][]string{
		{`\\.\PhysicalDrive0`},
		{`\\.\PhysicalDrive1`, `\\.\PhysicalDrive0`},
		{`PhysicalDrive1`},
		{"/dev/sda", "/home/user/disk.img"},
	} {
		for _, ids := range [][]string{nil, {}, {""}, {"size:1234"}} {
			got, moved, err := resolveSetDisks(devs, ids, present)
			if err != nil || !sameList(got, devs) || len(moved) != 0 {
				t.Errorf("resolveSetDisks(%v, %v) = %v, %v, %v; want the devices unchanged", devs, ids, got, moved, err)
			}
		}
	}
}

// The point of it all: the system renumbered the disk, the run follows it.
func TestResolveSetDisksFollowsRenumberedDisk(t *testing.T) {
	// The set saved PhysicalDrive0; that disk is PhysicalDrive2 now and
	// another disk has number 0.
	present := []PhysicalDiskInfo{
		disk(2, `\\.\PhysicalDrive2`, "serial:AAA"),
		disk(0, `\\.\PhysicalDrive0`, "serial:BBB"),
	}
	got, moved, err := resolveSetDisks([]string{`\\.\PhysicalDrive0`}, []string{"serial:AAA"}, present)
	if err != nil {
		t.Fatalf("resolveSetDisks: %v", err)
	}
	if !sameList(got, []string{`\\.\PhysicalDrive2`}) {
		t.Errorf("got %v, want [%s]", got, `\\.\PhysicalDrive2`)
	}
	if len(moved) != 1 || !strings.Contains(moved[0], `\\.\PhysicalDrive0`) || !strings.Contains(moved[0], `\\.\PhysicalDrive2`) {
		t.Errorf("moved = %v, want one line naming both numbers", moved)
	}

	// Where it still is, nothing is reported as moved.
	got, moved, err = resolveSetDisks([]string{`\\.\PhysicalDrive0`}, []string{"serial:BBB"}, present)
	if err != nil || !sameList(got, []string{`\\.\PhysicalDrive0`}) || len(moved) != 0 {
		t.Errorf("unmoved disk: %v, %v, %v", got, moved, err)
	}
}

// A disk is found by any identity it has, so a set that recorded its GPT GUID
// (as the 1.2.0-rc app could) finds it when the serial is preferred now.
func TestResolveSetDisksByOtherID(t *testing.T) {
	present := []PhysicalDiskInfo{
		disk(1, `\\.\PhysicalDrive1`, "serial:AAA", "guid:0000-abc"),
		disk(0, `\\.\PhysicalDrive0`, "serial:BBB", "sig:deadbeef"),
	}
	got, _, err := resolveSetDisks([]string{`\\.\PhysicalDrive0`}, []string{"guid:0000-abc"}, present)
	if err != nil || !sameList(got, []string{`\\.\PhysicalDrive1`}) {
		t.Errorf("by GUID: %v, %v; want [\\\\.\\PhysicalDrive1]", got, err)
	}

	// Linux: a by-id name other than the preferred one.
	present = []PhysicalDiskInfo{
		disk(0, "/dev/sdb", "byid:wwn-0x5000c500a1b2c3d4", "byid:ata-ST2000_Z4Z0ABCD"),
	}
	got, moved, err := resolveSetDisks([]string{"/dev/sda"}, []string{"byid:ata-ST2000_Z4Z0ABCD"}, present)
	if err != nil || !sameList(got, []string{"/dev/sdb"}) || len(moved) != 1 {
		t.Errorf("Linux by-id: %v, %v, %v; want [/dev/sdb] reported as moved", got, moved, err)
	}
}

// A different disk at the saved number is never backed up in the set's place.
func TestResolveSetDisksRefusesWrongDisk(t *testing.T) {
	present := []PhysicalDiskInfo{
		disk(0, `\\.\PhysicalDrive0`, "serial:BBB"), // AAA is gone, BBB has its number
	}
	_, _, err := resolveSetDisks([]string{`\\.\PhysicalDrive0`}, []string{"serial:AAA"}, present)
	if err == nil {
		t.Fatal("must refuse to back up the disk that took a set's disk's number")
	}
	if !strings.Contains(err.Error(), "serial:AAA") || !strings.Contains(err.Error(), `\\.\PhysicalDrive0`) || !strings.Contains(err.Error(), "not connected") {
		t.Errorf("error %q should name the disk, its saved path and say it is not connected", err)
	}

	// Also when no disk has the number now.
	present = []PhysicalDiskInfo{disk(1, `\\.\PhysicalDrive1`, "serial:BBB")}
	if _, _, err := resolveSetDisks([]string{`\\.\PhysicalDrive0`}, []string{"serial:AAA"}, present); err == nil {
		t.Error("must fail when the set's disk is not among the disks present")
	}

	// One missing disk fails the whole run, even when another is found.
	present = []PhysicalDiskInfo{disk(1, `\\.\PhysicalDrive1`, "serial:BBB")}
	if _, _, err := resolveSetDisks([]string{`\\.\PhysicalDrive0`, `\\.\PhysicalDrive1`}, []string{"serial:AAA", "serial:BBB"}, present); err == nil {
		t.Error("must fail when any of the set's disks is missing")
	}
}

// Without a single identity in the listing (the disks could not be opened)
// nothing can be checked: the saved devices are used, as before.
func TestResolveSetDisksWithoutListingPassesThrough(t *testing.T) {
	for _, present := range [][]PhysicalDiskInfo{
		nil,
		{disk(0, `\\.\PhysicalDrive0`, "size:512000000000")},
	} {
		got, moved, err := resolveSetDisks([]string{`\\.\PhysicalDrive0`}, []string{"serial:AAA"}, present)
		if err != nil || !sameList(got, []string{`\\.\PhysicalDrive0`}) || len(moved) != 0 {
			t.Errorf("listing %v: %v, %v, %v; want the saved device", present, got, moved, err)
		}
	}
}

// A set mixing disks with and without identities, and two saved disks that
// are one disk now.
func TestResolveSetDisksMixedAndDuplicate(t *testing.T) {
	present := []PhysicalDiskInfo{
		disk(2, `\\.\PhysicalDrive2`, "serial:AAA"),
		disk(1, `\\.\PhysicalDrive1`, "serial:BBB"),
	}
	got, _, err := resolveSetDisks([]string{`\\.\PhysicalDrive0`, `\\.\PhysicalDrive1`}, []string{"serial:AAA"}, present)
	if err != nil || !sameList(got, []string{`\\.\PhysicalDrive2`, `\\.\PhysicalDrive1`}) {
		t.Errorf("mixed: %v, %v", got, err)
	}
	// AAA moved to number 2, which the set also saved without an identity.
	got, _, err = resolveSetDisks([]string{`\\.\PhysicalDrive0`, `\\.\PhysicalDrive2`}, []string{"serial:AAA", ""}, present)
	if err != nil || !sameList(got, []string{`\\.\PhysicalDrive2`}) {
		t.Errorf("duplicate: %v, %v; want the disk once", got, err)
	}
}

func TestPinSetDisks(t *testing.T) {
	present := []PhysicalDiskInfo{
		disk(0, `\\.\PhysicalDrive0`, "serial:AAA"),
		disk(1, `\\.\PhysicalDrive1`, "size:1234"), // too weak to record
		disk(2, `\\.\PhysicalDrive2`, "guid:abc"),
	}
	job := ScheduledJob{BackupType: "machine", DriveLetters: []string{`\\.\PhysicalDrive0`, `\\.\PhysicalDrive1`, `PhysicalDrive2`}}
	pinSetDisks(&job, present)
	if !sameList(job.DiskIDs, []string{"serial:AAA", "", "guid:abc"}) {
		t.Errorf("DiskIDs = %q", job.DiskIDs)
	}

	// The identity the editor sent wins: it is the disk the user ticked, even
	// when that disk is not connected and another one has its number.
	job = ScheduledJob{BackupType: "machine", DriveLetters: []string{`\\.\PhysicalDrive0`, `\\.\PhysicalDrive2`}, DiskIDs: []string{"serial:GONE"}}
	pinSetDisks(&job, present)
	if !sameList(job.DiskIDs, []string{"serial:GONE", "guid:abc"}) {
		t.Errorf("DiskIDs = %q, want the sent identity kept and the other filled in", job.DiskIDs)
	}

	// Nothing identified: no identities at all.
	job = ScheduledJob{BackupType: "machine", DriveLetters: []string{`\\.\PhysicalDrive1`}}
	pinSetDisks(&job, present)
	if job.DiskIDs != nil {
		t.Errorf("DiskIDs = %q, want none", job.DiskIDs)
	}
	job = ScheduledJob{BackupType: "machine", DriveLetters: []string{`\\.\PhysicalDrive0`}}
	pinSetDisks(&job, nil)
	if job.DiskIDs != nil {
		t.Errorf("no listing: DiskIDs = %q, want none", job.DiskIDs)
	}

	// A folder set never carries any.
	job = ScheduledJob{BackupType: "directory", BackupDirs: []string{`C:\Data`}, DiskIDs: []string{"serial:AAA"}}
	pinSetDisks(&job, present)
	if job.DiskIDs != nil {
		t.Errorf("folder set DiskIDs = %q, want none", job.DiskIDs)
	}
}

// An identity two disks report tells them apart from nothing: each disk falls
// back to its next one.
func TestDropSharedDiskIDs(t *testing.T) {
	disks := dropSharedDiskIDs([]PhysicalDiskInfo{
		disk(0, `\\.\PhysicalDrive0`, "serial:0123456789ABCDEF", "guid:aaa"),
		disk(1, `\\.\PhysicalDrive1`, "serial:0123456789ABCDEF", "sig:bbb"),
		disk(2, `\\.\PhysicalDrive2`, "serial:CCC", "guid:ccc"),
	})
	want := [][]string{{"guid:aaa"}, {"sig:bbb"}, {"serial:CCC", "guid:ccc"}}
	for i, d := range disks {
		if !sameList(diskIDsOf(d), want[i]) {
			t.Errorf("disk %d identities %q, want %q", i, diskIDsOf(d), want[i])
		}
	}
}

func TestSortByIDNames(t *testing.T) {
	got := sortByIDNames([]string{"usb-Generic_X", "nvme-eui.0025", "ata-ST2000_Z4Z0", "wwn-0x5000", "scsi-SATA_ST2000", "ata-AAA"})
	want := []string{"wwn-0x5000", "scsi-SATA_ST2000", "ata-AAA", "ata-ST2000_Z4Z0", "nvme-eui.0025", "usb-Generic_X"}
	if !sameList(got, want) {
		t.Errorf("sortByIDNames = %v, want %v", got, want)
	}
}

// A Backup Set run carries the set's identities to the machine backup: in
// the scheduler's job copy, and in a GUI's request to the service (a machine
// set stored as a VM is "machine-vm" there).
func TestRunDiskIDs(t *testing.T) {
	a := &App{}
	key := a.registerPendingPostActions(ScheduledJob{ID: "m", BackupType: "machine",
		DriveLetters: []string{`\\.\PhysicalDrive0`}, DiskIDs: []string{"serial:AAA"}}, "scheduled")
	if got := a.runDiskIDs(key); !sameList(got, []string{"serial:AAA"}) {
		t.Errorf("scheduler run: %q", got)
	}

	skey := a.RegisterRunOptions("machine-vm", "", false)
	if got := a.runDiskIDs(skey); got != nil {
		t.Errorf("request without identities: %q", got)
	}
	a.SetRunDiskIDs(skey, []string{"guid:abc"})
	if got := a.runDiskIDs(skey); !sameList(got, []string{"guid:abc"}) {
		t.Errorf("service request: %q", got)
	}

	// A one-off backup has none, and is backed up as given without a listing.
	if got := a.runDiskIDs(""); got != nil {
		t.Errorf("one-off: %q", got)
	}
	devs, err := a.resolveRunDisks([]string{`\\.\PhysicalDrive9`}, "")
	if err != nil || !sameList(devs, []string{`\\.\PhysicalDrive9`}) {
		t.Errorf("one-off resolveRunDisks = %v, %v", devs, err)
	}
}

// A set whose disk was found at a new number names it there afterwards,
// unless the set was edited meanwhile.
func TestKeepMovedDisks(t *testing.T) {
	deployTestDir(t)
	a := &App{}
	old := []string{`\\.\PhysicalDrive1`}
	writeJobs := func(jobs []ScheduledJob) {
		t.Helper()
		path, err := getScheduledJobsPath()
		if err != nil {
			t.Fatal(err)
		}
		data, _ := json.Marshal(jobs)
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	drives := func(id string) []string {
		t.Helper()
		jobs, err := a.GetScheduledJobs()
		if err != nil {
			t.Fatal(err)
		}
		for _, j := range jobs {
			if j.ID == id {
				return j.DriveLetters
			}
		}
		t.Fatalf("set %s gone", id)
		return nil
	}

	writeJobs([]ScheduledJob{
		{ID: "m", Name: "Machine", BackupType: "machine", DriveLetters: old},
		{ID: "e", Name: "Edited", BackupType: "machine", DriveLetters: []string{`\\.\PhysicalDrive3`}},
	})
	a.keepMovedDisks("m", old, []string{`\\.\PhysicalDrive2`})
	if got := drives("m"); !sameList(got, []string{`\\.\PhysicalDrive2`}) {
		t.Errorf("moved disk not saved: %v", got)
	}
	// "e" no longer has the paths the run started from: left alone.
	a.keepMovedDisks("e", old, []string{`\\.\PhysicalDrive2`})
	if got := drives("e"); !sameList(got, []string{`\\.\PhysicalDrive3`}) {
		t.Errorf("edited set changed: %v", got)
	}
}
