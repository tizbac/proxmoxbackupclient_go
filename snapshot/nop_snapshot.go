//go:build darwin || freebsd || openbsd || netbsd || solaris
// +build darwin freebsd openbsd netbsd solaris

package snapshot

import (
	"context"
	"log"
)

// CreateVSSSnapshotCtx is CreateVSSSnapshot; nothing here waits.
func CreateVSSSnapshotCtx(_ context.Context, paths []string, needFiles bool, backup_callback func(sn map[string]SnapShot) error) error {
	return CreateVSSSnapshot(paths, needFiles, backup_callback)
}

func CreateVSSSnapshot(paths []string, needFiles bool, backup_callback func(sn map[string]SnapShot) error) error {
	log.Print("\033[31;1mWarning, on linux snapshots are not supported builtin, proceeding without!\033[0m")
	ret := make(map[string]SnapShot)
	for _, x := range paths {
		ret[x] = SnapShot{
			FullPath: x,
			Valid:    true,
		}
	}
	return backup_callback(ret)
}

func VSSCleanup() error {
	return nil
}

// DetectControl reports the Linux block snapshot module in use. There is
// none on this platform.
func DetectControl() (SnapControl, bool) {
	return SnapControl{}, false
}
