package machinebackuplib

import (
	"testing"

	"pbscommon"
)

const mib4 = pbscommon.PBS_FIXED_CHUNK_SIZE

// skippable counts the reader's 4 MiB windows with no used cluster, and
// nothing in a window that has one.
func TestUsedMapSkippable(t *testing.T) {
	// 4 KiB clusters, 40 MiB volume: all free but one cluster in window 2
	// and the whole of window 5.
	const cluster = 4096
	m := &usedMap{clusterSize: cluster, clusters: 10 * mib4 / cluster, bits: make([]byte, 10*mib4/cluster/8)}
	m.markUsed(2*mib4/cluster+7, 1)
	m.markUsed(5*mib4/cluster, mib4/cluster)
	if got := m.skippable(10 * mib4); got != 8*mib4 {
		t.Errorf("skippable = %d MiB, want 32", got>>20)
	}
	// A partition longer than the volume: past the map counts as used.
	if got := m.skippable(10*mib4 + 1000); got != 8*mib4 {
		t.Errorf("longer partition: %d MiB, want 32", got>>20)
	}
	// A short last window that is free.
	if got := m.skippable(mib4 + 512); got != mib4+512 {
		t.Errorf("short tail: %d, want %d", got, mib4+512)
	}
}

// ReadProgress leaves out what goes in unread, never goes back, and counts
// a disk whole until its free space is known.
func TestReadProgress(t *testing.T) {
	var p JobProgress
	p.BytesTotal.Store(100 * mib4)
	if done, total := p.ReadProgress(); done != 0 || total != 100*mib4 {
		t.Fatalf("before the map: %d / %d", done, total)
	}
	p.addToSkip(80 * mib4) // 80 of 100 blocks free or left out
	p.BytesDone.Store(10 * mib4)
	p.FreeSkipped.Store(6 * mib4)
	if done, total := p.ReadProgress(); done != 4*mib4 || total != 20*mib4 {
		t.Errorf("%d / %d blocks, want 4 / 20", done/mib4, total/mib4)
	}
	// A skipped window already in BytesDone, not yet in FreeSkipped: 5,
	// and it stays 5 once FreeSkipped catches up.
	p.BytesDone.Store(11 * mib4)
	if done, _ := p.ReadProgress(); done != 5*mib4 {
		t.Errorf("mid-window: %d blocks, want 5", done/mib4)
	}
	p.FreeSkipped.Store(7 * mib4)
	if done, _ := p.ReadProgress(); done != 5*mib4 {
		t.Errorf("went back to %d blocks", done/mib4)
	}
	// The end: everything read, partitions left out counted too.
	p.BytesDone.Store(100 * mib4)
	p.FreeSkipped.Store(70 * mib4)
	p.LeftOut.Store(10 * mib4)
	if done, total := p.ReadProgress(); done != total || total != 20*mib4 {
		t.Errorf("end: %d / %d blocks", done/mib4, total/mib4)
	}
	if f, ok := p.readFraction(); !ok || f != 1 {
		t.Errorf("fraction %v %v", f, ok)
	}
	p.Reset()
	if done, total := p.ReadProgress(); done != 0 || total != 0 || p.ToSkip.Load() != 0 {
		t.Errorf("after Reset: %d / %d, to skip %d", done, total, p.ToSkip.Load())
	}
	var nilP *JobProgress
	if _, ok := nilP.readFraction(); ok {
		t.Error("nil progress has a fraction")
	}
	nilP.addToSkip(1)
}
