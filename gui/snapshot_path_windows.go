//go:build windows
// +build windows

package main

import (
	"strings"

	"golang.org/x/sys/windows"
)

// pathSupportsSnapshot reports whether path lives on a volume VSS can
// actually snapshot. VSS shadow-copies a local volume; a network share
// (whether accessed via a raw UNC path or a drive letter mapped to one)
// has no local volume for VSS to snapshot at all, and fails outright if
// asked — see backupDirectory's call site for the live bug this fixes.
func pathSupportsSnapshot(path string) bool {
	root := volumeRoot(path)
	if root == "" {
		return true // couldn't determine — don't block a backup over it
	}
	rootW, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return true
	}
	return windows.GetDriveType(rootW) != windows.DRIVE_REMOTE
}

// volumeRoot extracts the volume portion GetDriveType expects: "C:\" for a
// drive-letter path, or "\\server\share\" for a UNC path. Returns "" if path
// doesn't look like either.
func volumeRoot(path string) string {
	if len(path) >= 2 && path[1] == ':' {
		return path[:2] + `\`
	}
	if strings.HasPrefix(path, `\\`) {
		rest := path[2:]
		// First two path segments after the leading \\ are server\share;
		// GetDriveType wants exactly \\server\share\, nothing deeper.
		parts := strings.SplitN(rest, `\`, 3)
		if len(parts) >= 2 && parts[0] != "" && parts[1] != "" {
			return `\\` + parts[0] + `\` + parts[1] + `\`
		}
	}
	return ""
}
