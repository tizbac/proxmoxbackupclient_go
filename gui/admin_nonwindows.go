//go:build !windows
// +build !windows

package main

import (
	"os"
	"os/exec"
	"os/user"
)

// isAdmin returns true if running as root (euid == 0) on non-Windows systems
func isAdmin() bool {
	return os.Geteuid() == 0
}

// canModifyJobs returns true if the current user has permission to modify
// scheduled jobs. On Linux, this means being in the wheel group or having
// sudo privileges. On Windows, this means running as administrator.
func canModifyJobs() bool {
	// On Windows, the check is done in admin_windows.go via isAdmin()
	// This function is only called on non-Windows systems
	if isAdmin() {
		return true
	}

	// Check if user is in wheel group
	currentUser, err := user.Current()
	if err == nil {
		groups, err := currentUser.GroupIds()
		if err == nil {
			for _, gid := range groups {
				group, err := user.LookupGroupId(gid)
				if err == nil && (group.Name == "wheel" || group.Name == "sudo") {
					return true
				}
			}
		}
	}

	// Check sudoers by trying to run sudo -n true (non-interactive)
	// If this succeeds, the user has passwordless sudo
	if _, err := os.Stat("/usr/bin/sudo"); err == nil {
		if err := runCommand("/usr/bin/sudo", "-n", "true"); err == nil {
			return true
		}
	}

	// Check pkexec availability (policykit)
	if _, err := os.Stat("/usr/bin/pkexec"); err == nil {
		// pkexec typically requires authentication, so we consider it available
		// but we'll need to prompt for credentials
		return true
	}

	return false
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	return cmd.Run()
}
