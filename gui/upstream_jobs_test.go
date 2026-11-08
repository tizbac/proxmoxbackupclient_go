package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// A scheduled_jobs.json written by the project's 1.2.0-rc app: daily, weekly
// (Tuesday and Thursday, 0 = Sunday) and monthly sets, one on a named PBS
// server and one machine set stored as a VM.
const upstreamJobsFile = `[
 {"id":"d","name":"Daily docs","scheduleTime":"21:30","runAtStartup":false,"backupDirs":["C:\\Docs"],"driveLetters":null,"backupId":"pc","useVSS":true,"backupType":"directory","excludeList":null,"compression":"default","enabled":true},
 {"id":"w","name":"Weekly","scheduleTime":"10:00","frequency":"weekly","weekdays":[2,4],"runAtStartup":false,"backupDirs":["C:\\Data"],"driveLetters":null,"backupId":"pc-data","useVSS":true,"backupType":"directory","excludeList":null,"compression":"default","pbs_id":"pbs-office","enabled":true},
 {"id":"m","name":"Monthly VM","scheduleTime":"02:00","frequency":"monthly","dayOfMonth":31,"runAtStartup":false,"backupDirs":null,"driveLetters":["\\\\.\\PhysicalDrive0"],"backupId":"101","useVSS":true,"backupType":"machine","excludeList":null,"compression":"default","backup_kind":"vm","enabled":false}
]`

func loadUpstreamJobs(t *testing.T) map[string]ScheduledJob {
	t.Helper()
	var jobs []ScheduledJob
	if err := json.Unmarshal([]byte(upstreamJobsFile), &jobs); err != nil {
		t.Fatal(err)
	}
	byID := map[string]ScheduledJob{}
	for i := range jobs {
		normalizeJobSchedules(&jobs[i])
		byID[jobs[i].ID] = jobs[i]
	}
	return byID
}

func TestMigrateUpstreamJobs(t *testing.T) {
	jobs := loadUpstreamJobs(t)

	d := jobs["d"]
	if d.ScheduleMode != ScheduleModeScheduled || len(d.Schedules) != 1 || d.Schedules[0].Unit != "days" || d.Schedules[0].At != "21:30" {
		t.Errorf("daily set: mode %q schedules %+v", d.ScheduleMode, d.Schedules)
	}

	w := jobs["w"]
	if w.PBSServerID != "pbs-office" {
		t.Errorf("weekly set server = %q, want pbs-office", w.PBSServerID)
	}
	if len(w.Schedules) != 1 || w.Schedules[0].Unit != "weeks" || w.Schedules[0].At != "10:00" ||
		strings.Join(w.Schedules[0].Weekdays, ",") != "Tue,Thu" {
		t.Errorf("weekly set schedules = %+v, want weeks at 10:00 on Tue,Thu", w.Schedules)
	}
	// From Monday 2 November 2026 09:00 the next run is Tuesday at 10:00.
	setNextRun(&w, time.Date(2026, 11, 2, 9, 0, 0, 0, time.Local))
	next, err := time.Parse(time.RFC3339, w.NextRun)
	if err != nil {
		t.Fatalf("next run %q: %v", w.NextRun, err)
	}
	if next = next.In(time.Local); next.Weekday() != time.Tuesday || next.Hour() != 10 || next.Minute() != 0 {
		t.Errorf("weekly next run = %v, want Tuesday 10:00", next)
	}

	m := jobs["m"]
	if !m.MachineAsVM || m.BackupID != "101" || m.Enabled {
		t.Errorf("monthly machine set: asVM %v id %q enabled %v", m.MachineAsVM, m.BackupID, m.Enabled)
	}
	if len(m.Schedules) != 1 || m.Schedules[0].Unit != "months" || m.Schedules[0].MonthMode != "day" ||
		m.Schedules[0].MonthDay != 31 || m.Schedules[0].At != "02:00" {
		t.Errorf("monthly set schedules = %+v, want months on day 31 at 02:00", m.Schedules)
	}
}

// Saved again, a migrated set has none of the upstream keys left.
func TestMigratedUpstreamJobSavesWithoutUpstreamKeys(t *testing.T) {
	jobs := loadUpstreamJobs(t)
	for id, j := range jobs {
		data, err := json.Marshal(j)
		if err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{`"frequency"`, `"weekdays":[`, `"dayOfMonth"`, `"pbs_id"`, `"backup_kind"`} {
			// "weekdays" also appears inside a schedule; the job-level key is
			// an array of numbers.
			if key == `"weekdays":[` {
				if strings.Contains(string(data), `"weekdays":[2`) {
					t.Errorf("set %s still has upstream weekdays: %s", id, data)
				}
				continue
			}
			if strings.Contains(string(data), key) {
				t.Errorf("set %s still has %s: %s", id, key, data)
			}
		}
	}
}

// The 1.2.0-rc app's disk table (config.json pinned_disks) gives its machine
// sets the identities of their disks; Linux sets of that app name their disks
// by /dev/disk/by-id paths.
func TestMigrateUpstreamDiskPins(t *testing.T) {
	var cfg Config
	if err := json.Unmarshal([]byte(`{"pinned_disks":{
		"serial:S4XNX0A123": "\\\\.\\PhysicalDrive0",
		"guid:0bd1d3a2-5c1e-4f3a-9a52-1c2d3e4f5a6b": "PhysicalDrive1",
		"sig:1a2b3c4d": "\\\\.\\PhysicalDrive3",
		"serial:OTHER": "\\\\.\\PhysicalDrive3",
		"size:512110190592": "\\\\.\\PhysicalDrive4"}}`), &cfg); err != nil {
		t.Fatal(err)
	}
	pins := cfg.UpstreamPinnedDisks

	jobs := loadUpstreamJobs(t)
	m := jobs["m"]
	migrateUpstreamDiskPins(&m, pins)
	if !sameList(m.DiskIDs, []string{"serial:S4XNX0A123"}) {
		t.Errorf("monthly machine set DiskIDs = %q", m.DiskIDs)
	}

	// Any spelling of the path; a path two disks claim and a weak id give
	// none, and so does a path nothing was pinned to.
	j := ScheduledJob{BackupType: "machine", DriveLetters: []string{`\\.\physicaldrive1`, `\\.\PhysicalDrive3`, `\\.\PhysicalDrive4`, `\\.\PhysicalDrive5`}}
	migrateUpstreamDiskPins(&j, pins)
	if !sameList(j.DiskIDs, []string{"guid:0bd1d3a2-5c1e-4f3a-9a52-1c2d3e4f5a6b", "", "", ""}) {
		t.Errorf("DiskIDs = %q", j.DiskIDs)
	}
	j = ScheduledJob{BackupType: "machine", DriveLetters: []string{`\\.\PhysicalDrive3`}}
	migrateUpstreamDiskPins(&j, pins)
	if j.DiskIDs != nil {
		t.Errorf("only an ambiguous path: DiskIDs = %q, want none", j.DiskIDs)
	}

	// Linux: the by-id path is the identity.
	j = ScheduledJob{BackupType: "machine", DriveLetters: []string{"/dev/disk/by-id/wwn-0x5000c500a1b2c3d4", "/dev/sdc"}}
	migrateUpstreamDiskPins(&j, nil)
	if !sameList(j.DiskIDs, []string{"byid:wwn-0x5000c500a1b2c3d4", ""}) {
		t.Errorf("Linux DiskIDs = %q", j.DiskIDs)
	}

	// A set with identities of its own, and a folder set, are left alone.
	j = ScheduledJob{BackupType: "machine", DriveLetters: []string{`\\.\PhysicalDrive0`}, DiskIDs: []string{"serial:MINE"}}
	migrateUpstreamDiskPins(&j, pins)
	if !sameList(j.DiskIDs, []string{"serial:MINE"}) {
		t.Errorf("set with identities changed: %q", j.DiskIDs)
	}
	d := jobs["d"]
	migrateUpstreamDiskPins(&d, pins)
	if d.DiskIDs != nil {
		t.Errorf("folder set DiskIDs = %q", d.DiskIDs)
	}

	// The table stays in config.json when this app saves it.
	data, err := json.Marshal(&cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"pinned_disks":{`) || !strings.Contains(string(data), "S4XNX0A123") {
		t.Errorf("config saved without pinned_disks: %s", data)
	}
}

// A migrated machine set runs on the disk the 1.2.0-rc app pinned, wherever
// that disk is now.
func TestMigratedUpstreamSetFollowsItsDisk(t *testing.T) {
	jobs := loadUpstreamJobs(t)
	m := jobs["m"]
	migrateUpstreamDiskPins(&m, map[string]string{"guid:abc": `\\.\PhysicalDrive0`})
	present := []PhysicalDiskInfo{
		disk(0, `\\.\PhysicalDrive0`, "serial:NEWDISK"),
		disk(1, `\\.\PhysicalDrive1`, "serial:S4XNX0A123", "guid:abc"),
	}
	got, moved, err := resolveSetDisks(m.DriveLetters, m.DiskIDs, present)
	if err != nil || !sameList(got, []string{`\\.\PhysicalDrive1`}) || len(moved) != 1 {
		t.Errorf("resolveSetDisks = %v, %v, %v; want [\\\\.\\PhysicalDrive1]", got, moved, err)
	}
}

// A set that already has schedules is not touched by the upstream keys.
func TestMigrateUpstreamJobLeavesScheduledSetAlone(t *testing.T) {
	j := ScheduledJob{ID: "x", ScheduleTime: "08:00", ScheduleMode: ScheduleModeScheduled,
		Schedules:         []Schedule{{ID: "s1", Enabled: true, Unit: "hours", Every: 2}},
		UpstreamFrequency: "weekly", UpstreamWeekdays: []int{1}, PBSServerID: "mine", UpstreamPBSID: "other"}
	normalizeJobSchedules(&j)
	if len(j.Schedules) != 1 || j.Schedules[0].Unit != "hours" || j.PBSServerID != "mine" {
		t.Errorf("scheduled set changed: server %q schedules %+v", j.PBSServerID, j.Schedules)
	}
}
