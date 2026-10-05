package main

import "fmt"

// machineSnapshotType maps StartMachineBackup's backupType argument to the PBS
// snapshot type. Machine backups are "host" snapshots by default: "vm" makes
// machinebackuplib also write a Proxmox VE VM config, which needs a numeric
// VMID as the backup-id (found 2026-09-24, when every hostname-style machine
// backup failed at that last step after transferring the whole disk).
// "machine-vm" is the opt-in from the one-off backup form, where
// the GUI supplies the numeric ID itself.
func machineSnapshotType(backupType string) string {
	if backupType == "machine-vm" {
		return "vm"
	}
	return "host"
}

// scheduledMachineBackupType is machineSnapshotType's input for a Backup Set:
// "machine-vm" when the set opted in to the vm snapshot type, else "machine".
func scheduledMachineBackupType(job ScheduledJob) string {
	if job.BackupType == "machine" && job.MachineAsVM {
		return "machine-vm"
	}
	return job.BackupType
}

// validateScheduledJob rejects a vm-type machine set whose backup ID is not
// numeric, so the mistake surfaces when the set is saved instead of after the
// whole disk has been transferred.
func validateScheduledJob(job ScheduledJob) error {
	if job.BackupType != "machine" || !job.MachineAsVM {
		return nil
	}
	if job.BackupID == "" {
		return fmt.Errorf("a Proxmox VE vm backup set needs a numeric VM ID")
	}
	for _, c := range job.BackupID {
		if c < '0' || c > '9' {
			return fmt.Errorf("a Proxmox VE vm backup set needs a numeric backup ID, got %q", job.BackupID)
		}
	}
	return nil
}
