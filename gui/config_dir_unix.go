//go:build !windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// sharedConfigDir is the settings folder of the Linux background service, the
// counterpart of C:\ProgramData\ProxmoxBackupClient: root's alone (0700, see
// hardenConfigDir). A variable so tests can point it elsewhere.
var sharedConfigDir = "/var/lib/proxmoxbackupclient"

// unixServiceStateDir is serviceStateDir on Linux (and other Unix systems).
// The service first takes over the settings of the earlier pbsgo package.
func unixServiceStateDir() (string, error) {
	if serviceBuild {
		if err := adoptPbsgoState(); err != nil {
			return "", err
		}
	}
	return sharedConfigDir, nil
}

// pbsgoStateDir is where the project's earlier pbsgo package (1.2.0-rc
// packaging) kept the service's settings. A variable so tests can point it
// elsewhere.
var pbsgoStateDir = "/var/lib/pbsgo"

// adoptPbsgoState moves a pbsgo install's settings to sharedConfigDir when
// this computer has none there yet, so its PBS servers and Backup Sets carry
// on. Run by the service when it starts and by Run as Service when it sets
// the shared folder up; hardenConfigDir then gives the folder this app's
// permissions. Settings already in sharedConfigDir win, and pbsgo's are
// then left alone.
func adoptPbsgoState() error {
	if _, err := os.Stat(sharedConfigDir); err == nil {
		return nil
	}
	if _, err := os.Stat(filepath.Join(pbsgoStateDir, "config.json")); err != nil {
		return nil
	}
	if err := os.Rename(pbsgoStateDir, sharedConfigDir); err != nil {
		return fmt.Errorf("moving the settings of the earlier pbsgo package from %s to %s: %w", pbsgoStateDir, sharedConfigDir, err)
	}
	return nil
}
