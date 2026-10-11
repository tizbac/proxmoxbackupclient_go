//go:build linux

package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// linuxFolderDisk finds the disk holding dir: its whole-disk device
// ("/dev/sda"), the byte offset of the partition on it (0 for a file system
// on the whole disk), and the mount point the folder is under. Folders on
// LVM, RAID or other mapped devices are not in a single partition of a disk
// image and give an error.
func linuxFolderDisk(dir string) (disk string, offset int64, mount string, err error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", 0, "", err
	}
	if r, err := filepath.EvalSymlinks(abs); err == nil {
		abs = r
	}
	f, err := os.Open("/proc/self/mountinfo")
	if err != nil {
		return "", 0, "", err
	}
	defer f.Close()
	best, majmin := "", ""
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64<<10), 1<<20)
	for sc.Scan() {
		fl := strings.Fields(sc.Text())
		if len(fl) < 5 {
			continue
		}
		mp := unescapeMountPath(fl[4])
		if (abs == mp || strings.HasPrefix(abs, strings.TrimSuffix(mp, "/")+"/")) && len(mp) > len(best) {
			best, majmin = mp, fl[2]
		}
	}
	if best == "" {
		return "", 0, "", fmt.Errorf("no mount found for %s", dir)
	}
	sys, err := filepath.EvalSymlinks("/sys/dev/block/" + majmin)
	if err != nil {
		return "", 0, "", fmt.Errorf("%s is not on a block device", best)
	}
	if _, err := os.Stat(filepath.Join(sys, "partition")); err == nil {
		b, err := os.ReadFile(filepath.Join(sys, "start"))
		if err != nil {
			return "", 0, "", err
		}
		start, err := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
		if err != nil {
			return "", 0, "", err
		}
		return "/dev/" + filepath.Base(filepath.Dir(sys)), start * 512, best, nil
	}
	if strings.HasPrefix(filepath.Base(sys), "dm-") || strings.HasPrefix(filepath.Base(sys), "md") {
		return "", 0, "", fmt.Errorf("%s is on %s (LVM, RAID or encryption), not a disk partition", best, filepath.Base(sys))
	}
	return "/dev/" + filepath.Base(sys), 0, best, nil
}

// sameBlockDevice reports whether two device paths (/dev/sda,
// /dev/disk/by-id/...) are the same device.
func sameBlockDevice(a, b string) bool {
	ra, err1 := filepath.EvalSymlinks(a)
	rb, err2 := filepath.EvalSymlinks(b)
	if err1 != nil || err2 != nil {
		return a == b
	}
	return ra == rb
}

// unescapeMountPath undoes mountinfo's octal escapes (\040 for a space).
func unescapeMountPath(s string) string {
	if !strings.Contains(s, `\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+3 < len(s) {
			if v, err := strconv.ParseUint(s[i+1:i+4], 8, 8); err == nil {
				b.WriteByte(byte(v))
				i += 3
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}
