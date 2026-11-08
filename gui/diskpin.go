package main

import (
	"fmt"
	"sort"
	"strings"

	"machinebackuplib"

	"github.com/salvanza/salvanza_client/gui/api"
)

// Disk identity for machine Backup Sets. Windows numbers its disks
// (\\.\PhysicalDriveN) in the order it finds them, and Linux names them
// (/dev/sdX) the same way, so adding or removing a disk, or a driver update,
// can give a disk another number. A set saved as \\.\PhysicalDrive1 would then
// back up whichever disk is number 1 now. So a set also records what each of
// its disks carries itself (ScheduledJob.DiskIDs), and a run finds the disk
// by that (resolveSetDisks). Ported from the project's 1.2.0-rc app, which
// kept one table of these for the whole computer (config.json pinned_disks,
// see migrateUpstreamDiskPins).
const (
	diskIDPrefixSerial = "serial:" // the disk's serial number
	diskIDPrefixGUID   = "guid:"   // GPT disk GUID
	diskIDPrefixSig    = "sig:"    // MBR disk signature (4 bytes, hex)
	diskIDPrefixByID   = "byid:"   // Linux: the disk's name in /dev/disk/by-id
	diskIDPrefixSize   = "size:"   // not an identity: the size, when nothing else can be read
)

// linuxByIDDir holds a Linux computer's links to its disks by identity.
const linuxByIDDir = "/dev/disk/by-id/"

// stableDiskIDPrefixes are the identities that name one disk across restarts.
// Only these are recorded in a set or used to find a disk.
var stableDiskIDPrefixes = []string{diskIDPrefixSerial, diskIDPrefixGUID, diskIDPrefixSig, diskIDPrefixByID}

// isStableDiskID reports whether id names a disk by something the disk
// carries, rather than by how this computer numbers or sizes it.
func isStableDiskID(id string) bool {
	for _, p := range stableDiskIDPrefixes {
		if strings.HasPrefix(id, p) && len(id) > len(p) {
			return true
		}
	}
	return false
}

// diskIDsOf returns every identity the listing gave the disk, preferred first.
func diskIDsOf(d PhysicalDiskInfo) []string {
	var ids []string
	if d.UniqueID != "" {
		ids = append(ids, d.UniqueID)
	}
	return append(ids, d.OtherIDs...)
}

// dropSharedDiskIDs removes an identity that more than one disk reports: some
// USB enclosures give every disk the same serial, and a cloned disk carries
// its original's GPT GUID. Such an identity cannot tell the disks apart, so
// the disk's next identity takes its place.
func dropSharedDiskIDs(disks []PhysicalDiskInfo) []PhysicalDiskInfo {
	count := map[string]int{}
	for _, d := range disks {
		for _, id := range diskIDsOf(d) {
			count[id]++
		}
	}
	for i := range disks {
		var keep []string
		for _, id := range diskIDsOf(disks[i]) {
			if count[id] == 1 {
				keep = append(keep, id)
			}
		}
		disks[i].UniqueID, disks[i].OtherIDs = "", nil
		if len(keep) > 0 {
			disks[i].UniqueID = keep[0]
			if len(keep) > 1 {
				disks[i].OtherIDs = keep[1:]
			}
		}
	}
	return disks
}

// sortByIDNames orders a disk's /dev/disk/by-id names by how reliable they
// are, as the project's 1.2.0-rc app chose them (wwn, scsi, ata, nvme, then
// the rest), each group in name order, so a disk always gets the same first
// name.
func sortByIDNames(names []string) []string {
	rank := func(n string) int {
		for i, p := range []string{"wwn-", "scsi-", "ata-", "nvme-"} {
			if strings.HasPrefix(n, p) {
				return i
			}
		}
		return 4
	}
	out := append([]string(nil), names...)
	sort.SliceStable(out, func(i, j int) bool {
		if ri, rj := rank(out[i]), rank(out[j]); ri != rj {
			return ri < rj
		}
		return out[i] < out[j]
	})
	return out
}

// normalizePhysicalDrivePath writes a Windows disk reference the way
// machinebackuplib opens it ("\\.\PhysicalDrive3"), from "PhysicalDrive3",
// the \\?\ prefix or any case. ok is false for anything else (a Linux device,
// a file).
func normalizePhysicalDrivePath(ref string) (string, bool) {
	s := strings.TrimSpace(ref)
	for _, prefix := range []string{`\\.\`, `\\?\`, `//./`, `//?/`} {
		if len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix) {
			s = s[len(prefix):]
			break
		}
	}
	const marker = "PhysicalDrive"
	if len(s) <= len(marker) || !strings.EqualFold(s[:len(marker)], marker) {
		return "", false
	}
	num := s[len(marker):]
	for _, r := range num {
		if r < '0' || r > '9' {
			return "", false
		}
	}
	return `\\.\PhysicalDrive` + num, true
}

// canonicalDevicePath is the form two device paths are compared in: a
// Windows disk as normalizePhysicalDrivePath writes it, anything else as it is.
func canonicalDevicePath(p string) string {
	if n, ok := normalizePhysicalDrivePath(p); ok {
		return n
	}
	return strings.TrimSpace(p)
}

// resolveSetDisks finds the disks a machine Backup Set backs up among the
// disks present now. devices are the set's saved device paths and ids the
// identity recorded for each, at the same index; "" (or no entry) for a disk
// saved before identities were recorded, which is backed up by its path as
// before. A disk whose identity is now at another path is backed up there,
// and moved describes each such disk for the log. A disk whose identity is
// not present fails the run: the disk now at its old path is another one.
//
// When no disk present has an identity (the disks could not be listed, or
// this process may not open them), nothing can be checked and devices are
// returned as they are.
func resolveSetDisks(devices, ids []string, present []PhysicalDiskInfo) (resolved, moved []string, err error) {
	paths, moved, checked, err := resolveSetDiskPaths(devices, ids, present)
	if err != nil {
		return nil, nil, err
	}
	if !checked {
		return paths, nil, nil
	}
	return uniqueDevicePaths(paths), moved, nil
}

// uniqueDevicePaths drops a disk given twice.
func uniqueDevicePaths(paths []string) []string {
	resolved := make([]string, 0, len(paths))
	seen := map[string]bool{}
	for _, path := range paths {
		// Two saved disks can be one disk now (it moved to the other's
		// number); PBS refuses a disk given twice.
		if key := canonicalDevicePath(path); !seen[key] {
			seen[key] = true
			resolved = append(resolved, path)
		}
	}
	return resolved
}

// resolveSetDiskPaths is resolveSetDisks giving the path of each saved disk,
// at the same index as devices, before a disk given twice is dropped.
// checked is false when nothing could be checked and devices came back as
// they are.
func resolveSetDiskPaths(devices, ids []string, present []PhysicalDiskInfo) (paths, moved []string, checked bool, err error) {
	byID := map[string]string{}
	for _, d := range present {
		for _, id := range diskIDsOf(d) {
			if _, dup := byID[id]; isStableDiskID(id) && !dup {
				byID[id] = d.DevicePath
			}
		}
	}
	pinned := false
	for _, id := range ids {
		pinned = pinned || isStableDiskID(id)
	}
	if !pinned || len(byID) == 0 {
		return append([]string(nil), devices...), nil, false, nil
	}

	paths = make([]string, 0, len(devices))
	for i, dev := range devices {
		path := dev
		if i < len(ids) && isStableDiskID(ids[i]) {
			cur, ok := byID[ids[i]]
			if !ok {
				return nil, nil, false, fmt.Errorf("the disk this Backup Set saved as %s (%s) is not connected, and the set does not back up whichever disk is %s now: reconnect it, or edit the set and choose its disks again", dev, ids[i], dev)
			}
			if canonicalDevicePath(cur) != canonicalDevicePath(dev) {
				moved = append(moved, fmt.Sprintf("%s (%s) is %s now", dev, ids[i], cur))
			}
			path = cur
		}
		paths = append(paths, path)
	}
	return paths, moved, true, nil
}

// PartitionSkip is a partition a machine Backup Set leaves out.
type PartitionSkip = api.PartitionSkip

// skipDiskIndex finds the disk of a set (devices, and ids at the same index)
// that skip belongs to: the disk with its identity, or, for a skip saved
// without one, the disk at its device path that has no identity either. -1
// when the set has no such disk.
func skipDiskIndex(skip PartitionSkip, devices, ids []string) int {
	for i, dev := range devices {
		id := ""
		if i < len(ids) {
			id = ids[i]
		}
		if skip.DiskID != "" && id == skip.DiskID {
			return i
		}
		if skip.DiskID == "" && id == "" && canonicalDevicePath(dev) == canonicalDevicePath(skip.Device) {
			return i
		}
	}
	return -1
}

// skipsByDevice gives a run the partitions to leave out of each disk, keyed
// by the path the run backs the disk up at (paths, at the same index as the
// set's devices and ids). A skip whose disk is not in the set is left out of
// the map, so nothing is skipped for it, and named in dropped for the log.
func skipsByDevice(skips []PartitionSkip, devices, ids, paths []string) (byDev map[string][]machinebackuplib.PartitionSkip, dropped []string) {
	for _, s := range skips {
		i := skipDiskIndex(s, devices, ids)
		if i < 0 || i >= len(paths) {
			dropped = append(dropped, fmt.Sprintf("%s on %s", s.Label, s.Device))
			continue
		}
		if byDev == nil {
			byDev = map[string][]machinebackuplib.PartitionSkip{}
		}
		byDev[paths[i]] = append(byDev[paths[i]], machinebackuplib.PartitionSkip{Start: s.Start, Size: s.Size, Number: s.Number, Label: s.Label})
	}
	return byDev, dropped
}

// pinSetSkips keeps the partitions a machine set leaves out in step with its
// disks as it is saved: a skip of a disk the set no longer backs up is
// dropped, and one saved without an identity takes its disk's identity once
// the set has recorded one (pinSetDisks). A skip must name a partition:
// one without a start is dropped too.
func pinSetSkips(job *ScheduledJob) {
	if job.BackupType != "machine" {
		job.SkipPartitions = nil
		return
	}
	var keep []PartitionSkip
	for _, s := range job.SkipPartitions {
		if s.Start == 0 {
			continue
		}
		if s.DiskID == "" {
			for i, dev := range job.DriveLetters {
				if canonicalDevicePath(dev) == canonicalDevicePath(s.Device) && i < len(job.DiskIDs) {
					s.DiskID = job.DiskIDs[i]
					break
				}
			}
		}
		i := skipDiskIndex(s, job.DriveLetters, job.DiskIDs)
		if i < 0 {
			continue
		}
		s.Device = job.DriveLetters[i]
		keep = append(keep, s)
	}
	job.SkipPartitions = keep
}

// pinSetDisks records in a machine Backup Set the identity of each disk it
// backs up (DiskIDs, at the same index as DriveLetters), so its runs find the
// same disks later. An identity the set already carries for a device is kept:
// the editor sends the identity of the disk the user ticked, which is the
// right one even when that disk is not connected now. The others come from
// the disks present now. A set none of whose disks has an identity keeps
// none, and is backed up by its device paths as before.
func pinSetDisks(job *ScheduledJob, present []PhysicalDiskInfo) {
	if job.BackupType != "machine" || len(job.DriveLetters) == 0 {
		job.DiskIDs = nil
		return
	}
	byPath := map[string]string{}
	for _, d := range present {
		if isStableDiskID(d.UniqueID) {
			byPath[canonicalDevicePath(d.DevicePath)] = d.UniqueID
		}
	}
	ids := make([]string, len(job.DriveLetters))
	found := false
	for i, dev := range job.DriveLetters {
		if i < len(job.DiskIDs) && isStableDiskID(job.DiskIDs[i]) {
			ids[i] = job.DiskIDs[i]
		} else {
			ids[i] = byPath[canonicalDevicePath(dev)]
		}
		found = found || ids[i] != ""
	}
	if !found {
		ids = nil
	}
	job.DiskIDs = ids
}
