package machinebackuplib

import (
	"bytes"
	"testing"
)

// A partition of 64 KiB at 1 MiB on the disk whose shadow copy holds only
// the first 60 KiB (the volume): the last 4 KiB, where NTFS keeps its backup
// boot sector, must come from the disk and not be zeros.
func TestShadowThenDiskReadsTailFromDisk(t *testing.T) {
	const start, partLen, volLen = 1 << 20, 64 << 10, 60 << 10
	disk := make([]byte, start+partLen+(1<<20))
	for i := range disk {
		disk[i] = byte(i%251) | 1 // never zero
	}
	// The shadow differs from the live disk, as a snapshot does.
	shadow := make([]byte, volLen)
	for i := range shadow {
		shadow[i] = 0xAA
	}
	src := &shadowThenDisk{shadow: bytes.NewReader(shadow), disk: bytes.NewReader(disk), start: start, name: "test shadow"}

	var got []byte
	emit := func(b []byte) bool { got = append(got, b...); return true }
	emitZeros := func(n uint64) bool { got = append(got, make([]byte, n)...); return true }
	block := make([]byte, 16<<10) // the shadow ends part way through a read
	if err := readResilient(src, 0, partLen, block, 4096, emit, emitZeros, nil); err != nil {
		t.Fatal(err)
	}
	if len(got) != partLen {
		t.Fatalf("read %d bytes, want %d", len(got), partLen)
	}
	if !bytes.Equal(got[:volLen], shadow) {
		t.Error("the volume did not come from the shadow")
	}
	if !bytes.Equal(got[volLen:], disk[start+volLen:start+partLen]) {
		t.Error("the end of the partition did not come from the disk")
	}
	if !src.ended {
		t.Error("the end of the shadow was not noticed")
	}
}

// A shadow that covers the whole partition is read as it is: the disk is
// not touched.
func TestShadowThenDiskWholeShadow(t *testing.T) {
	shadow := bytes.Repeat([]byte{0xAA}, 32<<10)
	src := &shadowThenDisk{shadow: bytes.NewReader(shadow), disk: failingReaderAt{}, start: 0, name: "test shadow"}
	p := make([]byte, len(shadow))
	if n, err := src.ReadAt(p, 0); n != len(p) || err != nil {
		t.Fatalf("ReadAt = %d, %v", n, err)
	}
	if !bytes.Equal(p, shadow) || src.ended {
		t.Error("whole shadow not read as it is")
	}
}

type failingReaderAt struct{}

func (failingReaderAt) ReadAt(p []byte, off int64) (int, error) {
	panic("the disk was read")
}
