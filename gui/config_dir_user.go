package main

import (
	"os"
	"path/filepath"
)

// userConfigDir is the user's own settings folder, ~/.proxmox-backup-guardian
// on every system (upstream's standalone folder): all the app's settings when
// no service is installed, and its own files otherwise (getConfigDir).
func userConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".proxmox-backup-guardian"), nil
}
