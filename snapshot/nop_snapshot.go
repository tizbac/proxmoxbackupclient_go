//go:build linux || darwin || freebsd || openbsd
// +build linux darwin freebsd openbsd

package snapshot

func CreateVSSSnapshot(path string) SnapShot {
	return SnapShot{FullPath: path, Valid: false}
}

func VSSCleanup() {
}
