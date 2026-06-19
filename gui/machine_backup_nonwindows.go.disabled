//go:build !windows
// +build !windows

package main

import "fmt"

// PhysicalDiskInfo contains information about a physical disk
type PhysicalDiskInfo struct {
	DiskNumber int      `json:"diskNumber"`
	SizeBytes  int64    `json:"sizeBytes"`
	SizeText   string   `json:"sizeText"`
	Letters    []string `json:"letters"`
	Label      string   `json:"label"`
	Path       string   `json:"path"`
}

// ListPhysicalDisks is not supported on non-Windows platforms
func ListPhysicalDisks() ([]PhysicalDiskInfo, error) {
	return nil, fmt.Errorf("Machine backup is only supported on Windows")
}

// RunMachineBackup is not supported on non-Windows platforms
func RunMachineBackup(opts BackupOptions) error {
	return fmt.Errorf("Machine backup is only supported on Windows")
}
