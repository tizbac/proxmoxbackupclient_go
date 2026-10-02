package main

// machineSnapshotType maps StartMachineBackup's backupType argument to the PBS
// snapshot type. Machine backups are "host" snapshots by default: "vm" makes
// machinebackuplib also write a Proxmox VE VM config, which needs a numeric
// VMID as the backup-id (found 2026-09-24, when every hostname-style machine
// backup failed at that last step after transferring the whole disk).
// "machine-vm" is the experimental opt-in from the one-off backup form, where
// the GUI supplies the numeric ID itself.
func machineSnapshotType(backupType string) string {
	if backupType == "machine-vm" {
		return "vm"
	}
	return "host"
}
