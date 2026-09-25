//go:build linux
// +build linux

package snapshot

import (
	"fmt"
	"runtime"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Talks to the elastio-snap / dattobd kernel modules directly over their
// ioctl control interface instead of shelling out to their elioctl/dbdctl
// CLI wrappers. Those wrappers are themselves thin: they parse arguments and
// issue the exact same ioctl calls implemented here (see
// github.com/elastio/elastio-snap's app/elioctl.c and lib/libelastio-snap.c,
// and github.com/datto/dattobd's src/dattobd.h), so removing the dependency
// on the CLI binaries being installed loses nothing.
//
// Only IOCTL_SETUP_SNAP and IOCTL_DESTROY are implemented, the only two this
// project ever needs: creation and teardown of a short-lived snapshot for a
// single backup run. Reload/transition/reconfigure exist in both drivers for
// making a snapshot survive a reboot, which never applies here.

const (
	elastioSnapMagic uintptr = 0x41 // 'A', github.com/elastio/elastio-snap src/elastio-snap.h
	dattobdMagic     uintptr = 0x91 // github.com/datto/dattobd src/dattobd.h

	// Generic (non-mips/sparc) Linux ioctl number encoding, include/uapi/asm-generic/ioctl.h.
	iocNRBits    = 8
	iocTypeBits  = 8
	iocSizeBits  = 14
	iocNRShift   = 0
	iocTypeShift = iocNRShift + iocNRBits
	iocSizeShift = iocTypeShift + iocTypeBits
	iocDirShift  = iocSizeShift + iocSizeBits
	iocWrite     = 1
)

func iow(magic, nr, size uintptr) uintptr {
	return (iocWrite << iocDirShift) | (magic << iocTypeShift) | (nr << iocNRShift) | (size << iocSizeShift)
}

// setupParams mirrors both drivers' `struct setup_params` as compiled on
// amd64: two 8-byte C string pointers, two 8-byte unsigned longs, a 4-byte
// minor, and 4 trailing bytes. elastio-snap declares that tail as a trailing
// `bool ignore_snap_errors` (occupying its first byte, 1-3 pure alignment
// padding); dattobd has no such field and the same 4 bytes are pure
// structure-alignment padding there. Either way the kernel driver never
// reads those bytes as anything but that one optional flag or nothing, so
// zeroing them (== "don't ignore snapshot errors") is correct for both and
// one Go struct layout serves both drivers.
type setupParams struct {
	bdev             uintptr
	cow              uintptr
	fallocatedSpace  uint64
	cacheSize        uint64
	minor            uint32
	ignoreSnapErrors uint32
}

var setupParamsSize = unsafe.Sizeof(setupParams{})

func ioctlMagic(driver string) (uintptr, error) {
	switch driver {
	case "elastio-snap":
		return elastioSnapMagic, nil
	case "dattobd":
		return dattobdMagic, nil
	default:
		return 0, fmt.Errorf("no known ioctl interface for snapshot driver %q", driver)
	}
}

// ioctlSetupSnapshot issues IOCTL_SETUP_SNAP: create a copy-on-write
// snapshot of device at the given minor number, backed by cowFile.
func ioctlSetupSnapshot(ctlDevice, driver, device, cowFile string, minor int) error {
	magic, err := ioctlMagic(driver)
	if err != nil {
		return err
	}

	fd, err := unix.Open(ctlDevice, unix.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("open %s: %w", ctlDevice, err)
	}
	defer unix.Close(fd)

	bdevPtr, err := unix.BytePtrFromString(device)
	if err != nil {
		return fmt.Errorf("invalid device path %q: %w", device, err)
	}
	cowPtr, err := unix.BytePtrFromString(cowFile)
	if err != nil {
		return fmt.Errorf("invalid cow file path %q: %w", cowFile, err)
	}

	params := setupParams{
		bdev:  uintptr(unsafe.Pointer(bdevPtr)),
		cow:   uintptr(unsafe.Pointer(cowPtr)),
		minor: uint32(minor),
	}

	cmd := iow(magic, 1, setupParamsSize)
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), cmd, uintptr(unsafe.Pointer(&params)))
	// bdevPtr/cowPtr are only referenced by raw address inside params for the
	// duration of the syscall; a uintptr doesn't keep them alive for the GC,
	// so pin them explicitly until the kernel is done reading through that
	// pointer.
	runtime.KeepAlive(bdevPtr)
	runtime.KeepAlive(cowPtr)
	if errno != 0 {
		return fmt.Errorf("%s IOCTL_SETUP_SNAP on %s (minor %d): %w", driver, device, minor, errno)
	}
	return nil
}

// ioctlDestroySnapshot issues IOCTL_DESTROY: tear down the snapshot at minor.
func ioctlDestroySnapshot(ctlDevice, driver string, minor int) error {
	magic, err := ioctlMagic(driver)
	if err != nil {
		return err
	}

	fd, err := unix.Open(ctlDevice, unix.O_RDONLY, 0)
	if err != nil {
		return fmt.Errorf("open %s: %w", ctlDevice, err)
	}
	defer unix.Close(fd)

	m := uint32(minor)
	cmd := iow(magic, 4, unsafe.Sizeof(m))
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), cmd, uintptr(unsafe.Pointer(&m)))
	if errno != 0 {
		return fmt.Errorf("%s IOCTL_DESTROY minor %d: %w", driver, minor, errno)
	}
	return nil
}
