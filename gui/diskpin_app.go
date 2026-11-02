package main

import (
	"encoding/json"
	"fmt"
	"slices"
)

// runJob returns the Backup Set of the run under postActionsKey (the
// scheduler's job copy, or the one made for a GUI's request to the
// service); nil for a one-off backup.
func (a *App) runJob(postActionsKey string) *ScheduledJob {
	if postActionsKey == "" {
		return nil
	}
	v, ok := a.pendingPostActions.Load(postActionsKey)
	if !ok {
		return nil
	}
	return v.(*pendingPostActionsEntry).job
}

// runDiskIDs returns the disk identities of the run under postActionsKey: a
// machine Backup Set's DiskIDs, from the scheduler's job copy or from the
// GUI's request to the service (SetRunDiskIDs). nil for a one-off backup.
// Only the machine-backup paths read it.
func (a *App) runDiskIDs(postActionsKey string) []string {
	if job := a.runJob(postActionsKey); job != nil {
		return job.DiskIDs
	}
	return nil
}

// SetRunDiskIDs records the disk identities a GUI sent with a machine Backup
// Set run it handed to the service, on the run's key (api.RunDiskRegistrar).
func (a *App) SetRunDiskIDs(runKey string, diskIDs []string) {
	if v, ok := a.pendingPostActions.Load(runKey); ok {
		if e := v.(*pendingPostActionsEntry); e.job != nil {
			e.job.DiskIDs = append([]string(nil), diskIDs...)
		}
	}
}

// pinJobDisks records the identity of a machine Backup Set's disks as the set
// is saved (pinSetDisks), from the disks this process can list now. Listing
// may find none (a process that may not open the disks); the identities the
// editor sent are kept then.
func pinJobDisks(job *ScheduledJob) {
	var present []PhysicalDiskInfo
	if job.BackupType == "machine" && len(job.DriveLetters) > 0 {
		present, _ = listPhysicalDisks()
	}
	pinSetDisks(job, present)
}

// resolveRunDisks gives a machine backup the devices to read. For a Backup
// Set run, those are the set's disks found by their identity among the disks
// present now (resolveSetDisks): a disk the system has renumbered is backed
// up at its new number, and a disk that is not connected fails the run
// instead of the disk that has its old number. Anything else (a one-off
// backup, a set saved before identities were recorded) is used as given.
// Both places that run a machine backup call it: startMachineBackupDirect and
// the service's StartMachineBackup.
func (a *App) resolveRunDisks(devices []string, postActionsKey string) ([]string, error) {
	ids := a.runDiskIDs(postActionsKey)
	if len(ids) == 0 {
		return devices, nil
	}
	present, err := listPhysicalDisks()
	if err != nil {
		writeDebugLog(fmt.Sprintf("[DiskPin] disks could not be listed (%v): backing up %v as saved", err, devices))
		return devices, nil
	}
	resolved, moved, err := resolveSetDisks(devices, ids, present)
	if err != nil {
		writeDebugLog(fmt.Sprintf("[DiskPin] %v", err))
		return nil, err
	}
	for _, m := range moved {
		writeDebugLog(fmt.Sprintf("[DiskPin] the disk saved as %s: backing it up there", m))
	}
	if len(moved) > 0 {
		if job := a.runJob(postActionsKey); job != nil {
			a.keepMovedDisks(job.ID, devices, resolved)
		}
	}
	return resolved, nil
}

// keepMovedDisks saves a Backup Set's disks at the paths they were found at,
// so the set names them as they are now. On Windows the image's archive is
// named after the disk's number, and Undelete and Roll back find a folder's
// disk by the set's paths; both would otherwise look at the old number until
// the set is saved again. Only if the set still has the paths this run
// started from: a set edited meanwhile is left as the user saved it.
func (a *App) keepMovedDisks(jobID string, saved, found []string) {
	if jobID == "" || slices.Equal(saved, found) {
		return
	}
	jobsFileMu.Lock()
	defer jobsFileMu.Unlock()
	jobs, err := a.GetScheduledJobs()
	if err != nil {
		// Never rewrite the jobs file from a failed read.
		writeDebugLog(fmt.Sprintf("[DiskPin] set %s keeps its old disk paths: jobs not read: %v", jobID, err))
		return
	}
	for i := range jobs {
		if jobs[i].ID != jobID {
			continue
		}
		if !slices.Equal(jobs[i].DriveLetters, saved) {
			return
		}
		jobs[i].DriveLetters = append([]string(nil), found...)
		path, err := getScheduledJobsPath()
		if err != nil {
			writeDebugLog(fmt.Sprintf("[DiskPin] set %s keeps its old disk paths: %v", jobID, err))
			return
		}
		data, err := json.MarshalIndent(jobs, "", "  ")
		if err == nil {
			err = atomicWriteFile(path, data, 0600)
		}
		if err != nil {
			writeDebugLog(fmt.Sprintf("[DiskPin] set %s keeps its old disk paths: %v", jobID, err))
			return
		}
		writeDebugLog(fmt.Sprintf("[DiskPin] set %s now names its disks %v (were %v)", jobID, found, saved))
		return
	}
}
