//go:build !windows

package main

import (
	"os"

	"github.com/tizbac/proxmoxbackupclient_go/gui/imagefs"
)

// applyImageOwnerMode gives a file or folder restored from a Linux volume in
// a disk image its permissions and, when restoring as root, its owner.
func applyImageOwnerMode(dest string, e imagefs.Entry) {
	if e.Mode&0o7777 != 0 {
		_ = os.Chmod(dest, os.FileMode(e.Mode&0o777)|unixSpecialBits(e.Mode))
	}
	if e.HasOwner && os.Geteuid() == 0 {
		_ = os.Lchown(dest, int(e.UID), int(e.GID))
	}
}

// unixSpecialBits maps setuid, setgid and sticky to os.FileMode.
func unixSpecialBits(m uint32) os.FileMode {
	var out os.FileMode
	if m&0o4000 != 0 {
		out |= os.ModeSetuid
	}
	if m&0o2000 != 0 {
		out |= os.ModeSetgid
	}
	if m&0o1000 != 0 {
		out |= os.ModeSticky
	}
	return out
}
