//go:build linux || darwin || freebsd || openbsd || netbsd || solaris
// +build linux darwin freebsd openbsd netbsd solaris

package snapshot

import "log"

func CreateVSSSnapshot(paths []string, backup_callback func(sn map[string]SnapShot) error) error {
	log.Printf("\033[31;1mWarning, on linux snapshots are not supported builtin, proceeding without!\033[0m")
	ret := make(map[string]SnapShot)
	for _, x := range paths {
		ret[x] = SnapShot{
			FullPath: x,
			Valid:    true,
		}
	}
	return backup_callback(ret)
}

func VSSCleanup() {
}
