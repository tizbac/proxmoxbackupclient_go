package main

import "testing"

func TestMachineSnapshotType(t *testing.T) {
	for in, want := range map[string]string{
		"machine":    "host",
		"":           "host",
		"directory":  "host",
		"machine-vm": "vm",
	} {
		if got := machineSnapshotType(in); got != want {
			t.Errorf("machineSnapshotType(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestScheduledMachineBackupType(t *testing.T) {
	cases := []struct {
		name string
		job  ScheduledJob
		want string
	}{
		{"host set", ScheduledJob{BackupType: "machine"}, "machine"},
		{"vm set", ScheduledJob{BackupType: "machine", MachineAsVM: true}, "machine-vm"},
		{"directory ignores flag", ScheduledJob{BackupType: "directory", MachineAsVM: true}, "directory"},
	}
	for _, c := range cases {
		if got := scheduledMachineBackupType(c.job); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
	if machineSnapshotType(scheduledMachineBackupType(ScheduledJob{BackupType: "machine", MachineAsVM: true})) != "vm" {
		t.Error("vm set must resolve to PBS type vm")
	}
	if machineSnapshotType(scheduledMachineBackupType(ScheduledJob{BackupType: "machine"})) != "host" {
		t.Error("plain machine set must stay host")
	}
}

func TestValidateScheduledJob(t *testing.T) {
	ok := []ScheduledJob{
		{BackupType: "machine", BackupID: "myhost"},
		{BackupType: "machine", MachineAsVM: true, BackupID: "9002929"},
		{BackupType: "machine", MachineAsVM: true, BackupID: "0002929"},
		{BackupType: "directory", MachineAsVM: true, BackupID: "name"},
	}
	for _, j := range ok {
		if err := validateScheduledJob(j); err != nil {
			t.Errorf("%+v rejected: %v", j, err)
		}
	}
	for _, id := range []string{"", "myhost", "12ab", "9002929 "} {
		if err := validateScheduledJob(ScheduledJob{BackupType: "machine", MachineAsVM: true, BackupID: id}); err == nil {
			t.Errorf("vm set with ID %q accepted", id)
		}
	}
}
