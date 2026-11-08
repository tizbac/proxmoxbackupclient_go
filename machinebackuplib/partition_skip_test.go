package machinebackuplib

import (
	"reflect"
	"strings"
	"testing"

	"pbscommon"
)

// A skip leaves out the partition that starts where it says and is as long
// as it says, and nothing else: not the space between partitions, and not a
// partition that has moved or changed size since the set was saved.
func TestSkipsMatch(t *testing.T) {
	parts := []partRange{
		{start: 0, end: 1 * mib},                    // space before the first partition
		{start: 1 * mib, end: 101 * mib, number: 1}, // EFI
		{start: 101 * mib, end: 601 * mib, number: 2},
		{start: 601 * mib, end: 1000 * mib, number: 3},
	}

	skip, warn := skipsMatch("disk", parts, []PartitionSkip{{Start: 101 * mib, Size: 500 * mib, Number: 2, Label: "D: (NTFS)"}})
	if !reflect.DeepEqual(skip, []bool{false, false, true, false}) || len(warn) != 0 {
		t.Errorf("exact match: skip %v, warnings %q", skip, warn)
	}

	// The partition was resized, or another starts there now: backed up, with
	// a warning that names it.
	for _, s := range []PartitionSkip{
		{Start: 101 * mib, Size: 400 * mib, Label: "D: (NTFS)"},
		{Start: 102 * mib, Size: 500 * mib, Label: "D: (NTFS)"},
	} {
		skip, warn = skipsMatch("disk", parts, []PartitionSkip{s})
		if !reflect.DeepEqual(skip, []bool{false, false, false, false}) {
			t.Errorf("%+v: skip %v, want nothing left out", s, skip)
		}
		if len(warn) != 1 || !strings.Contains(warn[0], "WARNING") || !strings.Contains(warn[0], "D: (NTFS)") || !strings.Contains(warn[0], "backing it up") {
			t.Errorf("%+v: warnings %q", s, warn)
		}
	}

	// Never the space between partitions, even where it starts.
	skip, warn = skipsMatch("disk", []partRange{{start: 4096, end: 1 * mib}, {start: 1 * mib, end: 2 * mib, number: 1}}, []PartitionSkip{{Start: 4096}})
	if skip[0] || skip[1] || len(warn) != 1 {
		t.Errorf("gap: skip %v, warnings %q", skip, warn)
	}

	// By number, as -skippartition gives it.
	skip, warn = skipsMatch("disk", parts, []PartitionSkip{{Number: 3}, {Number: 9}})
	if !reflect.DeepEqual(skip, []bool{false, false, false, true}) {
		t.Errorf("by number: skip %v", skip)
	}
	if len(warn) != 1 || !strings.Contains(warn[0], "no partition 9") {
		t.Errorf("by number: warnings %q", warn)
	}

	// No skips: nothing left out, nothing said.
	skip, warn = skipsMatch("disk", parts, nil)
	if !reflect.DeepEqual(skip, []bool{false, false, false, false}) || warn != nil {
		t.Errorf("no skips: %v, %q", skip, warn)
	}
}

// A Windows partition left out is not snapshotted: its drive letter is taken
// off the VSS set, and the plan keeps it marked so it is written as zeros.
func TestMarkSkippedWindows(t *testing.T) {
	total := 1000 * mib
	efi := Partition{StartByte: 1 * mib, EndByte: 101 * mib, Number: 1}
	c := Partition{StartByte: 101 * mib, EndByte: 601 * mib, Number: 2, RequiresVSS: true, Letter: "C", FileSystem: "NTFS"}
	d := Partition{StartByte: 601 * mib, EndByte: 1000 * mib, Number: 3, RequiresVSS: true, Letter: "D", FileSystem: "NTFS"}
	parts := []Partition{efi, c, d}

	if got := vssPaths(parts); !reflect.DeepEqual(got, []string{`C:\\`, `D:\\`}) {
		t.Fatalf("before: vssPaths = %q", got)
	}
	msgs := markSkipped(`\\.\PhysicalDrive1`, parts, []PartitionSkip{{Start: d.StartByte, Size: d.EndByte - d.StartByte, Number: 3, Label: "D: (NTFS)"}})
	if !parts[2].Skip || parts[2].RequiresVSS || parts[0].Skip || parts[1].Skip {
		t.Fatalf("marked %+v", parts)
	}
	if got := vssPaths(parts); !reflect.DeepEqual(got, []string{`C:\\`}) {
		t.Errorf("after: vssPaths = %q, want C: only", got)
	}
	if len(msgs) != 1 || !strings.Contains(msgs[0], "leaving out D: (NTFS)") || !strings.Contains(msgs[0], "zeros") {
		t.Errorf("messages %q", msgs)
	}
	if got := parts[2].label(); got != "D: (NTFS), left out" {
		t.Errorf("progress card label %q", got)
	}

	plan, err := planDiskReads(parts, total)
	if err != nil {
		t.Fatal(err)
	}
	checkPlan(t, plan, total)
	if !plan[len(plan)-1].Skip || plan[1].Skip || plan[2].Skip {
		t.Errorf("plan lost the mark: %+v", plan)
	}

	// A skip that matches nothing changes nothing and is reported.
	parts = []Partition{efi, c, d}
	msgs = markSkipped(`\\.\PhysicalDrive1`, parts, []PartitionSkip{{Start: 2 * mib, Size: mib, Label: "partition 7"}})
	if !reflect.DeepEqual(parts, []Partition{efi, c, d}) {
		t.Errorf("unmatched skip changed the partitions: %+v", parts)
	}
	if len(msgs) != 1 || !strings.HasPrefix(msgs[0], "WARNING") || !strings.Contains(msgs[0], "partition 7") {
		t.Errorf("unmatched: messages %q", msgs)
	}
}

// A Windows disk with nothing left to snapshot says so, instead of
// "Creating VSS snapshot..." for a snapshot it never takes.
func TestSnapshotStep(t *testing.T) {
	efi := Partition{StartByte: 1 * mib, EndByte: 101 * mib, Number: 1}
	d := Partition{StartByte: 101 * mib, EndByte: 1000 * mib, Number: 2, RequiresVSS: true, Letter: "D", FileSystem: "NTFS"}
	const dev = `\\.\PhysicalDrive1`

	if got := snapshotStep(dev, []Partition{efi, d}); got != "Creating VSS snapshot..." {
		t.Errorf("a lettered volume backed up: %q", got)
	}
	parts := []Partition{efi, d}
	markSkipped(dev, parts, []PartitionSkip{{Number: 2}})
	got := snapshotStep(dev, parts)
	if strings.Contains(got, "VSS") || !strings.Contains(got, "no snapshot needed") || !strings.Contains(got, "left out") {
		t.Errorf("every lettered volume left out: %q", got)
	}
	got = snapshotStep(dev, []Partition{efi, {StartByte: 101 * mib, EndByte: 1000 * mib, Number: 2}})
	if strings.Contains(got, "VSS") || !strings.Contains(got, "no volume of this disk has a drive letter") {
		t.Errorf("no lettered volume: %q", got)
	}
}

// A partition left out is written as zeros in 4 MiB pieces, each counted in
// LeftOut as it goes, and the zeros add up to the partition, so the read
// position (and the progress bar) gets to its end.
func TestLeaveOutCounts(t *testing.T) {
	const length = 2*pbscommon.PBS_FIXED_CHUNK_SIZE + 12345
	jp := &JobProgress{}
	var written uint64
	var seen []uint64
	ok := leaveOut(length, func(n uint64) bool {
		if n > pbscommon.PBS_FIXED_CHUNK_SIZE {
			t.Errorf("piece of %d bytes, want at most 4 MiB", n)
		}
		written += n
		seen = append(seen, jp.LeftOut.Load())
		return true
	}, jp)
	if !ok || written != length || jp.LeftOut.Load() != length {
		t.Fatalf("ok %v, written %d, LeftOut %d, want %d", ok, written, jp.LeftOut.Load(), length)
	}
	if !reflect.DeepEqual(seen, []uint64{0, pbscommon.PBS_FIXED_CHUNK_SIZE, 2 * pbscommon.PBS_FIXED_CHUNK_SIZE}) {
		t.Errorf("LeftOut while writing %v: want it counted piece by piece", seen)
	}

	// Stopped part way: false, and only what went out is counted.
	jp = &JobProgress{}
	calls := 0
	if leaveOut(length, func(n uint64) bool { calls++; return calls < 2 }, jp) {
		t.Error("leaveOut went on after emitZeros stopped")
	}
	if jp.LeftOut.Load() != pbscommon.PBS_FIXED_CHUNK_SIZE {
		t.Errorf("LeftOut after a stop = %d, want one piece", jp.LeftOut.Load())
	}

	// No progress to keep (the CLI tests): no panic.
	if !leaveOut(length, func(uint64) bool { return true }, nil) {
		t.Error("nil JobProgress")
	}
}

func TestSkipsForAndParse(t *testing.T) {
	all := map[string][]PartitionSkip{`\\.\PHYSICALDRIVE1`: {{Number: 2}}, "/dev/sda": {{Number: 3}}}
	if got := skipsFor(all, `\\.\PhysicalDrive1`); len(got) != 1 || got[0].Number != 2 {
		t.Errorf("Windows disk in another case: %+v", got)
	}
	if got := skipsFor(all, "/dev/sda"); len(got) != 1 || got[0].Number != 3 {
		t.Errorf("/dev/sda: %+v", got)
	}
	if got := skipsFor(all, `\\.\PhysicalDrive11`); got != nil {
		t.Errorf("another disk: %+v", got)
	}

	for in, want := range map[string]string{`\\.\PhysicalDrive0:4`: `\\.\PhysicalDrive0`, "/dev/sda:2": "/dev/sda", "/dev/nvme0n1:12": "/dev/nvme0n1"} {
		dev, s, err := ParseSkipPartition(in)
		if err != nil || dev != want || s.Number == 0 || s.Start != 0 {
			t.Errorf("ParseSkipPartition(%q) = %q, %+v, %v", in, dev, s, err)
		}
	}
	for _, in := range []string{"/dev/sda", "/dev/sda:", ":2", "/dev/sda:0", "/dev/sda:x", "/dev/sda:-1"} {
		if _, _, err := ParseSkipPartition(in); err == nil {
			t.Errorf("ParseSkipPartition(%q) accepted", in)
		}
	}
}

func TestGUIDString(t *testing.T) {
	// The EFI system partition type as a GPT entry stores it.
	b := []byte{0x28, 0x73, 0x2A, 0xC1, 0x1F, 0xF8, 0xD2, 0x11, 0xBA, 0x4B, 0x00, 0xA0, 0xC9, 0x3E, 0xC9, 0x3B}
	if got := guidString(b); got != efiSystemGUID {
		t.Errorf("guidString = %s, want %s", got, efiSystemGUID)
	}
}
