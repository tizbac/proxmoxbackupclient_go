//go:build !windows

package main

import "fmt"

// volumeDiskExtent: Undelete and Roll back from disk images read NTFS, so
// they are offered for Windows machine sets only.
func volumeDiskExtent(path string) (disk int, offset int64, err error) {
	return 0, 0, fmt.Errorf("Undelete and Roll back from disk images need Windows")
}

// liveVolumeSerial: only NTFS volumes are matched by serial number; the
// Linux file systems are found by their partition offset.
func liveVolumeSerial(path string) (uint32, bool) { return 0, false }
