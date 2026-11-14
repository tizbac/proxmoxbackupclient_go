//go:build linux
// +build linux

package main

import (
	"fmt"
	"os"
	"path/filepath"
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

	// The /dev/disk/by-id names of each disk: they come from the disk itself
	// (its serial, WWN or EUI), so they stay the same when the kernel names
	// the disk differently (sda becoming sdb).
	byID := diskByIDNames()

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

		devicePath := fmt.Sprintf("/dev/%s", entry.Name())
		deviceID := entry.Name()

		diskInfo := PhysicalDiskInfo{
			DiskNumber: 0, // Linux doesn't use disk numbers like Windows
			Size:       size,
			Model:      model,
			// IsBootDisk is set from the disk's partitions, which say where / and
			// the boot partition are (markBootDisks, disklist_types.go).
			IsBootDisk:   false,
			IsSystemDisk: false, // Linux doesn't have a direct equivalent
			DeviceID:     deviceID,
			DevicePath:   devicePath,
		}
		if names := byID[entry.Name()]; len(names) > 0 {
			diskInfo.UniqueID = diskIDPrefixByID + names[0]
			for _, n := range names[1:] {
				diskInfo.OtherIDs = append(diskInfo.OtherIDs, diskIDPrefixByID+n)
			}
		}

		disks = append(disks, diskInfo)
	}

	return dropSharedDiskIDs(disks), nil
}

// diskByIDNames maps a kernel disk name ("sda", "nvme0n1") to its names in
// /dev/disk/by-id, best first: wwn-, then scsi-, ata- and nvme-, then the
// rest. Partition links are left out. Empty when the folder does not exist.
func diskByIDNames() map[string][]string {
	out := map[string][]string{}
	entries, err := os.ReadDir(linuxByIDDir)
	if err != nil {
		return out
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), "-part") {
			continue
		}
		target, err := os.Readlink(filepath.Join(linuxByIDDir, entry.Name()))
		if err != nil {
			continue
		}
		// The link is relative: "../../sda".
		dev := filepath.Base(target)
		out[dev] = append(out[dev], entry.Name())
	}
	for dev, names := range out {
		out[dev] = sortByIDNames(names)
	}
	return out
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
