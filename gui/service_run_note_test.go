package main

import (
	"testing"
	"time"

	"pbscommon"
)

func TestNewestSnapshotSince(t *testing.T) {
	start := time.Unix(1_800_000_000, 0)
	snaps := []pbscommon.BackupManifest{
		{BackupTime: start.Add(-2 * time.Hour).Unix()},     // an earlier run
		{BackupTime: start.Add(-30 * time.Second).Unix()},  // this run, server clock 30 s behind
		{BackupTime: start.Add(-90 * time.Second).Unix()},  // too early even with the leeway
	}
	got := newestSnapshotSince(snaps, start)
	if got == nil || got.BackupTime != start.Add(-30*time.Second).Unix() {
		t.Fatalf("got %+v, want the snapshot from 30 s before the start", got)
	}
	if newestSnapshotSince(snaps[:1], start) != nil {
		t.Fatal("an earlier run's snapshot was taken for this run's")
	}
}

func TestSnapshotGroup(t *testing.T) {
	cases := []struct {
		job          ScheduledJob
		typ, wantID  string
	}{
		{ScheduledJob{BackupType: "directory", BackupID: "pc-data"}, "host", "pc-data"},
		{ScheduledJob{BackupType: "machine", BackupID: "pc"}, "host", "pc"},
		{ScheduledJob{BackupType: "machine", MachineAsVM: true, BackupID: "9000107"}, "vm", "9000107"},
	}
	for _, c := range cases {
		typ, id := snapshotGroup(c.job)
		if typ != c.typ || id != c.wantID {
			t.Errorf("%+v: got %s/%s, want %s/%s", c.job, typ, id, c.typ, c.wantID)
		}
	}
	if _, id := snapshotGroup(ScheduledJob{BackupType: "directory"}); id == "" {
		t.Error("an empty Backup ID should fall back to the computer's name")
	}
}
