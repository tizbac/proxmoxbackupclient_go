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

// RequestQuit is the GUI's real quit (main.go). The service has no window to
// quit and never offers "Restart as administrator" (RequestElevation, its only
// shared caller), so it does nothing here.
func (a *App) RequestQuit() {}

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

// RefreshIdleTooltip is a no-op in the service process — a Windows Service
// has no tray icon (tray.go/tray_stub.go, both built only for !service).
// scheduler.go's own once-a-minute tick calls this unconditionally in every
// build, so a stub is needed here even though there's nothing to refresh.
func (a *App) RefreshIdleTooltip() {}

// The service has no tray or window: these are no-ops there.
func (a *App) MinimizeToTray()                                           {}
func (a *App) UpdateTrayTooltip(message string)                          {}
func (a *App) ShowToastNotification(title, message string, isError bool) {}

// ReloadConfig reloads configuration from disk. The long-running service loads
// config once at startup (service.go), so without this it never sees changes
// made afterwards — a rotated PBS token, a new default PBS, or a fingerprint
// pinned from a standalone GUI — until the service is restarted. The GUI build's
// equivalent lives in main.go; both satisfy the optional ReloadConfig interface
// the API server probes after a config-changing request.
func (a *App) ReloadConfig() {
	a.config = LoadConfig()
	applySpeedLimits(a.config)
	writeDebugLog("Config reloaded from disk")
}

// StartBackup starts a backup job
// Service implementation using RunBackupInline
// postActionsKey matches main.go's StartBackup signature (required by
// api.BackupHandler). Post-backup actions do not use it here: a scheduled/
// Run-Now job running in service mode gets them fired by scheduler.go's own
// service-mode branch, using its own local job variable directly — see
// app_types.go's pendingPostActions doc comment. It does carry the run's
// change-detection mode in, and its structured result back out.
func (a *App) StartBackup(backupType string, backupDirs, driveLetters, excludeList []string, backupID string, useVSS bool, compression string, pbsServerID string, comment string, postActionsKey string) error {
	writeDebugLog(fmt.Sprintf("[Service] StartBackup called: type=%s, dirs=%v, id=%s, vss=%v, compression=%s, pbsServerID=%s", backupType, backupDirs, backupID, useVSS, compression, pbsServerID))
	// The run's entry on the service API, where a GUI follows it.
	runID := a.runIDFor(postActionsKey)

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

	// The run's compression: the one asked for, else the service's Backup
	// Options setting, else default (effectiveCompression).
	compression = a.effectiveCompression(compression)

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

	// Unlock the configured encryption key. Without this, SCHEDULED backups ran
	// unencrypted even when a key was configured: the scheduler goes through
	// StartBackup, which resolves to this file under -tags service, and the GUI
	// build's copy of StartBackup (main.go, !service) is the one that already
	// did this. A scheduled backup silently dropping the key also makes its own
	// snapshots unrestorable from the GUI later.
	if err := pbsCfg.loadCryptConfig(); err != nil {
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

	// Resolve the PBS comment — see main.go's startBackupDirect: the run's
	// own name (the scheduler passes the set's), else the generic fallback
	// matching this kind.
	if comment == "" {
		comment = fmt.Sprintf("Manual backup - %s", backupID)
		if kind == "machine" {
			comment = fmt.Sprintf("Backup machine - %s", backupID)
		}
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
		Crypt:           pbsCfg.Crypt,
		BackupObjects:   allDirs,
		BackupID:        backupID,
		Comment:         comment,
		Kind:            kind,
		BackupType:      pbsBackupType,
		UseVSS:          useVSS,
		Compression:     compression,
		ExcludeList:     excludeList,
		ShareLogins:     copyShareLogins(a.config.ShareLogins),
		DisableSplit:    pbsCfg.DisableSplit,
		SplitSizeBytes:  pbsCfg.SplitSizeBytes(),
		// Only meaningful for kind=="directory" — harmless to set unconditionally,
		// the machine-backup path (StartMachineBackup below) never reads it.
		PrefetchWorkers: pbsCfg.EffectivePrefetchWorkers(),
		// The Backup Set's change detection for this run (scheduler.go), or
		// the mode a GUI request carried (api server); "" for one-off runs.
		ChangeDetectionMode: a.runChangeDetectionMode(postActionsKey),
		OnSnapshot:          func(n snapshotNoter) { a.setRunNoter(postActionsKey, n) },
		OnProgress: func(percent float64, message string) {
			writeDebugLog(fmt.Sprintf("[Backup Progress] %.1f%% - %s", percent*100, message))
			a.reportRun(runID, percent, message)
		},
		OnComplete: func(success bool, message string, key MessageKey, params msgParams) {
			if success {
				writeDebugLog(fmt.Sprintf("[Backup Complete] SUCCESS - %s", message))
			} else {
				writeDebugLog(fmt.Sprintf("[Backup Complete] FAILED - %s", message))
			}
			// Reports and the Message Log take the run's own summary from
			// here (scheduler.go), and a GUI that asked for the run from its
			// API entry.
			a.setRunOutcome(postActionsKey, serviceRunOutcome{success: success, message: message, key: key, params: params})
			a.finishRunKeyed(runID, success, message, key, params)
		},
		OnResult: func(status *BackupStatus) { a.setRunResult(postActionsKey, status) },
		OnStats:  func(stats *BackupProgressStats) { a.reportRunStats(runID, stats) },
		OnFile:   func(path string) { a.reportRunFile(runID, path) },
	}
	if kind == "machine" {
		opts.ChangeDetectionMode = ""
	}
	// A key registered for a GUI request (RegisterRunOptions) has no
	// scheduler run to consume it.
	if v, ok := a.pendingPostActions.Load(postActionsKey); ok && v.(*pendingPostActionsEntry).job.ID == "service-request" {
		defer a.takePendingPostActions(postActionsKey)
	}

	// Execute backup using inline implementation, once its turn comes (see
	// operation_queue.go, and its doc comment for the cross-process caveat:
	// a manual restore run from a separate GUI process isn't covered here).
	opKindFor := opBackup
	if kind == "machine" {
		opKindFor = opMachine
	}
	return a.runServiceBackup(opts, kind, runID, opKindFor)
}

// runServiceBackup registers the service's run (so the API can pause or
// stop it by its entry), waits for its turn and runs it, synchronously.
func (a *App) runServiceBackup(opts BackupOptions, kind, runID string, k opKind) error {
	run := newBackupRun(opts.Comment, kind, opts.BackupID)
	run.apiID = runID
	defer run.end()
	req := opRequest{name: opts.Comment, kind: k,
		group: opGroupKey(opts.BaseURL, opts.Datastore, opts.Namespace, opts.BackupType, opts.BackupID)}
	h, err := acquireOperation(run.ctx, req, func(waitingFor string) {
		run.queued.Store(true)
		writeDebugLog(fmt.Sprintf("[Service] Queued: %s, waiting for %s", opts.Comment, waitingFor))
		a.reportRun(runID, 0, queuedMessage(opts.Comment, waitingFor))
	})
	run.queued.Store(false)
	if err != nil {
		writeDebugLog(fmt.Sprintf("[Service] %s was stopped while it waited its turn", opts.Comment))
		a.finishRun(runID, false, "Backup cancelled by user")
		return fmt.Errorf("backup cancelled by user")
	}
	defer h.Release()

	writeDebugLog(fmt.Sprintf("[Service] Executing %s backup via RunBackupInline: %s", kind, opts.Comment))
	opts.Run = run
	return RunBackupInline(opts)
}

// StartMachineBackup starts a machine (whole-disk) backup job — service
// implementation using RunBackupInline. Mirrors StartBackup's own stub
// exactly; added 2026-09-23 (the service build previously didn't implement
// this at all, failing api.BackupHandler's interface check).
// postActionsKey: see StartBackup's own doc comment above.
func (a *App) StartMachineBackup(backupType string, backupDevices []string, backupID string, useVSS bool, compression string, pbsServerID string, comment string, postActionsKey string) error {
	writeDebugLog(fmt.Sprintf("[Service] StartMachineBackup called: type=%s, devices=%v, id=%s, vss=%v, compression=%s, pbsServerID=%s", backupType, backupDevices, backupID, useVSS, compression, pbsServerID))
	runID := a.runIDFor(postActionsKey)
	// A key registered for a GUI request has no scheduler run to consume it.
	if v, ok := a.pendingPostActions.Load(postActionsKey); ok && v.(*pendingPostActionsEntry).job.ID == "service-request" {
		defer a.takePendingPostActions(postActionsKey)
	}

	a.ReloadConfig()
	if a.config == nil {
		return fmt.Errorf("configuration not loaded")
	}

	if backupID == "" {
		backupID, _ = os.Hostname()
		writeDebugLog(fmt.Sprintf("[Backup ID] Empty backup-id, using hostname: %s", backupID))
	}
	compression = a.effectiveCompression(compression) // see StartBackup
	for _, device := range backupDevices {
		if device == "" {
			return fmt.Errorf("one or more devices are empty")
		}
	}
	// A Backup Set's disks, found by their identity: from the scheduler's job
	// copy or the GUI's request (SetRunDiskIDs).
	backupDevices, skipParts, err := a.resolveRunDisks(backupDevices, postActionsKey)
	if err != nil {
		return err
	}
	// A Linux machine set with Use snapshots off: read live if accepted,
	// else stop here.
	liveRead, err := a.runLiveRead(postActionsKey)
	if err != nil {
		return err
	}

	pbsCfg, err := a.resolvePBS(pbsServerID)
	if err != nil {
		return err
	}
	// Unlock the configured encryption key, as StartBackup above does: without
	// it a machine backup run by the service went up unencrypted although a
	// key is configured.
	if err := pbsCfg.loadCryptConfig(); err != nil {
		return err
	}

	// Resolve the PBS comment — see main.go's startMachineBackupDirect for
	// the full reasoning.
	if comment == "" {
		comment = fmt.Sprintf("Backup machine - %s", backupID)
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
		Comment:         comment,
		Kind:            "machine",
		// "host" unless the "vm" type was asked for: see
		// machineSnapshotType.
		BackupType:     machineSnapshotType(backupType),
		UseVSS:         useVSS,
		Compression:    compression,
		ExcludeList:    []string{},
		DisableSplit:   pbsCfg.DisableSplit,
		SplitSizeBytes: pbsCfg.SplitSizeBytes(),
		Crypt:          pbsCfg.Crypt,
		// A machine set's forensic copy, from the scheduler's job copy or
		// the GUI's request (RegisterRunOptions).
		ReadFreeSpace:  a.runReadFreeSpace(postActionsKey),
		LiveRead:       liveRead,
		SkipPartitions: skipParts,
		OnSnapshot:     func(n snapshotNoter) { a.setRunNoter(postActionsKey, n) },
		OnProgress: func(percent float64, message string) {
			writeDebugLog(fmt.Sprintf("[Machine Backup Progress] %.1f%% - %s", percent*100, message))
			a.reportRun(runID, percent, message)
		},
		OnComplete: func(success bool, message string, key MessageKey, params msgParams) {
			if success {
				writeDebugLog(fmt.Sprintf("[Machine Backup Complete] SUCCESS - %s", message))
			} else {
				writeDebugLog(fmt.Sprintf("[Machine Backup Complete] FAILED - %s", message))
			}
			// As for a folder backup above.
			a.setRunOutcome(postActionsKey, serviceRunOutcome{success: success, message: message, key: key, params: params})
			a.finishRunKeyed(runID, success, message, key, params)
		},
		OnStats: func(stats *BackupProgressStats) { a.reportRunStats(runID, stats) },
	}

	return a.runServiceBackup(opts, "machine", runID, opMachine)
}
