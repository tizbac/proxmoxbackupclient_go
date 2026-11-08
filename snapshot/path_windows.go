//go:build windows
// +build windows

package snapshot

import (
	"golang.org/x/sys/windows"
)

// PathSupportsSnapshot reports whether path lives on a volume VSS can
// actually snapshot. VSS shadow-copies a local volume; a network share
// (whether accessed via a raw UNC path or a drive letter mapped to one)
// has no local volume for VSS to snapshot at all, and fails outright if
// asked. The app's folder backups and salvanza-cli backup-folder read such a
// path live instead.
//
// A UNC path is a share without asking: GetDriveType says DRIVE_REMOTE only
// for a share it can open, and answers DRIVE_NO_ROOT_DIR for one this account
// cannot (the service runs as SYSTEM, which reaches the network as the
// computer's own account). Such a share was taken for a local volume, and the
// backup failed with "snapshots are not supported" instead of reading it, or
// saying it was denied (found 2026-11-05 on a scheduled set).
func PathSupportsSnapshot(path string) bool {
	if isNetworkPath(path) {
		return false
	}
	root := volumeRoot(path)
	if root == "" {
		return true // couldn't determine: don't block a backup over it
	}
	rootW, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return true
	}
	return windows.GetDriveType(rootW) != windows.DRIVE_REMOTE
}
