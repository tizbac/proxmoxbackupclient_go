//go:build service
// +build service

// Stubs for service compilation
// These methods are required by api.BackupHandler interface
// Full implementations are in main.go (GUI mode)

package main

import (
	"fmt"
	"os"
)

// GetConfigWithHostname returns the configuration with hostname
func (a *App) GetConfigWithHostname() map[string]interface{} {
	hostname, _ := os.Hostname()
	result := map[string]interface{}{
		"hostname": hostname,
	}

	if a.config != nil {
		result["baseurl"] = a.config.BaseURL
		result["datastore"] = a.config.Datastore
		result["certfingerprint"] = a.config.CertFingerprint
		result["backup-id"] = a.config.BackupID
	}

	return result
}

// emitAnalysisProgress is a no-op in the service process (no GUI event sink).
func (a *App) emitAnalysisProgress(done, total int, scannedBytes uint64) {}

// ReloadConfig reloads configuration from disk. The long-running service loads
// config once at startup (service.go), so without this it never sees changes
// made afterwards — a rotated PBS token, a new default PBS, or a fingerprint
// pinned from a standalone GUI — until the service is restarted. The GUI build's
// equivalent lives in main.go; both satisfy the optional ReloadConfig interface
// the API server probes after a config-changing request.
func (a *App) ReloadConfig() {
	a.config = LoadConfig()
	writeDebugLog("Config reloaded from disk")
}

// StartBackup starts a backup job
// Service implementation using RunBackupInline
func (a *App) StartBackup(backupType string, backupDirs, driveLetters, excludeList []string, backupID string, useVSS bool, compression string) error {
	writeDebugLog(fmt.Sprintf("[Service] StartBackup called: type=%s, dirs=%v, id=%s, vss=%v, compression=%s", backupType, backupDirs, backupID, useVSS, compression))

	// Re-read config from disk so this run uses the current token / default PBS /
	// pinned fingerprint rather than the snapshot loaded when the service started.
	a.ReloadConfig()

	if a.config == nil {
		return fmt.Errorf("configuration not loaded")
	}

	// Use hostname as fallback if backupID is empty
	if backupID == "" {
		backupID, _ = os.Hostname()
		writeDebugLog(fmt.Sprintf("[Backup ID] Empty backup-id, using hostname: %s", backupID))
	}

	// Default to "fastest" if compression is empty
	if compression == "" {
		compression = "fastest"
		writeDebugLog("[Compression] Using default: fastest")
	}

	// Merge directories: backupDirs for directory backup, driveLetters for machine backup
	var allDirs []string
	if backupType == "directory" {
		allDirs = backupDirs
	} else if backupType == "machine" {
		allDirs = driveLetters
	}

	// Resolve the EFFECTIVE PBS config: a multi-PBS-only config keeps the legacy
	// BaseURL/AuthID/Secret/Datastore fields empty, so building options from those
	// directly yielded "PBS connection parameters required" in service mode (the GUI
	// standalone path already used EffectivePBS — audit M-01/M-04, reported in prod).
	pbsCfg := a.config.EffectivePBS()

	// Prepare backup options
	opts := BackupOptions{
		BaseURL:         pbsCfg.BaseURL,
		AuthID:          pbsCfg.AuthID,
		Secret:          pbsCfg.Secret,
		Datastore:       pbsCfg.Datastore,
		Namespace:       pbsCfg.Namespace,
		CertFingerprint: pbsCfg.CertFingerprint,
		BackupDirs:      allDirs,
		BackupID:        backupID,
		BackupType:      backupType,
		UseVSS:          useVSS,
		Compression:     compression,
		ExcludeList:     excludeList,
		DisableSplit:    pbsCfg.DisableSplit,
		SplitSizeBytes:  pbsCfg.SplitSizeBytes(),
		OnProgress: func(percent float64, message string) {
			writeDebugLog(fmt.Sprintf("[Backup Progress] %.1f%% - %s", percent, message))
		},
		OnComplete: func(success bool, message string) {
			if success {
				writeDebugLog(fmt.Sprintf("[Backup Complete] SUCCESS - %s", message))
			} else {
				writeDebugLog(fmt.Sprintf("[Backup Complete] FAILED - %s", message))
			}
		},
	}

	// Execute backup using inline implementation
	writeDebugLog("[Service] Executing backup via RunBackupInline")
	return RunBackupInline(opts)
}
