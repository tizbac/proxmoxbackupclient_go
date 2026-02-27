//go:build linux || darwin || freebsd || openbsd
// +build linux darwin freebsd openbsd

package main

import (
	"fmt"
	"pbscommon"
)

func backupWindowsDisk(client *pbscommon.PBSClient, index int) error {
	return fmt.Errorf("Not supported on this platform")
}

func sysTraySetup() {

}
