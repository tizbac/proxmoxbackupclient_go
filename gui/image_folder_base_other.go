//go:build !linux

package main

import "fmt"

// linuxFolderDisk: only Linux machine sets name their disks by device path.
func linuxFolderDisk(dir string) (disk string, offset int64, mount string, err error) {
	return "", 0, "", fmt.Errorf("%s: the set backs up no Windows disk it is on", dir)
}

// sameBlockDevice compares device paths as written.
func sameBlockDevice(a, b string) bool { return a == b }
