package main

import (
	"testing"

	"pbscommon"
)

// The full-read counter moves only on a successful run: a cancelled or failed
// run leaves it alone, a full read (scheduled, data mode, or no usable
// previous snapshot) resets it, and a metadata run that reused files advances
// it.
func TestNextFullReadCount(t *testing.T) {
	meta := ScheduledJob{BackupType: "directory", ChangeDetectionMode: pbscommon.ChangeDetectionMetadata, FullReadEvery: 7, MetadataRunsSinceFullRead: 3}
	data := meta
	data.ChangeDetectionMode = pbscommon.ChangeDetectionData // the run copy of a scheduled full read
	machine := ScheduledJob{BackupType: "machine", MetadataRunsSinceFullRead: 2}
	cases := []struct {
		name          string
		job           ScheduledJob
		success, read bool
		want          int
	}{
		{"metadata run reusing files", meta, true, false, 4},
		{"metadata run without a usable previous snapshot", meta, true, true, 0},
		{"cancelled or failed metadata run", meta, false, false, 3},
		{"scheduled full read completed", data, true, true, 0},
		{"scheduled full read stopped", data, false, true, 3},
		{"machine set", machine, true, false, 2},
	}
	for _, c := range cases {
		if got := nextFullReadCount(c.job, c.success, c.read); got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, got, c.want)
		}
	}
}

func TestIsCancelledRun(t *testing.T) {
	if !isCancelledRun("anything", MsgBackupCancelled) {
		t.Error("the cancelled message key was not recognised")
	}
	if !isCancelledRun("backup cancelled by user", "") {
		t.Error("the cancelled error text was not recognised")
	}
	if isCancelledRun("PBS session lost", MsgSessionLost) {
		t.Error("a failure was taken for a cancel")
	}
}

// Sizes carry both units, decimal first, like the GUI's speeds.
func TestFormatByteSizeBothUnits(t *testing.T) {
	cases := map[uint64]string{
		512:          "512 B",
		480850777117: "480.9 GB (447.8 GiB)",
		8589934592:   "8.6 GB (8.0 GiB)",
		1048576:      "1.0 MB (1.0 MiB)",
	}
	for in, want := range cases {
		if got := formatByteSize(in); got != want {
			t.Errorf("formatByteSize(%d) = %q, want %q", in, got, want)
		}
	}
}

// Until every background scan has finished, the totals fall back to the
// previous snapshot's when those are larger; afterwards the scans win.
func TestJobProgressTotals(t *testing.T) {
	p := &jobProgress{}
	p.scansWanted.Store(1)
	if b, f := p.totals(); b != 0 || f != 0 {
		t.Fatalf("empty totals = %d, %d", b, f)
	}
	p.prevBytes.Store(1000)
	p.prevFiles.Store(10)
	if b, f := p.totals(); b != 1000 || f != 10 {
		t.Errorf("before the scan: %d, %d, want the previous snapshot's 1000, 10", b, f)
	}
	p.sizeEstimate.Store(900)
	p.filesEstimate.Store(9)
	p.scansDone.Store(1)
	if b, f := p.totals(); b != 900 || f != 9 {
		t.Errorf("after the scan: %d, %d, want the scan's 900, 9", b, f)
	}
}
