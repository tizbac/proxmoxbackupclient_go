//go:build windows

package machinebackuplib

import (
	"errors"
	"syscall"
)

// Win32 errors a disk returns for a sector it cannot read. Anything else
// (an invalid parameter, access denied, a vanished snapshot) is not a bad
// sector and must not be papered over with zeros.
var mediaErrnos = []syscall.Errno{
	23,   // ERROR_CRC: "Data error (cyclic redundancy check)", what the BIWIN SSD gave
	25,   // ERROR_SEEK
	27,   // ERROR_SECTOR_NOT_FOUND
	30,   // ERROR_READ_FAULT
	31,   // ERROR_GEN_FAILURE
	1117, // ERROR_IO_DEVICE
	483,  // ERROR_DEVICE_HARDWARE_ERROR
}

func isMediaError(err error) bool {
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return false
	}
	for _, e := range mediaErrnos {
		if errno == e {
			return true
		}
	}
	return false
}
