//go:build linux || darwin || freebsd || openbsd || netbsd || solaris
// +build linux darwin freebsd openbsd netbsd solaris

package main

import (
	"fmt"
	"pbscommon"
)

func backupWindowsDisk(client *pbscommon.PBSClient, index int) (int64, error) {
	return 0, fmt.Errorf("Not supported on this platform")
}

func sysTraySetup() {

}
