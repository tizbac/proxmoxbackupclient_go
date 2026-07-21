//go:build linux
// +build linux

package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"pbscommon"
)

type faultyReaderAt struct {
	data []byte
	bad  map[uint64]bool
}

func (f *faultyReaderAt) ReadAt(p []byte, off int64) (int, error) {
	for i := range p {
		abs := uint64(off) + uint64(i)
		if abs >= uint64(len(f.data)) {
			return i, io.EOF
		}
		if f.bad[abs/sectorSize] {
			if i == 0 {
				return 0, io.ErrUnexpectedEOF
			}
			return i, io.ErrUnexpectedEOF
		}
		p[i] = f.data[abs]
	}
	return len(p), nil
}

type seg = diskSegment

func TestAssembleSegments(t *testing.T) {
	const total = 1000
	cases := []struct {
		name    string
		regions []seg
		want    []seg
	}{
		{
			name:    "no snapshot regions - whole disk raw",
			regions: nil,
			want:    []seg{{start: 0, end: total}},
		},
		{
			name:    "leading gap, region, trailing gap",
			regions: []seg{{start: 100, end: 400, snapDev: "/dev/snap0"}},
			want: []seg{
				{start: 0, end: 100},
				{start: 100, end: 400, snapDev: "/dev/snap0"},
				{start: 400, end: total},
			},
		},
		{
			name: "two regions with a gap and unmounted partition between",
			regions: []seg{
				{start: 100, end: 300, snapDev: "/dev/snap0"},
				{start: 500, end: 700, snapDev: "/dev/snap1"},
			},
			want: []seg{
				{start: 0, end: 100},
				{start: 100, end: 300, snapDev: "/dev/snap0"},
				{start: 300, end: 500},
				{start: 500, end: 700, snapDev: "/dev/snap1"},
				{start: 700, end: total},
			},
		},
		{
			name:    "region flush to disk start and end (no gaps)",
			regions: []seg{{start: 0, end: total, snapDev: "/dev/snap0"}},
			want:    []seg{{start: 0, end: total, snapDev: "/dev/snap0"}},
		},
		{
			name: "unsorted input gets ordered",
			regions: []seg{
				{start: 500, end: 700, snapDev: "/dev/snap1"},
				{start: 100, end: 300, snapDev: "/dev/snap0"},
			},
			want: []seg{
				{start: 0, end: 100},
				{start: 100, end: 300, snapDev: "/dev/snap0"},
				{start: 300, end: 500},
				{start: 500, end: 700, snapDev: "/dev/snap1"},
				{start: 700, end: total},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := assembleSegments(total, tc.regions)

			if len(got) != len(tc.want) {
				t.Fatalf("segment count = %d, want %d\ngot:  %+v\nwant: %+v", len(got), len(tc.want), got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("segment[%d] = %+v, want %+v", i, got[i], tc.want[i])
				}
			}

			var cursor uint64 = 0
			for i, s := range got {
				if s.start != cursor {
					t.Fatalf("segment[%d] starts at %d, expected contiguous %d", i, s.start, cursor)
				}
				if s.end <= s.start {
					t.Fatalf("segment[%d] is empty/backwards: %+v", i, s)
				}
				cursor = s.end
			}
			if cursor != total {
				t.Fatalf("segments cover up to %d, want %d", cursor, total)
			}
		})
	}
}

func TestWriteSegmentsByteExact(t *testing.T) {
	dir := t.TempDir()

	const total = 3*pbscommon.PBS_FIXED_CHUNK_SIZE + 12345

	disk := make([]byte, total)
	for i := range disk {
		disk[i] = byte(i%251) + 1
	}
	diskPath := filepath.Join(dir, "disk.img")
	if err := os.WriteFile(diskPath, disk, 0o600); err != nil {
		t.Fatal(err)
	}

	const snapStart = pbscommon.PBS_FIXED_CHUNK_SIZE - 1000
	const snapEnd = 2*pbscommon.PBS_FIXED_CHUNK_SIZE + 4000
	snapLen := snapEnd - snapStart

	const shortBy = 777
	snap := make([]byte, snapLen-shortBy)
	for i := range snap {
		snap[i] = 0xAA
	}
	snapPath := filepath.Join(dir, "snap0")
	if err := os.WriteFile(snapPath, snap, 0o600); err != nil {
		t.Fatal(err)
	}

	segments := assembleSegments(total, []diskSegment{
		{start: snapStart, end: snapEnd, snapDev: snapPath},
	})

	want := make([]byte, total)
	copy(want, disk)
	for i := snapStart; i < snapEnd; i++ {
		if i-snapStart < len(snap) {
			want[i] = snap[i-snapStart]
		} else {
			want[i] = 0
		}
	}

	ch := make(chan []byte)
	errc := make(chan error, 1)
	go func() { errc <- writeSegments(diskPath, segments, ch) }()

	got := make([]byte, 0, total)
	var chunkCount int
	for chunk := range ch {
		chunkCount++
		got = append(got, chunk...)
	}
	if err := <-errc; err != nil {
		t.Fatalf("writeSegments: %v", err)
	}

	if len(got) != total {
		t.Fatalf("stitched length = %d, want %d", len(got), total)
	}
	if !bytes.Equal(got, want) {

		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("byte %d = %#x, want %#x", i, got[i], want[i])
			}
		}
	}

	expectedChunks := (total + pbscommon.PBS_FIXED_CHUNK_SIZE - 1) / pbscommon.PBS_FIXED_CHUNK_SIZE
	if chunkCount != expectedChunks {
		t.Fatalf("chunk count = %d, want %d", chunkCount, expectedChunks)
	}
}

func TestResilientCopyBadSectors(t *testing.T) {
	const length = 5*sectorSize + 100

	data := make([]byte, length)
	for i := range data {
		data[i] = byte(i%251) + 1
	}

	badSet := map[uint64]bool{1: true, 3: true}
	src := &faultyReaderAt{data: data, bad: badSet}

	var out []byte
	emit := func(b []byte) { out = append(out, b...) }
	zeroBlk := make([]byte, sectorSize)
	emitZeros := func(n uint64) {
		for n > 0 {
			m := uint64(len(zeroBlk))
			if m > n {
				m = n
			}
			out = append(out, zeroBlk[:m]...)
			n -= m
		}
	}

	block := make([]byte, pbscommon.PBS_FIXED_CHUNK_SIZE)
	bad := resilientCopy(src, 0, length, block, emit, emitZeros)

	if bad != 2 {
		t.Fatalf("bad sector count = %d, want 2", bad)
	}
	if uint64(len(out)) != length {
		t.Fatalf("output length = %d, want %d", len(out), length)
	}

	want := make([]byte, length)
	copy(want, data)
	for sec := range badSet {
		start := sec * sectorSize
		end := start + sectorSize
		if end > length {
			end = length
		}
		for i := start; i < end; i++ {
			want[i] = 0
		}
	}
	if !bytes.Equal(out, want) {
		for i := range out {
			if out[i] != want[i] {
				t.Fatalf("byte %d (sector %d) = %#x, want %#x", i, uint64(i)/sectorSize, out[i], want[i])
			}
		}
	}
}
