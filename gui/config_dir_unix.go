//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"strings"
)

// sharedConfigDir is the settings folder of the Linux background service, the
// counterpart of C:\ProgramData\ProxmoxBackupClient: owned by root, readable by
// the computer's administrators (see hardenConfigDir). A variable so tests can
// point it elsewhere.
var sharedConfigDir = "/var/lib/proxmoxbackupclient"

// useSharedConfigDir: the service always uses the shared folder; the app uses
// it once it exists (the service was installed on this computer), so the app
// and the service see the same Backup Sets. Before that each user keeps their
// own settings in ~/.proxmox-backup-guardian.
func useSharedConfigDir() bool {
	if serviceBuild {
		return true
	}
	_, err := os.Stat(sharedConfigDir)
	return err == nil
}

// unixConfigDir is getConfigDir's answer on Linux (and other Unix systems).
func unixConfigDir() (string, error) {
	if useSharedConfigDir() {
		return sharedConfigDir, nil
	}
	return userConfigDir()
}

// userConfigDir is a user's own settings folder, used until the service is
// installed.
func userConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".proxmox-backup-guardian"), nil
}

// inSharedConfigDir reports whether path is the shared settings folder or
// inside it.
func inSharedConfigDir(path string) bool {
	p := filepath.Clean(path)
	return p == sharedConfigDir || strings.HasPrefix(p, sharedConfigDir+string(filepath.Separator))
}

// sharedFileMode is the mode a settings file gets: in the shared folder the
// administrators' group may read it (never write it, and others get nothing);
// anywhere else perm is kept.
func sharedFileMode(path string, perm os.FileMode) os.FileMode {
	if !inSharedConfigDir(path) {
		return perm
	}
	return perm&0o700 | 0o040
}
