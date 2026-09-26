//go:build linux
// +build linux

package main

import "golang.org/x/sys/unix"

// Network/pseudo filesystem magic numbers (linux/magic.h) that elastio-snap/
// dattobd can't snapshot: both operate on a real underlying block device,
// which a network share or FUSE mount doesn't have.
const (
	nfsSuperMagic   = 0x6969
	smbSuperMagic   = 0x517B
	cifsMagicNumber = 0xFF534D42
	fuseSuperMagic  = 0x65735546
)

// pathSupportsSnapshot reports whether path lives on a filesystem
// elastio-snap/dattobd can actually snapshot. See the Windows implementation
// of this same function for the shared reasoning (network shares have no
// local volume/block device to snapshot at all).
func pathSupportsSnapshot(path string) bool {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return true // couldn't determine — don't block a backup over it
	}
	switch int64(st.Type) {
	case nfsSuperMagic, smbSuperMagic, cifsMagicNumber, fuseSuperMagic:
		return false
	default:
		return true
	}
}
