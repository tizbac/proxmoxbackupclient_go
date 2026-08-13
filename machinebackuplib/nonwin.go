//go:build linux || darwin || freebsd || openbsd || netbsd || solaris
// +build linux darwin freebsd openbsd netbsd solaris

package machinebackuplib

import (
	"fmt"
	"io"
	"pbscommon"
	"os"
)

// GetDiskSize returns the size of a disk or file path
func GetDiskSize(path string) (int64, error) {
	// For non-Windows platforms, we need to handle block devices specially
	// os.Stat on block devices returns 0, so we need to seek to end
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	// Get the size by seeking to the end
	size, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, err
	}

	return size, nil
}

func BackupWindowsDisk(client *pbscommon.PBSClient, index int,progressCallback ProgressCallback) (int64, error) {
	return 0, fmt.Errorf("Not supported on this platform")
}

func SysTraySetup() {

}
