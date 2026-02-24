//go:build linux || darwin || freebsd || openbsd
// +build linux darwin freebsd openbsd

package snapshot

func CreateVSSSnapshot(paths []string, backup_callback func(sn map[string]SnapShot) error) error {
	return backup_callback(make(map[string]SnapShot))
}

func VSSCleanup() {
}
