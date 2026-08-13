//go:build linux || darwin || freebsd || openbsd || netbsd || solaris
// +build linux darwin freebsd openbsd netbsd solaris

package machinebackuplib

import (
	"fmt"
	"pbscommon"
)

func BackupWindowsDisk(client *pbscommon.PBSClient, index int) (int64, error) {
	return 0, fmt.Errorf("Not supported on this platform")
}

func SysTraySetup() {

}
