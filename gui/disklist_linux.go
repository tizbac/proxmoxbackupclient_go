//go:build linux
// +build linux

package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// This function implements Linux disk listing using /sys/block and /dev
func listPhysicalDisks() ([]PhysicalDiskInfo, error) {
	disks := make([]PhysicalDiskInfo, 0)
	
	// Read from /sys/block to get disk information
	blockDir := "/sys/block"
	
	entries, err := os.ReadDir(blockDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read /sys/block: %w", err)
	}
	
	for _, entry := range entries {
		// Skip non-disks (like loop devices, ram disks, etc.)
		if !isDisk(entry.Name()) {
			continue
		}
		
		// Get disk size
		size, err := getDiskSize(entry.Name())
		if err != nil {
			continue // Skip disks we can't get size for
		}
		
		// Get model information
		model, err := getDiskModel(entry.Name())
		if err != nil {
			model = ""
		}
		
		// Check if this is a boot disk (check if / is on this disk)
		isBootDisk := isBootDisk(entry.Name())
		
		devicePath := fmt.Sprintf("/dev/%s", entry.Name())
		deviceID := entry.Name()
		
		diskInfo := PhysicalDiskInfo{
			DiskNumber:   0, // Linux doesn't use disk numbers like Windows
			Size:         size,
			Model:        model,
			IsBootDisk:   isBootDisk,
			IsSystemDisk: false, // Linux doesn't have a direct equivalent
			DeviceID:     deviceID,
			DevicePath:   devicePath,
		}
		
		disks = append(disks, diskInfo)
	}
	
	return disks, nil
}

// isDisk checks if a device is a physical disk (not a loop device or ram disk)
func isDisk(deviceName string) bool {
	// Skip loop devices, ram disks, and other virtual devices
	skipPrefixes := []string{"loop", "ram", "fd", "sr", "md", "dm"}
	
	for _, prefix := range skipPrefixes {
		if strings.HasPrefix(deviceName, prefix) {
			return false
		}
	}
	
	// Skip partitions (they end with numbers)
	if _, err := strconv.Atoi(strings.TrimPrefix(deviceName, "sd")); err == nil {
		return false
	}
	
	// Check if this is a valid disk device by checking if it has a size
	devicePath := fmt.Sprintf("/sys/block/%s/size", deviceName)
	_, err := os.Stat(devicePath)
	return err == nil
}

// getDiskSize gets the size of a disk in bytes
func getDiskSize(deviceName string) (int64, error) {
	sizePath := fmt.Sprintf("/sys/block/%s/size", deviceName)
	
	content, err := os.ReadFile(sizePath)
	if err != nil {
		return 0, fmt.Errorf("failed to read size for %s: %w", deviceName, err)
	}
	
	sizeStr := strings.TrimSpace(string(content))
	size, err := strconv.ParseInt(sizeStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse size for %s: %w", deviceName, err)
	}
	
	// Size is in 512-byte sectors, convert to bytes
	return size * 512, nil
}

// getDiskModel gets the model of a disk
func getDiskModel(deviceName string) (string, error) {
	modelPath := fmt.Sprintf("/sys/block/%s/device/model", deviceName)
	
	content, err := os.ReadFile(modelPath)
	if err != nil {
		return "", fmt.Errorf("failed to read model for %s: %w", deviceName, err)
	}
	
	model := strings.TrimSpace(string(content))
	if model == "" {
		return "Unknown", nil
	}
	
	return model, nil
}

// isBootDisk checks if a disk is the boot disk (contains root filesystem)
func isBootDisk(deviceName string) bool {
	// This is a simplified check - in practice we'd want to check mount points
	// For now, we'll check if the device is a candidate for root
	// This is a best-effort implementation
	
	// Try to find if this is used for / mount point
	// In a real implementation, this would use more robust methods
	return strings.HasPrefix(deviceName, "nvme") || strings.HasPrefix(deviceName, "sd") 
}