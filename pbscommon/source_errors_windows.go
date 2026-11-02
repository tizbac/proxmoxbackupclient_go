//go:build windows

package pbscommon

import (
	"errors"
	"syscall"
)

// Windows errors seen when the disk holding a file goes away (removed,
// dropped off a USB hub, dismounted) as opposed to a problem with the file.
var deviceGoneErrnos = map[syscall.Errno]bool{
	15:   true, // ERROR_INVALID_DRIVE
	20:   true, // ERROR_BAD_UNIT
	21:   true, // ERROR_NOT_READY
	31:   true, // ERROR_GEN_FAILURE
	55:   true, // ERROR_DEV_NOT_EXIST
	433:  true, // ERROR_NO_SUCH_DEVICE ("A device which does not exist was specified")
	1006: true, // ERROR_FILE_INVALID (volume dismounted under an open handle)
	1117: true, // ERROR_IO_DEVICE
	1167: true, // ERROR_DEVICE_NOT_CONNECTED
	1617: true, // ERROR_DEVICE_REMOVED
}

func isDeviceGoneError(err error) bool {
	var errno syscall.Errno
	return errors.As(err, &errno) && deviceGoneErrnos[errno]
}
