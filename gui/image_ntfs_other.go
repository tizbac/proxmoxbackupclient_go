//go:build !windows

package main

import "github.com/tizbac/proxmoxbackupclient_go/gui/imagefs"

// restoreImageStreams: alternate data streams have nowhere to go off Windows.
func restoreImageStreams(vol imagefs.Volume, dest string, e imagefs.Entry, buf []byte) (int, error) {
	return 0, nil
}

// applyImagePermissions: Windows permissions do not apply off Windows.
func applyImagePermissions(vol imagefs.Volume, dest string, e imagefs.Entry, cache map[uint32]string) (applied, ownerKept bool, err error) {
	return false, false, nil
}
