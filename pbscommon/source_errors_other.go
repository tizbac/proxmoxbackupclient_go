//go:build !windows

package pbscommon

import (
	"errors"
	"syscall"
)

// isDeviceGoneError reports errors seen when the disk or share holding a file
// goes away, as opposed to a problem with the file.
func isDeviceGoneError(err error) bool {
	return errors.Is(err, syscall.EIO) || errors.Is(err, syscall.ENXIO) ||
		errors.Is(err, syscall.ENODEV) || errors.Is(err, syscall.ESTALE)
}
