//go:build !windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// sharedConfigDir is the settings folder of the Linux background service, the
// counterpart of C:\ProgramData\Salvanza: /var/lib/<brand>, root's alone
// (0700, see hardenConfigDir). A variable so tests can point it elsewhere.
var sharedConfigDir = "/var/lib/" + folderNameLower()

// unixServiceStateDir is serviceStateDir on Linux (and other Unix systems).
// The service first takes over the settings of an earlier install.
func unixServiceStateDir() (string, error) {
	if serviceBuild {
		if err := adoptLinuxState(); err != nil {
			return "", err
		}
	}
	return sharedConfigDir, nil
}

// legacyStateDirs are where earlier packages kept the service's settings:
// /var/lib/proxmoxbackupclient (Proxmox Backup Client, before the rename to
// Salvanza) and /var/lib/pbsgo (the 1.2.0-rc packaging). Variables so tests
// can point them elsewhere.
var (
	legacyStateDir = "/var/lib/proxmoxbackupclient"
	pbsgoStateDir  = "/var/lib/pbsgo"
)

// adoptLinuxState gives sharedConfigDir an earlier install's settings when it
// has none yet, so its PBS servers and Backup Sets carry on: the newest of
// /var/lib/proxmoxbackupclient, /var/lib/pbsgo and another brand's folder
// (adoptFolder; it is moved). Run by the service when it starts and by Run as
// Service when it sets the shared folder up; hardenConfigDir then gives the
// folder this app's permissions. Settings already in sharedConfigDir win, and
// the earlier ones are then left alone.
func adoptLinuxState() error {
	candidates := []string{legacyStateDir, pbsgoStateDir}
	for _, f := range otherBrandFolders() {
		candidates = append(candidates, filepath.Join(filepath.Dir(sharedConfigDir), strings.ToLower(f)))
	}
	from, err := adoptFolder(sharedConfigDir, candidates)
	if err != nil {
		return fmt.Errorf("taking over the settings of an earlier install: %w", err)
	}
	if from != "" {
		fmt.Fprintf(os.Stderr, "Took over the settings of an earlier install: %s is now %s\n", from, sharedConfigDir)
	}
	return nil
}
