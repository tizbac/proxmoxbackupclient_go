//go:build windows

package main

import (
	"encoding/binary"
	"fmt"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/windows"
)

const ioctlVolumeGetVolumeDiskExtents = 0x00560000

// volumeDiskExtent says where the volume holding path lives: the physical
// disk's number (as in \\.\PhysicalDriveN) and the byte offset of its
// partition, which is where the same partition sits in that disk's image.
func volumeDiskExtent(path string) (disk int, offset int64, err error) {
	vol := filepath.VolumeName(filepath.Clean(path))
	if len(vol) != 2 || vol[1] != ':' {
		return 0, 0, fmt.Errorf("%s is not on a drive letter", path)
	}
	name, _ := syscall.UTF16PtrFromString(`\\.\` + vol)
	h, err := syscall.CreateFile(name, 0, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE, nil, syscall.OPEN_EXISTING, 0, 0)
	if err != nil {
		return 0, 0, fmt.Errorf("open %s: %w", vol, err)
	}
	defer syscall.CloseHandle(h)
	// VOLUME_DISK_EXTENTS: a count (padded to 8 bytes), then one DISK_EXTENT
	// per extent: disk number (padded), starting offset, length.
	var out [8 + 24*4]byte
	var n uint32
	if err := syscall.DeviceIoControl(h, ioctlVolumeGetVolumeDiskExtents, nil, 0, &out[0], uint32(len(out)), &n, nil); err != nil {
		return 0, 0, fmt.Errorf("disk extents of %s: %w", vol, err)
	}
	if c := binary.LittleEndian.Uint32(out[0:]); c != 1 {
		return 0, 0, fmt.Errorf("%s spans %d disks", vol, c)
	}
	return int(binary.LittleEndian.Uint32(out[8:])), int64(binary.LittleEndian.Uint64(out[16:])), nil
}

// liveVolumeSerial is the volume serial number of the drive holding path, as
// `vol` shows it (the low 32 bits of an NTFS volume's serial).
func liveVolumeSerial(path string) (uint32, bool) {
	vol := filepath.VolumeName(filepath.Clean(path))
	if len(vol) != 2 || vol[1] != ':' {
		return 0, false
	}
	root, _ := windows.UTF16PtrFromString(vol + `\`)
	var serial uint32
	if err := windows.GetVolumeInformation(root, nil, 0, &serial, nil, nil, nil, 0); err != nil {
		return 0, false
	}
	return serial, true
}
