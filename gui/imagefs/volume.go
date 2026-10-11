package imagefs

import (
	"errors"
	"fmt"
	"io"
	"time"
)

// Volume is a file system inside a disk image that can be listed and read:
// NTFS, FAT12/16/32, exFAT, ext2/3/4 or XFS.
type Volume interface {
	// FSName is the file system's display name: "NTFS", "FAT32", "exFAT",
	// "ext4", "XFS"...
	FSName() string
	// Label is the volume's name, "" if it has none.
	Label() string
	// List returns every file, folder and symbolic link, sorted by path.
	// progress, if set, gets how much of the volume's metadata has been read
	// and its total, in whatever unit the file system counts (bytes, inodes).
	List(cancel func() bool, progress func(done, total int64)) ([]Entry, error)
	// Open opens a file entry from List for reading.
	Open(e Entry) (FileReader, error)
	// CaseInsensitive: names match without regard to case (NTFS, FAT, exFAT).
	CaseInsensitive() bool
}

// FileReader reads one file of a volume.
type FileReader interface {
	io.ReaderAt
	Size() int64
	// Extents are the byte ranges of the volume that hold the file's data,
	// [start, end), for read-ahead; nil when not known.
	Extents() [][2]int64
}

// ErrBitLocker: the partition is encrypted with BitLocker. A machine backup
// reads raw partitions without a snapshot only when it cannot take one, and a
// BitLocker volume read that way is the encrypted data.
var ErrBitLocker = errors.New("encrypted with BitLocker: the backup holds the encrypted data, so its files cannot be browsed (back the volume up with its snapshot, or restore the whole disk)")

// ErrUnsupported: no reader for this partition's file system.
var ErrUnsupported = errors.New("file system not supported for browsing")

// OpenVolume opens the file system fs (as Probe names it) on r, a partition
// of size bytes.
func OpenVolume(fs string, r io.ReaderAt, size int64) (v Volume, err error) {
	// The readers guard against malformed structures, but a damaged image
	// must give an error, never take the GUI down.
	defer func() {
		if p := recover(); p != nil {
			v, err = nil, fmt.Errorf("%s: %v", fs, p)
		}
	}()
	switch fs {
	case "ntfs":
		return OpenNTFS(r)
	case "fat32", "fat":
		return OpenFAT(r, size)
	case "exfat":
		return OpenExFAT(r, size)
	case "ext":
		return OpenExt(r, size)
	case "xfs":
		return OpenXFS(r, size)
	case "bitlocker":
		return nil, ErrBitLocker
	case "":
		return nil, fmt.Errorf("%w (unknown)", ErrUnsupported)
	}
	return nil, fmt.Errorf("%w (%s)", ErrUnsupported, fs)
}

// FATTimeLocation is the time zone FAT timestamps are read in. FAT records
// local time without saying which; Windows writes the time of the computer
// that wrote the file, so the restoring computer's zone is the best guess.
var FATTimeLocation = time.Local

// Filetime converts a time to a FILETIME (100 ns since 1601), the unit Entry
// uses for every file system.
func Filetime(t time.Time) int64 {
	return t.Unix()*10000000 + int64(t.Nanosecond())/100 + 116444736000000000
}

// unixFiletime is Filetime for Unix seconds and nanoseconds.
func unixFiletime(sec int64, nsec int64) int64 {
	return sec*10000000 + nsec/100 + 116444736000000000
}

// POSIX file type bits of Entry.Mode.
const (
	ModeTypeMask = 0o170000
	ModeDir      = 0o040000
	ModeRegular  = 0o100000
	ModeSymlink  = 0o120000
)
