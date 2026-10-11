package machinebackuplib

import (
	"testing"

	"pbscommon"
)

func TestBlocksAndBlockMessage(t *testing.T) {
	const mib4 = pbscommon.PBS_FIXED_CHUNK_SIZE
	// 128,035,676,160 bytes: the BIWIN SSD on new-laptop, as PBS counted it
	// ("unexpected chunk count (30527 != 734)").
	if got := Blocks(128035676160); got != 30527 {
		t.Fatalf("Blocks(128 GB) = %d, want 30527", got)
	}
	if got := Blocks(0); got != 0 {
		t.Fatalf("Blocks(0) = %d, want 0", got)
	}
	cases := []struct {
		pos, size uint64
		want      string
	}{
		{0, 10 * mib4, "d: Block 1 / 10"},
		{1, 10 * mib4, "d: Block 1 / 10"},
		{mib4, 10 * mib4, "d: Block 1 / 10"},
		{mib4 + 1, 10 * mib4, "d: Block 2 / 10"},
		{10 * mib4, 10 * mib4, "d: Block 10 / 10"},
		{10*mib4 + 5, 10*mib4 + 5, "d: Block 11 / 11"},
	}
	for _, c := range cases {
		if got := blockMessage("d", c.pos, c.size); got != c.want {
			t.Errorf("blockMessage(%d, %d) = %q, want %q", c.pos, c.size, got, c.want)
		}
	}
}

func TestJobProgressCountsAndNilSafety(t *testing.T) {
	var nilJP *JobProgress
	nilJP.countChunk(10, true, true) // must not panic
	nilJP.setPhase("data")
	if (nilJP.Where() != Location{}) {
		t.Fatal("nil Where should be empty")
	}

	jp := &JobProgress{}
	jp.countChunk(4, true, true)   // first blank chunk, uploaded
	jp.countChunk(4, true, false)  // more blank space, reused
	jp.countChunk(7, false, true)  // data, uploaded
	jp.countChunk(7, false, false) // data, reused
	if n, r, b, u := jp.NewChunks.Load(), jp.ReusedChunks.Load(), jp.BlankChunks.Load(), jp.UploadedBytes.Load(); n != 2 || r != 2 || b != 1 || u != 11 {
		t.Fatalf("new=%d reused=%d blank=%d uploaded=%d, want 2 2 1 11", n, r, b, u)
	}
	jp.startDisk(`\\.\PhysicalDrive0`, 1, 2, 100)
	jp.setPartition("C:")
	jp.setDiskPos(40, 40)
	w := jp.Where()
	if w.Disk != `\\.\PhysicalDrive0` || w.DiskNo != 1 || w.DiskCount != 2 || w.DiskSize != 100 || w.DiskPos != 40 || w.Partition != "C:" {
		t.Fatalf("Where = %+v", w)
	}
	jp.startDisk("d2", 2, 2, 50)
	if w := jp.Where(); w.Partition != "" || w.DiskPos != 0 {
		t.Fatalf("a new disk must clear the partition and position: %+v", w)
	}
}

func TestZeroChunkDetector(t *testing.T) {
	client := &pbscommon.PBSClient{}
	z := newZeroChunkDetector(client)
	blank := make([]byte, pbscommon.PBS_FIXED_CHUNK_SIZE)
	if !z.isBlank(blank, client.ChunkDigest(blank)) {
		t.Fatal("a 4 MiB chunk of zeros is blank")
	}
	data := make([]byte, pbscommon.PBS_FIXED_CHUNK_SIZE)
	data[12345] = 1
	if z.isBlank(data, client.ChunkDigest(data)) {
		t.Fatal("a chunk with data is not blank")
	}
	short := make([]byte, 4096)
	if z.isBlank(short, client.ChunkDigest(short)) {
		t.Fatal("short chunks are not checked")
	}
}

func TestPartitionLabel(t *testing.T) {
	if got := (Partition{Letter: "C", Number: 3}).label(); got != "C:" {
		t.Errorf("got %q", got)
	}
	if got := (Partition{Number: 1}).label(); got != "partition 1" {
		t.Errorf("got %q", got)
	}
	if got := (Partition{}).label(); got != "unpartitioned space" {
		t.Errorf("got %q", got)
	}
}
