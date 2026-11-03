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
// asked. The app's folder backups and proxmoxbackup-directory read such a
// path live instead.
func PathSupportsSnapshot(path string) bool {
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
