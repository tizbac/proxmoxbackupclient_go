//go:build windows
// +build windows

package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"strconv"
	"strings"
	"unsafe"

	"machinebackuplib"

	"golang.org/x/sys/windows"
)

// ioctlStorageQueryProperty is IOCTL_STORAGE_QUERY_PROPERTY: asked for the
// STORAGE_DEVICE_DESCRIPTOR, it gives the disk's serial number.
const ioctlStorageQueryProperty = 0x002D1400

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

	// The drive letters on each disk and the disk Windows runs from, for the
	// disk picker.
	lettersByDisk := diskDriveLetters()
	bootDisk := windowsBootDiskNumber()

	// For each disk, get information
	for _, diskPath := range diskPaths {
		diskNumber, size, model, ids, err := getDiskInfo(diskPath)
		if err != nil {
			// Skip this disk if we can't get info
			continue
		}

		diskInfo := PhysicalDiskInfo{
			DiskNumber:   diskNumber,
			Size:         size,
			Model:        model,
			IsBootDisk:   bootDisk >= 0 && diskNumber == bootDisk,
			IsSystemDisk: false, // Not applicable on Windows
			DeviceID:     fmt.Sprintf("PhysicalDrive%d", diskNumber),
			DevicePath:   diskPath,
			DriveLetters: lettersByDisk[diskNumber],
		}
		if len(ids) > 0 {
			diskInfo.UniqueID, diskInfo.OtherIDs = ids[0], ids[1:]
		}

		disks = append(disks, diskInfo)
	}

	return dropSharedDiskIDs(disks), nil
}

// getDiskInfo gets the number, size, model and identities of a disk.
func getDiskInfo(diskPath string) (int64, int64, string, []string, error) {
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
		return 0, 0, "", nil, fmt.Errorf("failed to open disk %s: %w", diskPath, err)
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
		return 0, 0, "", nil, fmt.Errorf("failed to get disk size: %w", err)
	}

	model := machinebackuplib.DiskModel(diskPath)

	diskNumber, err := extractDiskNumber(diskPath)
	if err != nil {
		diskNumber = 0
	}

	return diskNumber, lengthInfo.Length, model, diskIdentities(handle, lengthInfo.Length), nil
}

// diskIdentities returns what identifies the disk itself, best first: its
// serial number, then its GPT disk GUID or MBR disk signature. When neither
// can be read, the size is returned as a weak hint that is never recorded in
// a Backup Set (isStableDiskID).
func diskIdentities(handle windows.Handle, size int64) []string {
	var ids []string
	if serial := sanitizeDiskIDValue(storageDeviceSerial(handle)); serial != "" {
		ids = append(ids, diskIDPrefixSerial+serial)
	}
	if id, ok := readDiskGUIDOrSignature(handle); ok {
		ids = append(ids, id)
	}
	if len(ids) == 0 && size > 0 {
		ids = append(ids, diskIDPrefixSize+strconv.FormatInt(size, 10))
	}
	return ids
}

// storageDeviceSerial reads the disk's serial number from its
// STORAGE_DEVICE_DESCRIPTOR (SerialNumberOffset at byte 24, the offset of a
// NUL-terminated string in the buffer; 0 means none). "" when the driver does
// not say.
func storageDeviceSerial(handle windows.Handle) string {
	// STORAGE_PROPERTY_QUERY for the device descriptor (StorageDeviceProperty,
	// PropertyStandardQuery: both 0).
	query := struct {
		PropertyID           uint32
		QueryType            uint32
		AdditionalParameters [1]byte
	}{}
	buf := make([]byte, 4096)
	var n uint32
	if err := windows.DeviceIoControl(handle, ioctlStorageQueryProperty,
		(*byte)(unsafe.Pointer(&query)), uint32(unsafe.Sizeof(query)),
		&buf[0], uint32(len(buf)), &n, nil); err != nil || n < 28 || int(n) > len(buf) {
		return ""
	}
	off := binary.LittleEndian.Uint32(buf[24:28])
	if off == 0 || off >= n {
		return ""
	}
	end := off
	for end < n && buf[end] != 0 {
		end++
	}
	return string(buf[off:end])
}

// readDiskGUIDOrSignature reads the start of the disk and returns the GPT
// disk GUID of a GPT disk, else the MBR disk signature. Both are written on
// the disk itself, so they stay the same whatever number Windows gives it;
// they change only when the disk is initialised again.
func readDiskGUIDOrSignature(handle windows.Handle) (string, bool) {
	// 8 KiB holds the GPT header both on 512-byte-sector disks (LBA 1 at
	// offset 512) and on 4Kn ones (LBA 1 at 4096), and is a whole number of
	// sectors for either.
	buf := make([]byte, 8192)
	var read uint32
	// The zero OVERLAPPED reads from offset 0.
	if err := windows.ReadFile(handle, buf, &read, &windows.Overlapped{}); err != nil {
		return "", false
	}
	if read < 1024 {
		return "", false
	}

	// GPT: the header at LBA 1 starts with "EFI PART", the disk GUID is at
	// offset 56 of it.
	for _, header := range []uint32{512, 4096} {
		if header+72 > read || string(buf[header:header+8]) != "EFI PART" {
			continue
		}
		return diskIDPrefixGUID + formatDiskGUID(buf[header+56:header+72]), true
	}
	// MBR: the 0x55AA boot signature, and the disk signature at 0x1B8.
	if buf[510] == 0x55 && buf[511] == 0xAA {
		if sig := binary.LittleEndian.Uint32(buf[0x1B8:0x1BC]); sig != 0 {
			return fmt.Sprintf("%s%08x", diskIDPrefixSig, sig), true
		}
	}
	return "", false
}

// formatDiskGUID writes a GUID as stored on disk (its first three parts
// little-endian) in the usual xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx form.
func formatDiskGUID(b []byte) string {
	return fmt.Sprintf("%08x-%04x-%04x-%02x%02x-%02x%02x%02x%02x%02x%02x",
		binary.LittleEndian.Uint32(b[0:4]),
		binary.LittleEndian.Uint16(b[4:6]),
		binary.LittleEndian.Uint16(b[6:8]),
		b[8], b[9], b[10], b[11], b[12], b[13], b[14], b[15])
}

// sanitizeDiskIDValue drops spaces and control characters, so the same disk
// always gives the same serial (drivers pad it differently).
func sanitizeDiskIDValue(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r <= ' ' || r == 0x7f {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// diskDriveLetters maps a disk number to the drive letters on it, e.g.
// 0 -> ["C:", "D:"]. A letter that cannot be placed on one disk (a network
// drive, an empty card reader, a volume spanning disks) is left out.
func diskDriveLetters() map[int64][]string {
	out := map[int64][]string{}
	mask, err := windows.GetLogicalDrives()
	if err != nil {
		return out
	}
	for i := 0; i < 26; i++ {
		if mask&(1<<uint(i)) == 0 {
			continue
		}
		letter := string(rune('A'+i)) + ":"
		if disk, _, err := volumeDiskExtent(letter + `\`); err == nil {
			out[int64(disk)] = append(out[int64(disk)], letter)
		}
	}
	return out
}

// windowsBootDiskNumber returns the number of the disk Windows runs from, or
// -1 when it cannot be told.
func windowsBootDiskNumber() int64 {
	root := os.Getenv("SystemRoot")
	if root == "" {
		return -1
	}
	disk, _, err := volumeDiskExtent(root)
	if err != nil {
		return -1
	}
	return int64(disk)
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
