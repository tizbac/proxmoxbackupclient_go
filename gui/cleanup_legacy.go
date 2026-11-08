//go:build windows
// +build windows

package main

import (
	"os"
	"os/exec"
	"strings"
	"syscall"

	"golang.org/x/sys/windows/registry"
)

// CleanupLegacyAutoStart removes old auto-start configurations
// from previous versions that used Task Scheduler or Registry
// This should be called on app startup to migrate to MSI service
func CleanupLegacyAutoStart() {
	writeDebugLog("Cleaning up legacy auto-start configurations...")

	// 1. Remove old Task Scheduler task (if exists)
	cleanupTaskScheduler()

	// 2. Remove old Registry entry (if exists)
	cleanupRegistryEntry()

	writeDebugLog("Legacy auto-start cleanup completed")
}

func cleanupTaskScheduler() {
	taskName := "NimbusBackup"

	// Try to delete the scheduled task (ignore errors if doesn't exist)
	cmd := exec.Command("schtasks", "/Delete", "/TN", taskName, "/F")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	output, err := cmd.CombinedOutput()

	if err == nil {
		writeDebugLog("Removed legacy Task Scheduler entry")
	} else {
		// Task doesn't exist or already removed - that's fine
		writeDebugLog("No legacy Task Scheduler entry found (already clean)")
	}

	if len(output) > 0 {
		writeDebugLog("schtasks output: " + string(output))
	}
}

// cleanupRegistryEntry removes the user's own start-at-sign-in entries that
// earlier builds made (portable copies set them per user): NimbusBackup's
// own, and the names before the rename to Salvanza (ProxmoxBackupClient,
// EtitechBackup, AcmeBackup), unless one starts this very program. The
// computer-wide entry is the installer's, which an upgrade replaces.
func cleanupRegistryEntry() {
	key, err := registry.OpenKey(
		registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Run`,
		registry.QUERY_VALUE|registry.SET_VALUE,
	)
	if err != nil {
		// Can't open key - probably fine
		return
	}
	defer key.Close()

	self, _ := os.Executable()
	for _, name := range []string{"NimbusBackup", "ProxmoxBackupClient", "EtitechBackup", "AcmeBackup"} {
		cmd, _, err := key.GetStringValue(name)
		if err != nil {
			continue // not there
		}
		if self != "" && strings.Contains(strings.ToLower(cmd), strings.ToLower(self)) {
			continue
		}
		if err := key.DeleteValue(name); err == nil {
			writeDebugLog("Removed the earlier start-at-sign-in entry " + name + " (" + cmd + ")")
		} else {
			writeDebugLog("Registry cleanup: " + err.Error())
		}
	}
}
