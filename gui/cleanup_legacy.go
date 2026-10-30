//go:build windows
// +build windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"

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

	// The value name is the executable base name (brand-dependent). Older
	// versions also used the hard-coded "NimbusBackup" name.
	names := []string{"NimbusBackup"}
	if exePath, err := os.Executable(); err == nil {
		if name := normalizeBrandKey(filepath.Base(exePath)); name != "" && name != "nimbusbackup" {
			names = append(names, name)
		}
	}

	for _, name := range names {
		// Try to delete the value (ignore error if it doesn't exist)
		err = key.DeleteValue(name)
		if err == nil {
			writeDebugLog("Removed legacy Registry auto-start entry: " + name)
		} else if err != registry.ErrNotExist {
			// Some other error, log it
			writeDebugLog("Registry cleanup (" + name + "): " + err.Error())
		} else {
			// Entry doesn't exist - that's fine
			writeDebugLog("No legacy Registry entry found for " + name + " (already clean)")
		}
	}
}
