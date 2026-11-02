package main

import "github.com/tizbac/proxmoxbackupclient_go/gui/api"

// machineNeedsElevation reports whether a machine backup started from this
// window needs the app relaunched as administrator first (from upstream,
// which shows a "Run as Administrator" box in the machine backup screen and
// greys the disks out until then).
//
// Only on Linux, without the service: reading the disks and the snapshot
// module need root, and the service, when it runs, does both. On Windows
// the fork already says what is missing (the VSS notices and the Backup Set
// policy), and its disk list does not need the relaunch, so no box there.
func machineNeedsElevation(goos string, admin, serviceProcess bool, mode api.ExecutionMode) bool {
	if goos != "linux" || admin || serviceProcess {
		return false
	}
	return mode != api.ModeService
}
