package main

import (
	"fmt"
	"strings"
)

// resolveMachineBackupDevices turns the device list a machine backup was asked
// to run with into the devices that exist RIGHT NOW, and remembers the mapping.
//
// What the config / a scheduled job stores is "\\.\PhysicalDrive0" — a slot
// number Windows hands out in enumeration order, so it can point at a different
// physical disk after a hardware change. Each such entry is resolved through
// Config.PinnedDisks (the disk's serial/GPT GUID/MBR signature recorded by an
// earlier run), and the current id→path pairs are written back, so the next run
// has an even better pin.
//
// Failure modes are deliberately conservative: if disks cannot be enumerated at
// all (unelevated process, platform without stable ids) the request is used
// verbatim, exactly as before pinning existed; only a request that names a disk
// which is provably NOT the wanted one fails, with a message naming both ids.
//
// Both machine-backup entry points call it: the GUI's direct path
// (startMachineBackupDirect) and the service (StartMachineBackup), which also
// covers scheduled jobs.
func (a *App) resolveMachineBackupDevices(devices []string) ([]string, error) {
	if len(devices) == 0 {
		return devices, nil
	}

	present, err := listPhysicalDisks()
	if err != nil || len(present) == 0 {
		writeDebugLog(fmt.Sprintf("[DiskPin] disk listing unavailable (%v) — keeping the saved device list as-is", err))
		return devices, nil
	}

	var pins map[string]string
	if a.config != nil && len(a.config.PinnedDisks) > 0 {
		pins = make(map[string]string, len(a.config.PinnedDisks))
		for k, v := range a.config.PinnedDisks {
			pins[k] = v
		}
	}

	resolved, err := resolvePinnedDevices(devices, present, pins)
	if err != nil {
		return nil, err
	}
	if strings.Join(resolved, ",") != strings.Join(devices, ",") {
		writeDebugLog(fmt.Sprintf("[DiskPin] resolved disk selection %v -> %v", devices, resolved))
	}

	// Record what this run proved: the id of every disk we are about to back
	// up and the path it currently has. Best-effort — a pin that cannot be
	// persisted only costs us the remapping on the next run.
	if a.config != nil && mergeDiskPins(a.config, collectDiskPins(resolved, present)) {
		if err := a.config.Save(); err != nil {
			writeDebugLog(fmt.Sprintf("[DiskPin] could not persist disk pins: %v", err))
		} else {
			writeDebugLog(fmt.Sprintf("[DiskPin] pins saved: %v", a.config.PinnedDisks))
		}
	}
	return resolved, nil
}
