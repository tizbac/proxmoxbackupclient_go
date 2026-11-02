//go:build windows

package main

import (
	"os"
	"path/filepath"
)

// userThemeDir is the user's own folder for their preferences:
// %APPDATA%\ProxmoxBackupClient (roams with the profile).
func userThemeDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "ProxmoxBackupClient"), nil
}
