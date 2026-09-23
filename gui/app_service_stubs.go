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
func (a *App) StartBackup(backupType string, backupDirs, driveLetters, excludeList []string, backupID string, useVSS bool, compression string, pbsServerID string) error {
	writeDebugLog(fmt.Sprintf("[Service] StartBackup called: type=%s, dirs=%v, id=%s, vss=%v, compression=%s, pbsServerID=%s", backupType, backupDirs, backupID, useVSS, compression, pbsServerID))

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

	// Resolve which configured PBS server this run targets. Empty pbsServerID
	// picks the configured default (same EffectivePBS fallback this used
	// before — audit M-01/M-04, reported in prod — resolvePBS keeps that
	// behavior for the empty case and adds picking a SPECIFIC server on top).
	pbsCfg, err := a.resolvePBS(pbsServerID)
	if err != nil {
		return err
	}

	// Prepare backup options. Kind/BackupType mirror the direct-mode mapping
	// in main.go's startBackupDirect exactly (RunBackupInline branches on
	// Kind == "machine" to decide whether to do a machine-type backup at
	// all — this stub never set it before, so a machine backup routed
	// through the service silently fell through to the directory path;
	// found and fixed 2026-09-23 alongside the service build's other
	// pre-existing compile errors).
	// BackupType is "host" even for machine backups — see the matching
	// comment in main.go's startBackupDirect for why "vm" is wrong here
	// (found 2026-09-24: it requires a numeric VMID backup-id, which every
	// machine backup here defaults to a hostname instead of).
	kind := "directory"
	if backupType == "machine" {
		kind = "machine"
	}
	pbsBackupType := "host"
	opts := BackupOptions{
		BaseURL:         pbsCfg.BaseURL,
		AuthID:          pbsCfg.AuthID,
		Secret:          pbsCfg.Secret,
		Ticket:          pbsCfg.Ticket,
		CSRFToken:       pbsCfg.CSRFToken,
		Datastore:       pbsCfg.Datastore,
		Namespace:       pbsCfg.Namespace,
		CertFingerprint: pbsCfg.CertFingerprint,
		BackupObjects:   allDirs,
		BackupID:        backupID,
		Kind:            kind,
		BackupType:      pbsBackupType,
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

	// Execute backup using inline implementation. Queues behind any
	// backup/restore already running in THIS process — see
	// operation_queue.go's doc comment for the cross-process caveat (a
	// manual restore run from a separate GUI process isn't covered here).
	release := acquireOperationSlot(fmt.Sprintf("backup of %s", backupID), func(heldBy string) {
		writeDebugLog(fmt.Sprintf("[Service] Queued — waiting for %s to finish", heldBy))
	})
	defer release()

	writeDebugLog("[Service] Executing backup via RunBackupInline")
	return RunBackupInline(opts)
}

// StartMachineBackup starts a machine (whole-disk) backup job — service
// implementation using RunBackupInline. Mirrors StartBackup's own stub
// exactly; added 2026-09-23 (the service build previously didn't implement
// this at all, failing api.BackupHandler's interface check).
func (a *App) StartMachineBackup(backupType string, backupDevices []string, backupID string, useVSS bool, compression string, pbsServerID string) error {
	writeDebugLog(fmt.Sprintf("[Service] StartMachineBackup called: type=%s, devices=%v, id=%s, vss=%v, compression=%s, pbsServerID=%s", backupType, backupDevices, backupID, useVSS, compression, pbsServerID))

	a.ReloadConfig()
	if a.config == nil {
		return fmt.Errorf("configuration not loaded")
	}

	if backupID == "" {
		backupID, _ = os.Hostname()
		writeDebugLog(fmt.Sprintf("[Backup ID] Empty backup-id, using hostname: %s", backupID))
	}
	if compression == "" {
		compression = "fastest"
		writeDebugLog("[Compression] Using default: fastest")
	}
	for _, device := range backupDevices {
		if device == "" {
			return fmt.Errorf("one or more devices are empty")
		}
	}

	pbsCfg, err := a.resolvePBS(pbsServerID)
	if err != nil {
		return err
	}

	opts := BackupOptions{
		BaseURL:         pbsCfg.BaseURL,
		AuthID:          pbsCfg.AuthID,
		Secret:          pbsCfg.Secret,
		Ticket:          pbsCfg.Ticket,
		CSRFToken:       pbsCfg.CSRFToken,
		Datastore:       pbsCfg.Datastore,
		Namespace:       pbsCfg.Namespace,
		CertFingerprint: pbsCfg.CertFingerprint,
		BackupObjects:   backupDevices,
		BackupID:        backupID,
		Kind:            "machine",
		// "host", not "vm" — see startBackupDirect's machine branch in
		// main.go for why: "vm" requires a numeric VMID backup-id, which
		// every machine backup here defaults to a hostname instead of.
		BackupType:     "host",
		UseVSS:         useVSS,
		Compression:    compression,
		ExcludeList:    []string{},
		DisableSplit:   pbsCfg.DisableSplit,
		SplitSizeBytes: pbsCfg.SplitSizeBytes(),
		OnProgress: func(percent float64, message string) {
			writeDebugLog(fmt.Sprintf("[Machine Backup Progress] %.1f%% - %s", percent, message))
		},
		OnComplete: func(success bool, message string) {
			if success {
				writeDebugLog(fmt.Sprintf("[Machine Backup Complete] SUCCESS - %s", message))
			} else {
				writeDebugLog(fmt.Sprintf("[Machine Backup Complete] FAILED - %s", message))
			}
		},
	}

	release := acquireOperationSlot(fmt.Sprintf("machine backup of %s", backupID), func(heldBy string) {
		writeDebugLog(fmt.Sprintf("[Service] Queued — waiting for %s to finish", heldBy))
	})
	defer release()

	writeDebugLog("[Service] Executing machine backup via RunBackupInline")
	return RunBackupInline(opts)
}
