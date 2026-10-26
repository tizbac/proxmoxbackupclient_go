package machinebackuplib

import (
	"io"
	"log"
)

// shadowThenDisk reads a partition through its volume shadow copy and, past
// the shadow's end, from the disk itself.
//
// The shadow covers the volume, which can end before the partition does.
// NTFS always does: its backup boot sector is the partition's last sector,
// outside the volume. Read through the shadow alone, that part went into the
// image as zeros. The file system never writes past the volume's end, so
// reading it from the live disk is as consistent as the shadow.
//
// Offsets are in the partition. disk is read at start+offset.
type shadowThenDisk struct {
	shadow io.ReaderAt
	disk   io.ReaderAt
	start  int64
	name   string
	ended  bool // the shadow has ended (logged once)
}

func (s *shadowThenDisk) ReadAt(p []byte, off int64) (int, error) {
	n, err := s.shadow.ReadAt(p, off)
	if err != io.EOF || n == len(p) {
		return n, err
	}
	if !s.ended {
		s.ended = true
		log.Printf("%s ends at offset %d of the partition, reading the rest from the disk", s.name, off+int64(n))
	}
	m, err := s.disk.ReadAt(p[n:], s.start+off+int64(n))
	return n + m, err
}
