//go:build windows
// +build windows

package main

import (
	"fmt"
	"strconv"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// This function implements Windows disk listing using Windows API calls
func listPhysicalDisks() ([]PhysicalDiskInfo, error) {
	disks := make([]PhysicalDiskInfo, 0)
	
	// Enumerate physical disks using Windows API
	// Get list of physical disks
	diskPaths := make([]string, 0)
	
	// Get all physical drive paths
	for i := 0; i < 100; i++ { // Arbitrary limit to prevent infinite loop
		diskPath := fmt.Sprintf("\\\\.\\PhysicalDrive%d", i)
		// Test if disk exists
		handle, err := windows.CreateFile(
			windows.StringToUTF16Ptr(diskPath),
			windows.GENERIC_READ,
			windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
			nil,
			windows.OPEN_EXISTING,
			0,
			0,
		)
		if err != nil {
			// Disk doesn't exist, move to next
			continue
		}
		windows.CloseHandle(handle)
		
		diskPaths = append(diskPaths, diskPath)
	}
	
	// For each disk, get information
	for _, diskPath := range diskPaths {
		diskNumber, size, model, err := getDiskInfo(diskPath)
		if err != nil {
			// Skip this disk if we can't get info
			continue
		}
		
		// Check if it's a boot disk (simplified - in real implementation, we'd check
		// if it contains the system partition)
		isBootDisk := false // Would be more sophisticated in real implementation
		
		diskInfo := PhysicalDiskInfo{
			DiskNumber:   diskNumber,
			Size:         size,
			Model:        model,
			IsBootDisk:   isBootDisk,
			IsSystemDisk: false, // Not applicable on Windows
			DeviceID:     fmt.Sprintf("PhysicalDrive%d", diskNumber),
			DevicePath:   diskPath,
		}
		
		disks = append(disks, diskInfo)
	}
	
	return disks, nil
}

// getDiskInfo gets information about a specific disk
func getDiskInfo(diskPath string) (int64, int64, string, error) {
	// Open the device
	handle, err := windows.CreateFile(
		windows.StringToUTF16Ptr(diskPath),
		windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil,
		windows.OPEN_EXISTING,
		0,
		0,
	)
	if err != nil {
		return 0, 0, "", fmt.Errorf("failed to open disk %s: %w", diskPath, err)
	}
	defer windows.CloseHandle(handle)
	
	// Get disk size
	var lengthInfo struct {
		Length int64
	}
	var bytesReturned uint32
	
	err = windows.DeviceIoControl(
		handle,
		0x0007405C, // IOCTL_DISK_GET_LENGTH_INFO
		nil,
		0,
		(*byte)(unsafe.Pointer(&lengthInfo)),
		uint32(unsafe.Sizeof(lengthInfo)),
		&bytesReturned,
		nil,
	)
	if err != nil {
		return 0, 0, "", fmt.Errorf("failed to get disk size: %w", err)
	}
	
	// Get disk model (simplified implementation)
	model := "Unknown"
	
	// For now, return default values - more sophisticated model retrieval would be needed
	// In practice, this would involve more complex Windows API calls
	
	diskNumber, err := extractDiskNumber(diskPath)
	if err != nil {
		diskNumber = 0
	}
	
	return diskNumber, lengthInfo.Length, model, nil
}

// extractDiskNumber extracts the disk number from a disk path
func extractDiskNumber(diskPath string) (int64, error) {
	// Extract number from "\\.\PhysicalDriveX"
	parts := strings.Split(diskPath, "\\")
	if len(parts) >= 4 {
		if strings.HasPrefix(parts[3], "PhysicalDrive") {
			numStr := strings.TrimPrefix(parts[3], "PhysicalDrive")
			num, err := strconv.ParseInt(numStr, 10, 64)
			if err != nil {
				return 0, err
			}
			return num, nil
		}
	}
	return 0, fmt.Errorf("unable to extract disk number from path")
}