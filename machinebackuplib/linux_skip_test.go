//go:build linux

package machinebackuplib

import (
	"bytes"
	"io"
	"reflect"
	"strings"
	"sync"
	"testing"

	"pbscommon"
	"snapshot"
)

// A Linux partition left out needs no snapshot: a disk whose mounted
// partitions are all left out is read without one, so a set that skips them
// runs where no snapshot module is loaded.
func TestSkippedPartitionNotSnapshotted(t *testing.T) {
	parts := []diskPartition{
		{device: "/dev/sda1", number: 1, startByte: 1 * mib, endByte: 101 * mib, mountpoint: "/boot/efi", fstype: "vfat"},
		{device: "/dev/sda2", number: 2, startByte: 101 * mib, endByte: 601 * mib, mountpoint: "/", fstype: "ext4"},
		{device: "/dev/sda3", number: 3, startByte: 601 * mib, endByte: 1000 * mib, mountpoint: "/data", fstype: "xfs"},
	}
	if got := snapshotMountpoints(parts); !reflect.DeepEqual(got, []string{"/boot/efi", "/", "/data"}) {
		t.Fatalf("before: %q", got)
	}
	msgs := markSkippedLinux("/dev/sda", parts, []PartitionSkip{{Start: 601 * mib, Size: 399 * mib, Number: 3, Label: "/dev/sda3 (/data, xfs)"}})
	if !parts[2].skip || parts[0].skip || parts[1].skip {
		t.Fatalf("marked %+v", parts)
	}
	if len(msgs) != 1 || !strings.Contains(msgs[0], "leaving out /dev/sda3 (/data, xfs)") {
		t.Errorf("messages %q", msgs)
	}
	if got := snapshotMountpoints(parts); !reflect.DeepEqual(got, []string{"/boot/efi", "/"}) {
		t.Errorf("after: %q, want /data not snapshotted", got)
	}

	// Every mounted partition left out: nothing to snapshot, and the disk is
	// read straight away without asking for one (withSnapshots).
	for i := range parts {
		parts[i].skip = true
	}
	mps := snapshotMountpoints(parts)
	if len(mps) != 0 {
		t.Fatalf("all left out: %q", mps)
	}
	called := false
	if err := withSnapshots(mps, false, func(sn map[string]snapshot.SnapShot) error {
		called = true
		return nil
	}); err != nil || !called {
		t.Errorf("withSnapshots with every mounted partition left out = %v (read %v), want a read without a snapshot", err, called)
	}

	// A skip saved before the partition table changed: nothing left out, a
	// warning, and the partition is snapshotted as before.
	parts = []diskPartition{{device: "/dev/sda1", number: 1, startByte: 1 * mib, endByte: 1000 * mib, mountpoint: "/"}}
	msgs = markSkippedLinux("/dev/sda", parts, []PartitionSkip{{Start: 601 * mib, Size: 399 * mib, Label: "/dev/sda3"}})
	if parts[0].skip || len(msgs) != 1 || !strings.HasPrefix(msgs[0], "WARNING") || !strings.Contains(msgs[0], "backing it up") {
		t.Errorf("unmatched: %+v, %q", parts, msgs)
	}
	if got := snapshotMountpoints(parts); !reflect.DeepEqual(got, []string{"/"}) {
		t.Errorf("unmatched: snapshot %q", got)
	}
}

// watchReader is a disk that records every read, so a test can tell that a
// part of it was never read.
type watchReader struct {
	data  []byte
	mu    sync.Mutex
	reads [][2]int64
}

func (w *watchReader) ReadAt(p []byte, off int64) (int, error) {
	w.mu.Lock()
	w.reads = append(w.reads, [2]int64{off, off + int64(len(p))})
	w.mu.Unlock()
	if off >= int64(len(w.data)) {
		return 0, io.EOF
	}
	n := copy(p, w.data[off:])
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}

// A partition left out goes into the image as zeros, the image keeps its
// size and everything around it, and not one byte of it is read.
func TestSkippedPartitionZeroedNotRead(t *testing.T) {
	const total = 3*pbscommon.PBS_FIXED_CHUNK_SIZE + 12345
	const skipStart, skipEnd = pbscommon.PBS_FIXED_CHUNK_SIZE - 4096, 2*pbscommon.PBS_FIXED_CHUNK_SIZE + 8192
	disk := &watchReader{data: make([]byte, total)}
	for i := range disk.data {
		disk.data[i] = byte(i%251) + 1
	}
	parts := []diskPartition{
		{device: "/dev/sdx1", number: 1, startByte: 4096, endByte: skipStart},
		{device: "/dev/sdx2", number: 2, startByte: skipStart, endByte: skipEnd, skip: true},
		{device: "/dev/sdx3", number: 3, startByte: skipEnd, endByte: total},
	}
	segments := assembleSegments(total, diskRegions(parts, total, nil))
	var skipped int
	for _, s := range segments {
		if s.skip {
			skipped++
			if s.start != skipStart || s.end != skipEnd {
				t.Errorf("skip segment %+v", s)
			}
		}
	}
	if skipped != 1 {
		t.Fatalf("segments %+v, want one left out", segments)
	}

	want := bytes.Clone(disk.data)
	for i := skipStart; i < skipEnd; i++ {
		want[i] = 0
	}

	// The progress goes on through the partition left out, to the end of the
	// disk, and the partition is counted in LeftOut.
	jp := &JobProgress{}
	var fractions []float64
	var toSkipAtStart uint64
	progress := func(f float64, _ string) bool {
		if len(fractions) == 0 {
			toSkipAtStart = jp.ToSkip.Load()
		}
		fractions = append(fractions, f)
		return false
	}

	ch := make(chan []byte)
	errc := make(chan error, 1)
	go func() {
		err := copySegments(disk, "/dev/sdx", segments, total, ch, make(chan struct{}), progress, jp, false)
		close(ch)
		errc <- err
	}()
	got := make([]byte, 0, total)
	for chunk := range ch {
		got = append(got, chunk...)
	}
	if err := <-errc; err != nil {
		t.Fatalf("copySegments: %v", err)
	}
	if len(got) != total {
		t.Fatalf("image is %d bytes, want %d", len(got), total)
	}
	if !bytes.Equal(got, want) {
		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("byte %d = %#x, want %#x", i, got[i], want[i])
			}
		}
	}
	for _, r := range disk.reads {
		if r[0] < skipEnd && r[1] > skipStart {
			t.Errorf("read %d..%d touches the partition left out (%d..%d)", r[0], r[1], skipStart, skipEnd)
		}
	}
	if len(disk.reads) == 0 {
		t.Error("nothing read at all")
	}
	if got := jp.LeftOut.Load(); got != skipEnd-skipStart {
		t.Errorf("LeftOut = %d, want %d", got, skipEnd-skipStart)
	}
	// Known before the first block, so the progress counts it out from the start.
	if toSkipAtStart != skipEnd-skipStart {
		t.Errorf("ToSkip at the first block = %d, want %d", toSkipAtStart, skipEnd-skipStart)
	}
	if len(fractions) == 0 {
		t.Fatal("no progress reported")
	}
	for i := 1; i < len(fractions); i++ {
		if fractions[i] < fractions[i-1] {
			t.Fatalf("progress went back from %v to %v", fractions[i-1], fractions[i])
		}
	}
	// The last full chunk is reported; the short tail goes out after the
	// last report, so the reader ends within one chunk of the end.
	if last := fractions[len(fractions)-1]; last < float64(total-pbscommon.PBS_FIXED_CHUNK_SIZE)/float64(total) {
		t.Errorf("progress ended at %v, want the end of the disk", last)
	}
}
