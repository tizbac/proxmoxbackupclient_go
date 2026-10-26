package machinebackuplib

import (
	"reflect"
	"strings"
	"testing"
)

const mib = uint64(1) << 20

// checkPlan fails unless the plan reads the disk front to back exactly once:
// every section starts where the one before it ended, none runs backwards,
// and together they cover 0..total.
func checkPlan(t *testing.T, plan []Partition, total uint64) {
	t.Helper()
	var pos uint64
	for i, P := range plan {
		if P.StartByte != pos {
			t.Fatalf("section %d starts at %d, want %d:\n%+v", i, P.StartByte, pos, plan)
		}
		if P.EndByte < P.StartByte {
			t.Fatalf("section %d runs backwards, %d..%d:\n%+v", i, P.StartByte, P.EndByte, plan)
		}
		pos = P.EndByte
	}
	if pos != total {
		t.Fatalf("plan ends at %d, want %d:\n%+v", pos, total, plan)
	}
}

// Linux installed alongside Windows on an MBR disk: its root and swap are
// logical partitions in the space freed by shrinking C:, before the recovery
// partition at the end of the disk. Windows lists the primary partitions
// first, so this is the order it gave on a test disk laid out like that.
// Read in that order, the section before Linux started after it ended and
// the backup failed with "failed to read partition entirely".
func TestPlanDiskReadsDualBoot(t *testing.T) {
	total := 1024 * mib
	c := Partition{StartByte: 16 * mib, EndByte: 416 * mib, RequiresVSS: true, Letter: "C"}
	recovery := Partition{StartByte: 716 * mib, EndByte: 1016 * mib}
	root := Partition{StartByte: 416*mib + 64*1024, EndByte: 566 * mib}
	swap := Partition{StartByte: 566*mib + 128*1024, EndByte: 716 * mib}
	listed := []Partition{c, recovery, root, swap}

	plan, err := planDiskReads(listed, total)
	if err != nil {
		t.Fatal(err)
	}
	checkPlan(t, plan, total)
	want := []Partition{
		{StartByte: 0, EndByte: 16 * mib}, // MBR and the space before C:
		c,
		{StartByte: 416 * mib, EndByte: root.StartByte}, // extended partition's boot record
		root,
		{StartByte: 566 * mib, EndByte: swap.StartByte}, // next logical's boot record
		swap, recovery,
		{StartByte: 1016 * mib, EndByte: total},
	}
	if !reflect.DeepEqual(plan, want) {
		t.Errorf("plan\n%+v\nwant\n%+v", plan, want)
	}
	if !reflect.DeepEqual(listed, []Partition{c, recovery, root, swap}) {
		t.Error("the caller's partition list was reordered")
	}
}

// Partitions already in disk order give the same plan as before: raw sections
// only where there is space before, between or after them.
func TestPlanDiskReadsInOrder(t *testing.T) {
	total := 1000 * mib
	a := Partition{StartByte: 1 * mib, EndByte: 300 * mib}
	b := Partition{StartByte: 300 * mib, EndByte: 600 * mib, RequiresVSS: true, Letter: "D"}
	c := Partition{StartByte: 700 * mib, EndByte: 1000 * mib}

	plan, err := planDiskReads([]Partition{a, b, c}, total)
	if err != nil {
		t.Fatal(err)
	}
	checkPlan(t, plan, total)
	want := []Partition{{StartByte: 0, EndByte: 1 * mib}, a, b, {StartByte: 600 * mib, EndByte: 700 * mib}, c}
	if !reflect.DeepEqual(plan, want) {
		t.Errorf("plan\n%+v\nwant\n%+v", plan, want)
	}
}

func TestPlanDiskReadsEdges(t *testing.T) {
	total := 64 * mib

	plan, err := planDiskReads(nil, total)
	if err != nil {
		t.Fatal(err)
	}
	if want := []Partition{{StartByte: 0, EndByte: total}}; !reflect.DeepEqual(plan, want) {
		t.Errorf("no partitions: plan %+v, want the whole disk raw", plan)
	}

	whole := Partition{StartByte: 0, EndByte: total}
	plan, err = planDiskReads([]Partition{whole}, total)
	if err != nil {
		t.Fatal(err)
	}
	if want := []Partition{whole}; !reflect.DeepEqual(plan, want) {
		t.Errorf("one partition filling the disk: plan %+v, want just that partition", plan)
	}
}

// Layouts that cannot be read once, front to back, are refused before
// anything is read, instead of failing part way or reading data twice.
func TestPlanDiskReadsRefused(t *testing.T) {
	total := 100 * mib
	for _, tc := range []struct {
		name  string
		parts []Partition
		want  string
	}{
		{"overlap", []Partition{{StartByte: 1 * mib, EndByte: 50 * mib}, {StartByte: 40 * mib, EndByte: 90 * mib}}, "overlaps"},
		{"past the end", []Partition{{StartByte: 1 * mib, EndByte: 101 * mib}}, "past the end of the disk"},
		{"backwards", []Partition{{StartByte: 50 * mib, EndByte: 10 * mib}}, "ends before it starts"},
	} {
		_, err := planDiskReads(tc.parts, total)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error %v, want one saying %q", tc.name, err, tc.want)
		}
	}
}
