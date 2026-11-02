//go:build !windows

package pbscommon

import "syscall"

var testDeviceGoneErr error = syscall.EIO
