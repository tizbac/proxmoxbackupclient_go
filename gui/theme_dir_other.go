//go:build !windows

package main

// userThemeDir is the user's own settings folder, ~/.proxmox-backup-guardian:
// the administrator app (proxmoxbackupclient-root) hands it back to the
// user when it closes, so a theme saved there as root stays the user's.
func userThemeDir() (string, error) {
	return userConfigDir()
}
