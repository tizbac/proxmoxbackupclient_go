// +build windows

package main

import (
	"os/exec"

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

func cleanupRegistryEntry() {
	key, err := registry.OpenKey(
		registry.CURRENT_USER,
		`Software\Microsoft\Windows\CurrentVersion\Run`,
		registry.SET_VALUE,
	)
	if err != nil {
		// Can't open key - probably fine
		return
	}
	defer key.Close()

	// Try to delete the value (ignore error if doesn't exist)
	err = key.DeleteValue("NimbusBackup")
	if err == nil {
		writeDebugLog("Removed legacy Registry auto-start entry")
	} else if err != registry.ErrNotExist {
		// Some other error, log it
		writeDebugLog("Registry cleanup: " + err.Error())
	} else {
		// Entry doesn't exist - that's fine
		writeDebugLog("No legacy Registry entry found (already clean)")
	}
}
