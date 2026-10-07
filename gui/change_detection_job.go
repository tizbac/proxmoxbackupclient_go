package main

import (
	"encoding/json"
	"fmt"
	"strings"

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

// runScheduledFullRead reports whether the run registered under
// postActionsKey is its Backup Set's scheduled full read.
func (a *App) runScheduledFullRead(postActionsKey string) bool {
	if postActionsKey == "" {
		return false
	}
	v, ok := a.pendingPostActions.Load(postActionsKey)
	if !ok {
		return false
	}
	e := v.(*pendingPostActionsEntry)
	return e.job != nil && e.job.ScheduledFullRead
}

// nextFullReadCount is MetadataRunsSinceFullRead after a run of job ends.
// Only a successful run moves it: a full read (every file read, whether
// scheduled, a data-mode set, or no usable previous snapshot) resets it, a
// metadata run that reused files advances it. A cancelled or failed run
// leaves it as it was, so it neither brings the next full read closer nor
// stands in for one (found live 2026-10-07: a cancelled run, a failed run
// and a Stop counted, making a surprise full read 3 runs after the last).
func nextFullReadCount(job ScheduledJob, success, readEveryFile bool) int {
	if !success || job.BackupType == "machine" {
		return job.MetadataRunsSinceFullRead
	}
	if job.ChangeDetectionMode != pbscommon.ChangeDetectionMetadata || readEveryFile {
		return 0
	}
	return job.MetadataRunsSinceFullRead + 1
}

// recordChangeDetectionOutcome stores a finished run's effect on its Backup
// Set's full-read counter (nextFullReadCount). jobID "" (a one-off backup)
// does nothing.
func (a *App) recordChangeDetectionOutcome(jobID string, success, readEveryFile bool) {
	if jobID == "" || jobID == "service-request" {
		return
	}
	jobs, err := a.GetScheduledJobs()
	if err != nil {
		// Never rewrite the jobs file from a failed read: it would wipe every job.
		writeDebugLog(fmt.Sprintf("Warning: full-read counter not updated, could not load jobs: %v", err))
		return
	}
	for i := range jobs {
		if jobs[i].ID != jobID {
			continue
		}
		next := nextFullReadCount(jobs[i], success, readEveryFile)
		if next == jobs[i].MetadataRunsSinceFullRead {
			return
		}
		jobs[i].MetadataRunsSinceFullRead = next
		jobsPath, perr := getScheduledJobsPath()
		if perr != nil {
			writeDebugLog(fmt.Sprintf("Warning: cannot resolve jobs path: %v", perr))
			return
		}
		data, merr := json.MarshalIndent(jobs, "", "  ")
		if merr != nil {
			writeDebugLog(fmt.Sprintf("Warning: cannot marshal jobs: %v", merr))
			return
		}
		if werr := atomicWriteFile(jobsPath, data, 0600); werr != nil {
			writeDebugLog(fmt.Sprintf("Warning: failed to save the full-read counter: %v", werr))
		}
		return
	}
}

// isCancelledRun reports whether a run ended because the user stopped it,
// from its message key or, failing that, its error text.
func isCancelledRun(message string, key MessageKey) bool {
	return key == MsgBackupCancelled || strings.Contains(strings.ToLower(message), "cancelled by user")
}

// changeDetectionLabel is the history label for a run's mode.
func changeDetectionLabel(mode string) string {
	if mode == "" {
		return pbscommon.ChangeDetectionLegacy
	}
	return mode
}
