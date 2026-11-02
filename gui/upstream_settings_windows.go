//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// importUpstreamUserSettings brings across the settings of the project's
// 1.2.0-rc app run without its service, which kept them in the user's
// %USERPROFILE%\.proxmox-backup-guardian. This app keeps them in
// C:\ProgramData\ProxmoxBackupClient, shared by every user and the service,
// so after an upgrade the PBS servers and Backup Sets would have looked gone.
// On the first start that finds no settings in ProgramData but finds the
// user's, it copies config.json, scheduled_jobs.json and job_history.json
// across; the originals are left where they are. Loading then moves the
// sets' upstream fields over (migrateUpstreamJob) and seals any plain-text
// secret in config.json.
func importUpstreamUserSettings() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	src := filepath.Join(home, ".proxmox-backup-guardian")
	dst, err := getConfigDir()
	if err != nil {
		return
	}
	names := []string{"config.json", "scheduled_jobs.json", "job_history.json"}
	present := func(dir string) bool {
		for _, n := range names[:2] {
			if _, err := os.Stat(filepath.Join(dir, n)); err == nil {
				return true
			}
		}
		return false
	}
	if !present(src) || present(dst) {
		return
	}
	copied := 0
	for _, n := range names {
		data, err := os.ReadFile(filepath.Join(src, n)) // #nosec G304 -- the user's own settings folder
		if err != nil {
			continue
		}
		if err := atomicWriteFile(filepath.Join(dst, n), data, 0o600); err != nil {
			writeDebugLog(fmt.Sprintf("[Upgrade] could not copy %s from %s: %v", n, src, err))
			continue
		}
		copied++
	}
	writeDebugLog(fmt.Sprintf("[Upgrade] copied %d settings file(s) of an earlier version from %s to %s", copied, src, dst))
}
