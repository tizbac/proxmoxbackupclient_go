//go:build linux

package machinebackuplib

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// DiskModel returns the vendor and model of a disk such as /dev/sda or a
// /dev/disk/by-id link, from sysfs, or "" (a virtio disk has no model).
func DiskModel(dev string) string {
	if r, err := filepath.EvalSymlinks(dev); err == nil {
		dev = r
	}
	base := filepath.Join("/sys/block", filepath.Base(dev), "device")
	read := func(name string) string {
		b, err := os.ReadFile(filepath.Join(base, name))
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(b))
	}
	return joinModel(read("vendor"), read("model"))
}

// udevFileSystem reads a block device's file system type from the udev
// database (ID_FS_TYPE), for partitions that are not mounted.
func udevFileSystem(majmin string) string {
	return udevProperty(majmin, "ID_FS_TYPE")
}

// udevProperty reads one property of a block device from the udev database.
func udevProperty(majmin, key string) string {
	if majmin == "" {
		return ""
	}
	b, err := os.ReadFile("/run/udev/data/b" + majmin)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(b), "\n") {
		if v, ok := strings.CutPrefix(line, "E:"+key+"="); ok {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// ListPartitions lists the partitions of a Linux disk (/dev/sda) for the
// disk picker, from sysfs as a backup of the disk reads them, so a
// PartitionSkip saved from it matches at run time. It needs no root.
func ListPartitions(dev string) ([]PartitionInfo, error) {
	if r, err := filepath.EvalSymlinks(dev); err == nil {
		dev = r
	}
	parts, err := enumeratePartitions(filepath.Base(dev))
	if err != nil {
		return nil, err
	}
	out := make([]PartitionInfo, 0, len(parts))
	for _, p := range parts {
		info := PartitionInfo{Number: p.number, Start: p.startByte, Size: p.endByte - p.startByte, Mount: p.mountpoint, FileSystem: p.fstype}
		typ := udevProperty(p.majmin, "ID_PART_ENTRY_TYPE")
		boot := false
		switch udevProperty(p.majmin, "ID_PART_ENTRY_SCHEME") {
		case "gpt":
			g := strings.ToUpper(typ)
			info.Type = gptTypeNames[g]
			boot = g == efiSystemGUID || g == biosBootGUID
		case "dos":
			if t, err := strconv.ParseUint(strings.TrimPrefix(typ, "0x"), 16, 8); err == nil {
				info.Type = mbrTypeNames[byte(t)]
				boot = t == 0xEF
			}
		}
		switch p.mountpoint {
		case "/":
			info.Role = "system"
		case "/boot", "/boot/efi", "/efi":
			info.Role = "boot"
		default:
			if boot {
				info.Role = "boot"
			}
		}
		out = append(out, info)
	}
	return out, nil
}
