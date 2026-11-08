//go:build windows

package machinebackuplib

import (
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

const ioctlStorageQueryProperty = 0x002D1400 // IOCTL_STORAGE_QUERY_PROPERTY

// storagePropertyQuery is STORAGE_PROPERTY_QUERY asking for the device
// descriptor (StorageDeviceProperty, PropertyStandardQuery).
type storagePropertyQuery struct {
	PropertyID           uint32
	QueryType            uint32
	AdditionalParameters [1]byte
}

// DiskModel returns the vendor and product name of a disk such as
// \\.\PhysicalDrive0 ("Samsung SSD 860 EVO 500GB", "BIWIN SSD"), or "" when
// Windows does not say. Opening the disk with no access rights is enough for
// the query, so it works without administrator rights.
func DiskModel(dev string) string {
	h, err := windows.CreateFile(windows.StringToUTF16Ptr(dev), 0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(h)
	q := storagePropertyQuery{}
	buf := make([]byte, 1024)
	var n uint32
	if err := windows.DeviceIoControl(h, ioctlStorageQueryProperty,
		(*byte)(unsafe.Pointer(&q)), uint32(unsafe.Sizeof(q)),
		&buf[0], uint32(len(buf)), &n, nil); err != nil || n < 24 {
		return ""
	}
	// STORAGE_DEVICE_DESCRIPTOR: VendorIdOffset at 12, ProductIdOffset at 16
	// (byte offsets into the buffer of NUL-terminated ASCII strings; 0 = none).
	str := func(at int) string {
		off := int(*(*uint32)(unsafe.Pointer(&buf[at])))
		if off <= 0 || off >= int(n) {
			return ""
		}
		end := off
		for end < int(n) && buf[end] != 0 {
			end++
		}
		return strings.TrimSpace(string(buf[off:end]))
	}
	return joinModel(str(12), str(16))
}

// volumeFileSystem returns the file system of a drive letter ("NTFS",
// "FAT32", "ReFS"), or "" when it cannot be read.
func volumeFileSystem(letter string) string {
	if letter == "" {
		return ""
	}
	root := windows.StringToUTF16Ptr(letter[:1] + `:\`)
	fs := make([]uint16, windows.MAX_PATH+1)
	if err := windows.GetVolumeInformation(root, nil, 0, nil, nil, nil, &fs[0], uint32(len(fs))); err != nil {
		return ""
	}
	return windows.UTF16ToString(fs)
}
