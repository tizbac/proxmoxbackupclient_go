package main

import (
	"fmt"

	"pbscommon"
)

// Change detection for Backup Sets. A set chooses legacy, data or metadata
// (ScheduledJob.ChangeDetectionMode); metadata sets can also read every file
// on every Nth run (FullReadEvery). Each run's effective mode is decided once,
// when the run is dispatched, and travels with the run: in the job copy
// registered under the run's postActionsKey (direct and service-scheduler
// runs) or in the service request (a run the GUI hands to the service).

// maxFullReadEvery bounds FullReadEvery (a year of daily runs).
const maxFullReadEvery = 366

// validateJobChangeDetection checks a Backup Set's change-detection settings.
func validateJobChangeDetection(job ScheduledJob) error {
	if err := pbscommon.ValidateChangeDetectionMode(job.ChangeDetectionMode); err != nil {
		return err
	}
	if job.FullReadEvery < 0 || job.FullReadEvery == 1 || job.FullReadEvery > maxFullReadEvery {
		return fmt.Errorf("full read every %d runs: use 0 (never) or 2 to %d", job.FullReadEvery, maxFullReadEvery)
	}
	return nil
}

// changeDetectionForRun returns the mode for the next run of job and the
// MetadataRunsSinceFullRead value to store once it is dispatched. Machine
// sets always get "" (disk images do not use change detection).
func changeDetectionForRun(job ScheduledJob) (mode string, nextCount int) {
	if job.BackupType == "machine" {
		return "", job.MetadataRunsSinceFullRead
	}
	switch job.ChangeDetectionMode {
	case pbscommon.ChangeDetectionMetadata:
		if job.FullReadEvery > 0 && job.MetadataRunsSinceFullRead+1 >= job.FullReadEvery {
			return pbscommon.ChangeDetectionData, 0
		}
		return pbscommon.ChangeDetectionMetadata, job.MetadataRunsSinceFullRead + 1
	case pbscommon.ChangeDetectionData:
		return pbscommon.ChangeDetectionData, 0
	default:
		return "", 0
	}
}

// runChangeDetectionMode returns the change-detection mode registered for the
// run under postActionsKey ("" for a one-off backup, which uses the classic
// format).
func (a *App) runChangeDetectionMode(postActionsKey string) string {
	if postActionsKey == "" {
		return ""
	}
	v, ok := a.pendingPostActions.Load(postActionsKey)
	if !ok {
		return ""
	}
	e := v.(*pendingPostActionsEntry)
	if e.job == nil || e.job.BackupType == "machine" {
		return ""
	}
	return e.job.ChangeDetectionMode
}

// RegisterRunOptions records the options of a backup request the GUI handed
// to the service and returns the key to run it under (api.RunOptionsRegistrar).
func (a *App) RegisterRunOptions(backupType, changeDetectionMode string) string {
	if pbscommon.ValidateChangeDetectionMode(changeDetectionMode) != nil {
		changeDetectionMode = ""
	}
	return a.registerPendingPostActions(ScheduledJob{
		ID:                  "service-request",
		BackupType:          backupType,
		ChangeDetectionMode: changeDetectionMode,
	}, "")
}

// paramUint reads a numeric completion parameter (see backup_inline.go,
// which adds reusedFiles/reusedBytes for split-archive runs).
func paramUint(p msgParams, key string) uint64 {
	switch v := p[key].(type) {
	case uint64:
		return v
	case int:
		if v > 0 {
			return uint64(v)
		}
	case float64:
		if v > 0 {
			return uint64(v)
		}
	}
	return 0
}

// changeDetectionLabel is the history label for a run's mode.
func changeDetectionLabel(mode string) string {
	if mode == "" {
		return pbscommon.ChangeDetectionLegacy
	}
	return mode
}
