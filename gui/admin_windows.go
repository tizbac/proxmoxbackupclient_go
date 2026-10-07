//go:build windows
// +build windows

package main

import (
	"golang.org/x/sys/windows"
)

// isAdmin reports whether this process is ELEVATED, which is the only thing
// that matters on Windows: opening \\.\PhysicalDriveN to list the disks,
// reading the service's token file (DACL: SYSTEM + Administrators), running
// VSS.
//
// Membership in the Administrators group is not the same thing: since the
// manifest is asInvoker (wails.json), an administrator starts the GUI with a
// FILTERED token whose Administrators SID is deny-only, so a membership check
// would answer "admin" precisely when the process cannot do anything admin-like
// — and the elevation prompt would never be offered to someone who needs it.
func isAdmin() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}

// canModifyJobs returns true if the current user has permission to modify
// scheduled jobs. On Windows, this means running as administrator (UAC
// elevated) — see isAdmin.
func canModifyJobs() bool {
	return isAdmin()
}
