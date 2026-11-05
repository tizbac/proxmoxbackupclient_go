package main

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// userConfigDir is the user's own settings folder, ~/.salvanza (or the
// brand's, ~/.nimbusbackup) on every system: all the app's settings when no
// service is installed, and its own files otherwise (getConfigDir). The first
// time, it takes over the folder of an earlier install: ~/.proxmox-backup-guardian,
// the one every brand shared before the rename, or another brand's.
func userConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, "."+folderNameLower())
	adoptUserStateOnce.Do(func() {
		candidates := []string{filepath.Join(home, legacyUserFolder)}
		for _, f := range otherBrandFolders() {
			candidates = append(candidates, filepath.Join(home, "."+strings.ToLower(f)))
		}
		_, _ = adoptFolder(dir, candidates)
	})
	return dir, nil
}

var adoptUserStateOnce sync.Once
