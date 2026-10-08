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
		// Reported (path only, never the key) so the service's own status/config
		// output shows whether scheduled backups are encrypted.
		result["encryption_key_file"] = a.config.EncryptionKeyFile
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
func (a *App) StartBackup(backupType string, backupDirs, driveLetters, excludeList []string, backupID string, useVSS bool, compression string, pbsID string) error {
	writeDebugLog(fmt.Sprintf("[Service] StartBackup called: type=%s, dirs=%v, id=%s, vss=%v, compression=%s, pbsID=%s", backupType, backupDirs, backupID, useVSS, compression, pbsID))

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

	// Kind/BackupType follow the GUI direct path (startBackupDirect /
	// startMachineBackupDirect): empty Kind + "host" = directory backup,
	// "machine" + "vm" = whole-disk machine backup (runBackupInlineInternal
	// dispatches on Kind).
	kind := ""
	pbsBackupType := "host"
	if backupType == "machine" {
		kind = "machine"
		pbsBackupType = "vm"
	}

	// Resolve the EFFECTIVE PBS config: a multi-PBS-only config keeps the legacy
	// BaseURL/AuthID/Secret/Datastore fields empty, so building options from those
	// directly yielded "PBS connection parameters required" in service mode (the GUI
	// standalone path already used EffectivePBS — audit M-01/M-04, reported in prod).
	pbsCfg, err := a.withAuth(a.config.EffectivePBS())
	if err != nil {
		return err
	}

	// Unlock the configured encryption key. Without this, SCHEDULED backups ran
	// unencrypted even when a key was configured: the scheduler goes through
	// StartBackup, which resolves to this file under -tags service, and the GUI
	// build's copy of StartBackup (main.go, !service) is the one that already
	// did this. A scheduled backup silently dropping the key also makes its own
	// snapshots unrestorable from the GUI later.
	if err := pbsCfg.loadCryptConfig(); err != nil {
		return err
	}

	// Machine backups take no exclusion list (same as the GUI direct path).
	excludes := excludeList
	if kind == "machine" {
		excludes = []string{}
	}

	// Prepare backup options
	opts := BackupOptions{
		BaseURL:         pbsCfg.BaseURL,
		AuthID:          pbsCfg.AuthID,
		Secret:          pbsCfg.Secret,
		Ticket:          pbsCfg.Ticket,
		CSRFToken:       pbsCfg.CSRFToken,
		Datastore:       pbsCfg.Datastore,
		Namespace:       pbsCfg.Namespace,
		CertFingerprint: pbsCfg.CertFingerprint,
		Crypt:           pbsCfg.Crypt,
		BackupObjects:   allDirs,
		BackupID:        backupID,
		Kind:            kind,
		BackupType:      pbsBackupType,
		UseVSS:          useVSS,
		Compression:     compression,
		ExcludeList:     excludes,
		DisableSplit:    pbsCfg.DisableSplit,
		SplitSizeBytes:  pbsCfg.SplitSizeBytes(),
		OnProgress: func(percent float64, message string) {
			writeDebugLog(fmt.Sprintf("[Backup Progress] %.1f%% - %s", percent*100, message))
			// Feed the API server's progress map so the GUI's polling sees it.
			a.dispatchProgress(percent, message)
		},
		OnComplete: func(success bool, message string) {
			if success {
				writeDebugLog(fmt.Sprintf("[Backup Complete] SUCCESS - %s", message))
			} else {
				writeDebugLog(fmt.Sprintf("[Backup Complete] FAILED - %s", message))
			}
			a.dispatchComplete(success, message)
		},
	}

	// Use the backup context that was set via SetBackupContext for this job
	a.backupCtxMu.RLock()
	opts.Ctx = a.backupCtx
	a.backupCtxMu.RUnlock()

	// Execute backup using inline implementation
	writeDebugLog("[Service] Executing backup via RunBackupInline")
	return RunBackupInline(opts)
}

// StartMachineBackup starts a whole-disk machine backup job. The service
// implementation mirrors the GUI direct path (startMachineBackupDirect):
// Kind "machine" + BackupType "host" or "vm", no exclusion list.
func (a *App) StartMachineBackup(backupType string, backupDevices []string, backupID string, useVSS bool, compression string, pbsID string, backupKind string) error {
	writeDebugLog(fmt.Sprintf("[Service] StartMachineBackup called: type=%s, devices=%v, id=%s, vss=%v, compression=%s, pbsID=%s, backupKind=%s",
		backupType, backupDevices, backupID, useVSS, compression, pbsID, backupKind))

	// Re-read config from disk (see StartBackup above).
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

	// Resolve PBS config using the specified PBS ID (or default).
	pbsCfg, err := a.resolveBackupPBS(pbsID)
	if err != nil {
		return err
	}

	// Unlock the configured encryption key for machine backups too.
	if err := pbsCfg.loadCryptConfig(); err != nil {
		return err
	}

	// Prepare backup options
	opts := BackupOptions{
		BaseURL:         pbsCfg.BaseURL,
		AuthID:          pbsCfg.AuthID,
		Secret:          pbsCfg.Secret,
		Ticket:          pbsCfg.Ticket,
		CSRFToken:       pbsCfg.CSRFToken,
		Datastore:       pbsCfg.Datastore,
		Namespace:       pbsCfg.Namespace,
		CertFingerprint: pbsCfg.CertFingerprint,
		Crypt:           pbsCfg.Crypt,
		BackupObjects:   backupDevices,
		BackupID:        backupID,
		Kind:            "machine",
		BackupType:      backupKind, // "host" or "vm" based on user selection
		UseVSS:          useVSS,
		Compression:     compression,
		ExcludeList:     []string{},
		DisableSplit:    pbsCfg.DisableSplit,
		SplitSizeBytes:  pbsCfg.SplitSizeBytes(),
		OnProgress: func(percent float64, message string) {
			writeDebugLog(fmt.Sprintf("[Machine Backup Progress] %.1f%% - %s", percent*100, message))
			// Feed the API server's progress map so the GUI's polling sees it.
			a.dispatchProgress(percent, message)
		},
		OnComplete: func(success bool, message string) {
			if success {
				writeDebugLog(fmt.Sprintf("[Machine Backup Complete] SUCCESS - %s", message))
			} else {
				writeDebugLog(fmt.Sprintf("[Machine Backup Complete] FAILED - %s", message))
			}
			a.dispatchComplete(success, message)
		},
	}

	// Use the backup context that was set via SetBackupContext for this job
	a.backupCtxMu.RLock()
	opts.Ctx = a.backupCtx
	a.backupCtxMu.RUnlock()

	// Execute backup using inline implementation
	writeDebugLog("[Service] Executing machine backup via RunBackupInline")
	return RunBackupInline(opts)
}
// resolveBackupPBS picks the PBS server to use for backup operations.
// When pbsID is empty the default PBS server is used.
// Falls back to legacy single-server fields when no multi-PBS entry is configured.
func (a *App) resolveBackupPBS(pbsID string) (*Config, error) {
	var cfg *Config

	// In service mode the GUI doesn't hold PBS credentials — it asks the
	// service for a short-lived ticket and uses that for backup.
	if a.isDelegatedToService() {
		ticket, err := a.apiClient.MintPBSTicket(pbsID)
		if err != nil {
			return nil, fmt.Errorf("service failed to mint PBS ticket: %w", err)
		}
		// Build a Config with the ticket + non-secret connection params.
		cfg := &Config{
			BaseURL:         ticket.BaseURL,
			CertFingerprint: ticket.CertFingerprint,
			Datastore:       ticket.Datastore,
			Namespace:       ticket.Namespace,
			Ticket:          ticket.Ticket,
			CSRFToken:       ticket.CSRFToken,
		}
		return cfg, nil
	}

	if pbsID != "" {
		pbs, err := a.config.GetPBSServer(pbsID)
		if err != nil {
			return nil, err
		}
		cfg = pbs.ToConfig()
	} else {
		effective := a.config.EffectivePBS()
		if err := effective.Validate(); err != nil {
			return nil, err
		}
		cfg = effective
	}
	cfg, err := a.withAuth(cfg)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}
