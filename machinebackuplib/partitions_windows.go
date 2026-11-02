//go:build windows

package machinebackuplib

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// ListPartitions lists the partitions of a Windows disk (\\.\PhysicalDriveN)
// for the disk picker. It reads the same drive layout a backup of the disk
// reads, so a PartitionSkip saved from it matches at run time.
func ListPartitions(dev string) ([]PartitionInfo, error) {
	if !isPhysicalDrive(dev) {
		return nil, nil
	}
	index, err := physicalDriveIndex(dev)
	if err != nil {
		return nil, err
	}
	// Reading the layout needs no access rights, so this works without
	// administrator rights too.
	h, err := windows.CreateFile(windows.StringToUTF16Ptr(dev), 0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", dev, err)
	}
	defer windows.CloseHandle(h)
	var layout DRIVE_LAYOUT_INFORMATION_EX
	var n uint32
	if err := windows.DeviceIoControl(h, IOCTL_DISK_GET_DRIVE_LAYOUT_EX, nil, 0,
		(*byte)(unsafe.Pointer(&layout)), uint32(unsafe.Sizeof(layout)), &n, nil); err != nil {
		return nil, fmt.Errorf("read the partition table of %s: %w", dev, err)
	}

	vols, _ := enumVolumeDiskOffset()
	sysLetter := strings.TrimSuffix(os.Getenv("SystemDrive"), ":")
	out := make([]PartitionInfo, 0)
	for i := 0; i < int(layout.PartitionCount) && i < len(layout.PartitionEntry); i++ {
		E := layout.PartitionEntry[i]
		if E.PartitionNumber == 0 {
			continue // an unused MBR slot or the extended partition's container
		}
		p := PartitionInfo{Number: int(E.PartitionNumber), Start: E.StartingOffset, Size: E.PartitionLength}
		if letter := partitionLetter(p.Start, p.Start+p.Size, vols, index); letter != "" {
			p.Mount = letter + ":"
			p.FileSystem = volumeFileSystem(letter)
		}
		var boot bool
		p.Type, boot = windowsPartitionType(&E)
		switch {
		case sysLetter != "" && strings.EqualFold(p.Mount, sysLetter+":"):
			p.Role = "system"
		case boot:
			p.Role = "boot"
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Start < out[j].Start })
	return out, nil
}

// windowsPartitionType names a partition's type, and reports whether the
// disk needs it to start: the EFI system partition, or on an MBR disk the
// active one (the "System Reserved" partition of a BIOS install).
func windowsPartitionType(E *PARTITION_INFORMATION_EX) (string, bool) {
	// The MBR/GPT union starts at byte 32 of PARTITION_INFORMATION_EX, two
	// bytes into Padding.
	u := unsafe.Slice((*byte)(unsafe.Add(unsafe.Pointer(E), 32)), 18)
	switch E.PartitionStyle {
	case PartitionStyleGPT:
		g := guidString(u[:16])
		return gptTypeNames[g], g == efiSystemGUID
	case PartitionStyleMBR:
		// PartitionType, then BootIndicator.
		return mbrTypeNames[u[0]], u[0] == 0xEF || u[1] != 0
	}
	return "", false
}
