package machinebackuplib

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"pbscommon"
)

// PartitionSkip names a partition that a machine backup leaves out
// (tizbac/proxmoxbackupclient_go#132, #64). It is not read and not
// snapshotted, and goes into the image as zeros. The partition table is kept,
// so a restore gives the partition back in its place and at its size, but
// empty and without a file system: Windows shows it as RAW and offers to
// format it, Linux sees an empty partition. The zeros cost next to nothing to
// store or send, as free space does.
//
// It matches the partition that starts at Start and is Size bytes long. A
// skip saved before the partition table changed matches nothing, and that
// partition is backed up (skipsMatch). Start 0, where no partition starts,
// means the partition numbered Number instead, as -skippartition gives it.
type PartitionSkip struct {
	Start  uint64 `json:"start,omitempty"`
	Size   uint64 `json:"size,omitempty"`
	Number int    `json:"number,omitempty"`
	// Label names the partition in the log ("C: (NTFS)", "partition 3").
	Label string `json:"label,omitempty"`
}

func (s PartitionSkip) label() string {
	if s.Label != "" {
		return s.Label
	}
	return fmt.Sprintf("partition %d", s.Number)
}

// PartitionInfo is one partition of a disk, as the disk picker lists it
// (ListPartitions). Start and Size are what a PartitionSkip saved from it
// matches.
type PartitionInfo struct {
	Number     int    `json:"number"`
	Start      uint64 `json:"start"`
	Size       uint64 `json:"size"`
	Mount      string `json:"mount,omitempty"`       // "C:" or "/home"; "" when not mounted
	FileSystem string `json:"file_system,omitempty"` // "NTFS", "ext4"...; "" when not known
	Type       string `json:"type,omitempty"`        // "EFI system", "Windows recovery"...; "" when not known
	// Role is "system" for the partition the running system is on (the
	// Windows volume, or /), "boot" for an EFI system or boot partition, else
	// "". A disk restored without either does not start.
	Role string `json:"role,omitempty"`
}

// partRange is a partition as skipsMatch sees it.
type partRange struct {
	start, end uint64
	number     int
}

// skipsMatch reports, for each partition, whether one of skips names it. A
// skip that names no partition of the disk gives a warning, and nothing is
// left out for it: a partition is only left out when a skip names it exactly.
func skipsMatch(dev string, parts []partRange, skips []PartitionSkip) ([]bool, []string) {
	skip := make([]bool, len(parts))
	var warnings []string
	for _, s := range skips {
		found := false
		for i, p := range parts {
			if p.number <= 0 {
				continue // space between partitions
			}
			if s.Start == 0 {
				found = s.Number > 0 && p.number == s.Number
			} else {
				found = p.start == s.Start && (s.Size == 0 || p.end-p.start == s.Size)
			}
			if found {
				skip[i] = true
				break
			}
		}
		if found {
			continue
		}
		if s.Start == 0 {
			warnings = append(warnings, fmt.Sprintf("WARNING: %s has no partition %d to leave out", dev, s.Number))
		} else {
			warnings = append(warnings, fmt.Sprintf("WARNING: %s: the Backup Set leaves out %s, but no partition of %s starts at byte %d now (the partition table has changed): backing it up. Edit the set to choose again.",
				dev, s.label(), BytesToString(int64(s.Size)), s.Start))
		}
	}
	return skip, warnings
}

// markSkipped sets Skip on the partitions of a Windows disk that skips name,
// and takes them off the VSS snapshot: a partition left out is neither read
// nor snapshotted. It returns lines for the run log: each partition left out,
// and a warning for each skip that matched none.
func markSkipped(dev string, parts []Partition, skips []PartitionSkip) []string {
	if len(skips) == 0 {
		return nil
	}
	ranges := make([]partRange, len(parts))
	for i, p := range parts {
		ranges[i] = partRange{start: p.StartByte, end: p.EndByte, number: p.Number}
	}
	skip, msgs := skipsMatch(dev, ranges, skips)
	for i := range parts {
		if skip[i] {
			parts[i].Skip = true
			parts[i].RequiresVSS = false
			msgs = append(msgs, leavingOut(dev, parts[i].baseLabel(), parts[i].EndByte-parts[i].StartByte))
		}
	}
	return msgs
}

func leavingOut(dev, label string, size uint64) string {
	return fmt.Sprintf("%s: leaving out %s (%s) as the Backup Set asks: not read, it goes into the image as zeros and comes back empty and unformatted on restore",
		dev, label, BytesToString(int64(size)))
}

// vssPaths are the volumes of a Windows disk to snapshot: those with a drive
// letter, apart from the ones left out.
func vssPaths(parts []Partition) []string {
	paths := make([]string, 0)
	for _, p := range parts {
		if p.RequiresVSS && !p.Skip {
			paths = append(paths, fmt.Sprintf("%s:\\\\", p.Letter))
		}
	}
	return paths
}

// snapshotStep is the progress line before a Windows disk is read. A disk
// with no volume to snapshot (every lettered volume left out, or none with a
// drive letter) takes no VSS snapshot, so the line says none is needed.
func snapshotStep(dev string, parts []Partition) string {
	if len(vssPaths(parts)) > 0 {
		return "Creating VSS snapshot..."
	}
	if slices.ContainsFunc(parts, func(p Partition) bool { return p.Skip && p.Letter != "" }) {
		return fmt.Sprintf("%s: no snapshot needed, every volume of this disk with a drive letter is left out", dev)
	}
	return fmt.Sprintf("%s: no snapshot needed, no volume of this disk has a drive letter", dev)
}

// leaveOut writes length bytes of a partition left out as zeros, 4 MiB at a
// time, and counts them in jp.LeftOut as it goes, as free space is counted.
// The zeros move the read position like any other data, so the progress
// bar reaches the end of the disk. It returns false when emitZeros stops.
func leaveOut(length uint64, emitZeros func(uint64) bool, jp *JobProgress) bool {
	for w := uint64(0); w < length; w += pbscommon.PBS_FIXED_CHUNK_SIZE {
		n := min(uint64(pbscommon.PBS_FIXED_CHUNK_SIZE), length-w)
		if !emitZeros(n) {
			return false
		}
		if jp != nil {
			jp.LeftOut.Add(n)
		}
	}
	return true
}

// skipsFor returns the partitions to leave out of dev. A Windows disk is
// found however its \\.\PhysicalDriveN is written.
func skipsFor(all map[string][]PartitionSkip, dev string) []PartitionSkip {
	if s, ok := all[dev]; ok {
		return s
	}
	for k, s := range all {
		if sameDevice(k, dev) {
			return s
		}
	}
	return nil
}

func sameDevice(a, b string) bool {
	if a == b {
		return true
	}
	if !isPhysicalDrive(a) || !isPhysicalDrive(b) {
		return false
	}
	ia, ea := physicalDriveIndex(a)
	ib, eb := physicalDriveIndex(b)
	return ea == nil && eb == nil && ia == ib
}

// Partition types the disk picker names, and the ones a disk needs to start.
const (
	efiSystemGUID = "C12A7328-F81F-11D2-BA4B-00A0C93EC93B"
	biosBootGUID  = "21686148-6449-6E6F-744E-656564454649"
)

var gptTypeNames = map[string]string{
	efiSystemGUID:                          "EFI system",
	biosBootGUID:                           "BIOS boot",
	"E3C9E316-0B5C-4DB8-817D-F92DF00215AE": "Microsoft reserved",
	"EBD0A0A2-B9E5-4433-87C0-68B6B72699C7": "Microsoft basic data",
	"DE94BBA4-06D1-4D40-A16A-BFD50179D6AC": "Windows recovery",
	"0FC63DAF-8483-4772-8E79-3D69D8477DE4": "Linux",
	"0657FD6D-A4AB-43C4-84E5-0933C84B4F4F": "Linux swap",
	"E6D6D379-F507-44C2-A23C-238F2A3DF928": "Linux LVM",
	"A19D880F-05FC-4D3B-A006-743F0F84911E": "Linux RAID",
	"4F68BCE3-E8CD-4DB1-96E7-FBCAF984B709": "Linux root (x86-64)",
	"BC13C2FF-59E6-4262-A352-B275FD6F7172": "Linux /boot",
}

var mbrTypeNames = map[byte]string{
	0x07: "NTFS/exFAT", 0x0B: "FAT32", 0x0C: "FAT32", 0x06: "FAT16", 0x0E: "FAT16",
	0x27: "Windows recovery", 0x83: "Linux", 0x82: "Linux swap", 0x8E: "Linux LVM", 0xEF: "EFI system", 0xFD: "Linux RAID",
}

// guidString writes a GUID as a partition table stores it (its first three
// parts little-endian) in the usual form, in capitals.
func guidString(b []byte) string {
	return fmt.Sprintf("%08X-%04X-%04X-%X-%X",
		uint32(b[0])|uint32(b[1])<<8|uint32(b[2])<<16|uint32(b[3])<<24,
		uint16(b[4])|uint16(b[5])<<8, uint16(b[6])|uint16(b[7])<<8, b[8:10], b[10:16])
}

// ParseSkipPartition reads -skippartition <device>:<number>, e.g.
// \\.\PhysicalDrive1:3 or /dev/sda:2.
func ParseSkipPartition(v string) (string, PartitionSkip, error) {
	i := strings.LastIndex(v, ":")
	if i <= 0 || i == len(v)-1 {
		return "", PartitionSkip{}, fmt.Errorf("-skippartition %q: want <device>:<partition number>, e.g. /dev/sda:2", v)
	}
	n, err := strconv.Atoi(v[i+1:])
	if err != nil || n <= 0 {
		return "", PartitionSkip{}, fmt.Errorf("-skippartition %q: %q is not a partition number", v, v[i+1:])
	}
	return v[:i], PartitionSkip{Number: n}, nil
}
