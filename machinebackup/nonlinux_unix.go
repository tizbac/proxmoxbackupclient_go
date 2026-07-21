//go:build darwin || freebsd || openbsd || netbsd || solaris
// +build darwin freebsd openbsd netbsd solaris

package main

import "pbscommon"

func backupWholeDisk(client *pbscommon.PBSClient, dev string, index int) (bool, int64, error) {
	return false, 0, nil
}
