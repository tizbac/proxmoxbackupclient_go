//go:build darwin || freebsd || openbsd || netbsd || solaris
// +build darwin freebsd openbsd netbsd solaris

package machinebackuplib

import "pbscommon"

func backupWholeDisk(client *pbscommon.PBSClient, dev string, index int, progressCallback ProgressCallback, jp *JobProgress) (bool, int64, error) {
	return false, 0, nil
}
