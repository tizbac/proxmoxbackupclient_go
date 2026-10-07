//go:build windows

package pbscommon

import "syscall"

// testDeviceGoneErr is what a read returned on the live 2026-10-07 drop.
var testDeviceGoneErr error = syscall.Errno(433) // ERROR_NO_SUCH_DEVICE
