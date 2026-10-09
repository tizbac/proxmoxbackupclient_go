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

	"golang.org/x/sys/windows"
)

// Device / storage IOCTLs (CTL_CODE derived; see winioctl.h / ntdddisk.h).
const (
	// ioctlStorageQueryProperty asks for a STORAGE_DEVICE_DESCRIPTOR, which
	// carries the vendor/product strings and the disk's serial number.
	ioctlStorageQueryProperty = 0x002D5400
	// ioctlVolumeGetVolumeDiskExtents reports which physical disk(s) a volume
	// lives on. FILE_ANY_ACCESS, so drive letters resolve without elevation.
	ioctlVolumeGetVolumeDiskExtents = 0x00560000

	storageDeviceProperty = 0 // StorageDeviceProperty
	storageQueryStandard  = 0 // PropertyStandardQuery
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

	// Mounted volumes per disk number (C:, D:, ...) and the disk Windows itself
	// boots from — both are volume-level queries that do not need elevation.
	lettersByDisk := diskDriveLetters()
	bootDisk := windowsBootDiskNumber()

	// For each disk, get information
	for _, diskPath := range diskPaths {
		diskNumber, size, model, uniqueID, err := getDiskInfo(diskPath)
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
			UniqueID:     uniqueID,
			DriveLetters: lettersByDisk[uint32(diskNumber)],
		}

		disks = append(disks, diskInfo)
	}

	return disks, nil
}

// getDiskInfo gets the number, size, model and stable identity of a disk.
func getDiskInfo(diskPath string) (int64, int64, string, string, error) {
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
		return 0, 0, "", "", fmt.Errorf("failed to open disk %s: %w", diskPath, err)
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
		return 0, 0, "", "", fmt.Errorf("failed to get disk size: %w", err)
	}

	diskNumber, err := extractDiskNumber(diskPath)
	if err != nil {
		diskNumber = 0
	}

	// Model + a stable id for THIS disk (serial, else GPT GUID, else MBR
	// signature) — this is what a saved selection gets pinned to.
	model, uniqueID := diskIdentity(handle, lengthInfo.Length)

	return diskNumber, lengthInfo.Length, model, uniqueID, nil
}

// diskIdentity returns the disk's display model and its stable unique id. The
// id must survive reboots and re-enumeration, so it only ever comes from what
// the disk carries; when nothing readable identifies it, the size is used as an
// explicitly-unstable hint (diskpin.go refuses to pin such ids).
func diskIdentity(handle windows.Handle, size int64) (string, string) {
	// Best-effort: an unelevated process (or an odd driver) cannot answer the
	// descriptor query, and the fallbacks below still produce an id.
	vendor, product, serial, _ := storageDeviceStrings(handle)
	model := strings.TrimSpace(strings.TrimSpace(vendor) + " " + strings.TrimSpace(product))
	if model == "" {
		model = "Unknown"
	}

	if id := sanitizeDiskIDValue(serial); id != "" {
		return model, diskIDPrefixSerial + id
	}
	if id, ok := readDiskGUIDOrSignature(handle); ok {
		return model, id
	}
	if size > 0 {
		return model, diskIDPrefixSize + strconv.FormatInt(size, 10)
	}
	return model, ""
}

// storageDeviceStrings asks the driver for the STORAGE_DEVICE_DESCRIPTOR and
// pulls the NUL-terminated vendor / product / serial strings out of it.
// Layout (winioctl.h), same on 32 and 64 bit:
//
//	0 UINT32 Version | 4 UINT32 Size | 8 UCHAR DeviceType | 9 UCHAR DeviceTypeModifier
//	10 BOOLEAN RemovableMedia | 11 BOOLEAN CommandQueueing
//	12 UINT32 VendorIdOffset | 16 UINT32 ProductIdOffset
//	20 UINT32 ProductRevisionOffset | 24 UINT32 SerialNumberOffset
func storageDeviceStrings(handle windows.Handle) (vendor, product, serial string, err error) {
	query := struct {
		PropertyID uint32
		QueryType  uint32
		_          [4]byte // AdditionalParameters[1] padded to the C alignment
	}{
		PropertyID: storageDeviceProperty,
		QueryType:  storageQueryStandard,
	}

	buf := make([]byte, 4096)
	var returned uint32
	if err := windows.DeviceIoControl(
		handle,
		ioctlStorageQueryProperty,
		(*byte)(unsafe.Pointer(&query)),
		uint32(unsafe.Sizeof(query)),
		&buf[0],
		uint32(len(buf)),
		&returned,
		nil,
	); err != nil {
		return "", "", "", err
	}
	if returned < 28 || int(returned) > len(buf) {
		return "", "", "", fmt.Errorf("storage device descriptor too short (%d bytes)", returned)
	}

	str := func(offset uint32) string {
		if offset == 0 || uint64(offset) >= uint64(returned) {
			return ""
		}
		start := int(offset)
		end := start
		for end < int(returned) && buf[end] != 0 {
			end++
		}
		return string(buf[start:end])
	}
	return str(binary.LittleEndian.Uint32(buf[12:16])),
		str(binary.LittleEndian.Uint32(buf[16:20])),
		str(binary.LittleEndian.Uint32(buf[24:28])),
		nil
}

// readDiskGUIDOrSignature reads the start of the disk and derives an id from
// its partition table: the GPT disk GUID when the disk is GPT, else the MBR
// disk signature. Both live in the on-disk structure, so they are stable across
// reboots (they only change if the disk is re-initialised, and the serial
// number is preferred anyway).
func readDiskGUIDOrSignature(handle windows.Handle) (string, bool) {
	// 8 KiB covers the GPT header both on 512-byte-sector disks (LBA 1 at
	// offset 512) and on 4Kn ones (LBA 1 at offset 4096); both are multiples
	// of any logical sector size, so the read is always aligned.
	buf := make([]byte, 8192)
	var read uint32
	// Offset 0 through an OVERLAPPED with a zero offset: sector-aligned by
	// construction, and the handle is a normal buffered one.
	if err := windows.ReadFile(handle, buf, &read, &windows.Overlapped{}); err != nil {
		return "", false
	}
	if read < 1024 {
		return "", false
	}

	// GPT: the header at LBA 1 starts with "EFI PART", the disk GUID sits at
	// header offset 56.
	for _, header := range []uint32{512, 4096} {
		if uint64(header)+72 > uint64(read) || string(buf[header:header+8]) != "EFI PART" {
			continue
		}
		return diskIDPrefixGUID + formatDiskGUID(buf[header+56:header+72]), true
	}
	// MBR: 0x55AA boot signature and the 4-byte disk signature at 0x1B8.
	if buf[510] == 0x55 && buf[511] == 0xAA {
		if sig := binary.LittleEndian.Uint32(buf[0x1B8:0x1BC]); sig != 0 {
			return fmt.Sprintf("%s%08x", diskIDPrefixSig, sig), true
		}
	}
	return "", false
}

// formatDiskGUID renders a Windows GUID (mixed-endian on disk) in its usual
// xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx form.
func formatDiskGUID(b []byte) string {
	return fmt.Sprintf("%08x-%04x-%04x-%02x%02x-%02x%02x%02x%02x%02x%02x",
		binary.LittleEndian.Uint32(b[0:4]),
		binary.LittleEndian.Uint16(b[4:6]),
		binary.LittleEndian.Uint16(b[6:8]),
		b[8], b[9], b[10], b[11], b[12], b[13], b[14], b[15])
}

// sanitizeDiskIDValue strips padding whitespace and non-printables so the same
// disk always yields the same id (drivers pad serials differently depending on
// the path taken to query them).
func sanitizeDiskIDValue(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r <= ' ' || r == 0x7f {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// diskDriveLetters maps a physical disk number to the drive letters mounted on
// it, e.g. 0 -> ["C:", "D:"]. Best-effort: volumes that cannot be queried are
// simply absent, and an empty map is returned when nothing can be enumerated.
func diskDriveLetters() map[uint32][]string {
	out := make(map[uint32][]string)

	var nameBuf [512]uint16
	handle, err := windows.FindFirstVolume(&nameBuf[0], uint32(len(nameBuf)))
	if err != nil {
		return out
	}
	defer func() { _ = windows.FindVolumeClose(handle) }()

	for {
		volume := windows.UTF16ToString(nameBuf[:])
		if diskNumber, ok := volumeDiskNumber(volume); ok {
			for _, mount := range volumeMountPoints(volume) {
				// "C:\" -> "C:" — only drive-root mounts are shown, a folder
				// mount ("D:\Data\") is not what users recognise as a letter.
				if len(mount) >= 2 && mount[1] == ':' {
					letter := strings.ToUpper(mount[:2])
					if !containsString(out[diskNumber], letter) {
						out[diskNumber] = append(out[diskNumber], letter)
					}
				}
			}
		}

		nameBuf = [512]uint16{}
		if err := windows.FindNextVolume(handle, &nameBuf[0], uint32(len(nameBuf))); err != nil {
			break // ERROR_NO_MORE_FILES ends the enumeration
		}
	}
	return out
}

// volumeDiskNumber returns the physical disk a volume lives on, using
// IOCTL_VOLUME_GET_VOLUME_DISK_EXTENTS (FILE_ANY_ACCESS: works unelevated).
// DISK_EXTENTS is NumberOfDiskExtents (4) + pad (4), then DISK_EXTENT whose
// first member is DiskNumber.
func volumeDiskNumber(volume string) (uint32, bool) {
	namePtr, err := windows.UTF16PtrFromString(volume)
	if err != nil {
		return 0, false
	}
	handle, err := windows.CreateFile(
		namePtr,
		0, // query only: no access right required
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil,
		windows.OPEN_EXISTING,
		0,
		0,
	)
	if err != nil {
		return 0, false
	}
	defer windows.CloseHandle(handle)

	buf := make([]byte, 512) // room for a few extents
	var returned uint32
	if err := windows.DeviceIoControl(
		handle,
		ioctlVolumeGetVolumeDiskExtents,
		nil,
		0,
		&buf[0],
		uint32(len(buf)),
		&returned,
		nil,
	); err != nil {
		return 0, false
	}
	if returned < 12 || binary.LittleEndian.Uint32(buf[0:4]) == 0 {
		return 0, false
	}
	return binary.LittleEndian.Uint32(buf[8:12]), true
}

// volumeMountPoints lists the paths a volume is mounted at (["C:\"] typically,
// possibly several for a spanned volume, none for a volume with only a folder
// mount or none at all).
func volumeMountPoints(volume string) []string {
	namePtr, err := windows.UTF16PtrFromString(volume)
	if err != nil {
		return nil
	}

	// Multi-sz buffer: size first call is an estimate, ERROR_MORE_DATA hands
	// back the real requirement.
	buf := make([]uint16, 512)
	var need uint32
	err = windows.GetVolumePathNamesForVolumeName(namePtr, &buf[0], uint32(len(buf)), &need)
	if err == windows.ERROR_MORE_DATA || need > uint32(len(buf)) {
		if need == 0 {
			return nil
		}
		buf = make([]uint16, need)
		if err = windows.GetVolumePathNamesForVolumeName(namePtr, &buf[0], uint32(len(buf)), &need); err != nil {
			return nil
		}
	} else if err != nil {
		return nil
	}

	out := make([]string, 0, 2)
	start := 0
	for i := 0; i < len(buf); i++ {
		if buf[i] != 0 {
			continue
		}
		if i == start {
			break // end of the multi-sz list
		}
		out = append(out, windows.UTF16ToString(buf[start:i]))
		start = i + 1
	}
	return out
}

// windowsBootDiskNumber returns the disk holding %SystemRoot%, or -1 when it
// cannot be determined.
func windowsBootDiskNumber() int64 {
	windowsDir := os.Getenv("SystemRoot")
	if windowsDir == "" {
		return -1
	}
	// "C:\Windows" -> the volume root "C:\"
	if len(windowsDir) < 3 || windowsDir[1] != ':' {
		return -1
	}
	root := windowsDir[:3] + "\\"

	rootPtr, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return -1
	}
	var volName [512]uint16
	if err := windows.GetVolumeNameForVolumeMountPoint(rootPtr, &volName[0], uint32(len(volName))); err != nil {
		return -1
	}
	diskNumber, ok := volumeDiskNumber(windows.UTF16ToString(volName[:]))
	if !ok {
		return -1
	}
	return int64(diskNumber)
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

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
