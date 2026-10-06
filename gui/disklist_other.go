//go:build !windows && !linux
// +build !windows,!linux

package main

import (
	"fmt"
	"runtime"
)

// listPhysicalDisks is only implemented on Linux and Windows; machine
// backups are not supported on other platforms.
func listPhysicalDisks() ([]PhysicalDiskInfo, error) {
	return nil, fmt.Errorf("machine backups are not supported on %s", runtime.GOOS)
}
