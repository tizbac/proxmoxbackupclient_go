//go:build linux

package machinebackuplib

import (
	"os"
	"path/filepath"
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
	b, err := os.ReadFile("/run/udev/data/b" + majmin)
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(b), "\n") {
		if v, ok := strings.CutPrefix(line, "E:ID_FS_TYPE="); ok {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
