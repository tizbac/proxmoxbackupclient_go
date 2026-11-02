package machinebackuplib

import (
	"bytes"
	"io"
	"syscall"
	"testing"
)

// errBadSector is a media error on every platform: ERROR_CRC on Windows,
// and on Linux any read error counts.
var errBadSector error = syscall.Errno(23)

// faultyReaderAt fails any read that touches a bad unit (unit-sized, from 0),
// returning what it read before it, like a disk with bad sectors.
type faultyReaderAt struct {
	data []byte
	unit uint64
	bad  map[uint64]bool
	err  error // the error for a bad unit; errBadSector if nil
}

func (f *faultyReaderAt) ReadAt(p []byte, off int64) (int, error) {
	for i := range p {
		abs := uint64(off) + uint64(i)
		if abs >= uint64(len(f.data)) {
			return i, io.EOF
		}
		if f.bad[abs/f.unit] {
			if f.err != nil {
				return i, f.err
			}
			return i, errBadSector
		}
		p[i] = f.data[abs]
	}
	return len(p), nil
}

// collect runs readResilient into a byte slice.
func collect(t *testing.T, src io.ReaderAt, off, length, unit uint64, block int) ([]byte, [][2]uint64, error) {
	t.Helper()
	var out []byte
	var bad [][2]uint64
	emit := func(b []byte) bool { out = append(out, b...); return true }
	emitZeros := func(n uint64) bool { out = append(out, make([]byte, n)...); return true }
	err := readResilient(src, off, length, make([]byte, block), unit, emit, emitZeros,
		func(o, n uint64) { bad = append(bad, [2]uint64{o, n}) })
	return out, bad, err
}

func pattern(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i%251) + 1
	}
	return b
}

func TestReadResilientZeroFillsBadUnits(t *testing.T) {
	const unit = 512
	data := pattern(5*unit + 100)
	src := &faultyReaderAt{data: data, unit: unit, bad: map[uint64]bool{1: true, 3: true}}

	out, bad, err := collect(t, src, 0, uint64(len(data)), unit, 4096)
	if err != nil {
		t.Fatal(err)
	}
	want := append([]byte(nil), data...)
	for _, u := range []int{1, 3} {
		copy(want[u*unit:(u+1)*unit], make([]byte, unit))
	}
	if !bytes.Equal(out, want) {
		t.Fatal("output differs: bad units must be zeros, the rest unchanged")
	}
	if len(bad) != 2 || bad[0] != [2]uint64{512, 512} || bad[1] != [2]uint64{1536, 512} {
		t.Fatalf("bad ranges = %v", bad)
	}
}

func TestReadResilientOffsetAndShortSource(t *testing.T) {
	// A snapshot smaller than its partition: read from an offset, pad the end.
	const unit = 4096
	data := pattern(3 * unit)
	src := &faultyReaderAt{data: data, unit: unit, bad: map[uint64]bool{2: true}}
	out, bad, err := collect(t, src, unit, 4*unit, unit, 8192)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 4*unit {
		t.Fatalf("length %d, want %d", len(out), 4*unit)
	}
	if !bytes.Equal(out[:unit], data[unit:2*unit]) || !bytes.Equal(out[unit:], make([]byte, 3*unit)) {
		t.Fatal("expected data, then a zero-filled bad unit, then zero padding")
	}
	if len(bad) != 1 || bad[0] != [2]uint64{2 * unit, unit} {
		t.Fatalf("bad ranges = %v (offsets are in the source)", bad)
	}
}

func TestReadResilientStops(t *testing.T) {
	src := &faultyReaderAt{data: pattern(10000), unit: 512}
	n := 0
	err := readResilient(src, 0, 10000, make([]byte, 1024), 512,
		func(b []byte) bool { n++; return n < 3 }, func(uint64) bool { return true }, nil)
	if err != errCopyStopped {
		t.Fatalf("err = %v, want errCopyStopped", err)
	}
}

func TestJobProgressUnreadableMerges(t *testing.T) {
	jp := &JobProgress{}
	jp.noteUnreadable("d", 512, 512)
	jp.noteUnreadable("d", 1024, 512) // continues the first
	jp.noteUnreadable("d", 4096, 4096)
	jp.noteUnreadable("e", 8192, 512) // other disk
	got := jp.Unreadable()
	want := []BadRange{{"d", 512, 1024}, {"d", 4096, 4096}, {"e", 8192, 512}}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
	if jp.UnreadableBytes.Load() != 512+512+4096+512 {
		t.Fatalf("UnreadableBytes = %d", jp.UnreadableBytes.Load())
	}
}
