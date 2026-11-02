package main

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// imageFolderBase names where a machine set's folder dir sits in the set's
// disk images, as imageSetFolders Bases are written:
// "drive-sata<n>.img.fidx@<partition offset>:<path inside the volume>".
// Windows: n is the disk number (\\.\PhysicalDriveN, how Windows machine
// backups name their images); Linux: n is the disk's place in the set's
// device list (how Linux machine backups name them).
func imageFolderBase(job ScheduledJob, dir string) (string, error) {
	disks := map[int]bool{}
	for _, d := range job.DriveLetters {
		if m := physicalDriveRe.FindStringSubmatch(d); m != nil {
			n, _ := strconv.Atoi(m[1])
			disks[n] = true
		}
	}
	if len(disks) > 0 {
		disk, offset, err := volumeDiskExtent(dir)
		if err != nil {
			return "", err
		}
		if !disks[disk] {
			return "", fmt.Errorf("it is on disk %d, which the set does not back up", disk)
		}
		rel := strings.Trim(filepath.ToSlash(strings.TrimPrefix(dir, filepath.VolumeName(dir))), "/")
		return fmt.Sprintf("drive-sata%d.img.fidx@%d:%s", disk, offset, rel), nil
	}
	disk, offset, mount, err := linuxFolderDisk(dir)
	if err != nil {
		return "", err
	}
	for i, d := range job.DriveLetters {
		if sameBlockDevice(d, disk) {
			rel, err := filepath.Rel(mount, dir)
			if err != nil {
				return "", err
			}
			rel = strings.Trim(filepath.ToSlash(rel), "/")
			if rel == "." {
				rel = ""
			}
			return fmt.Sprintf("drive-sata%d.img.fidx@%d:%s", i, offset, rel), nil
		}
	}
	return "", fmt.Errorf("it is on %s, which the set does not back up", disk)
}
