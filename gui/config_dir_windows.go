//go:build windows

package main

import (
	"os"
	"path/filepath"
)

// The settings folder on Windows is C:\ProgramData\ProxmoxBackupClient
// (getConfigDir); these keep the Unix shared-folder helpers callable.

// unixConfigDir: only reached when neither ProgramData nor SystemDrive is
// set (tests do this); the home folder, as before.
func unixConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".proxmox-backup-guardian"), nil
}

func sharedFileMode(path string, perm os.FileMode) os.FileMode { return perm }

// shareConfigFile: the folder's ACL (hardenConfigDir) covers the API token.
func shareConfigFile(path string) {}
