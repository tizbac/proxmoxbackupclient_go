// Package imagefs reads file systems inside a disk image (a machine backup's
// drive-*.img.fidx) through an io.ReaderAt, without mounting anything: the
// partition table (GPT or MBR), and NTFS volumes (listing every file from the
// master file table, and opening a file's data).
package imagefs

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf16"
)

// Partition is one entry of a disk's partition table.
type Partition struct {
	Number int    // 1-based, in table order (MBR logical partitions from 5)
	Start  int64  // byte offset on the disk
	Size   int64  // bytes
	Scheme string // "gpt" or "mbr"
	Type   string // readable type: "EFI system", "Microsoft basic data", "Linux", "0x07"...
	Name   string // GPT partition name ("Basic data partition"), "" for MBR
	FS     string // file system found at the start: "ntfs", "fat32", "exfat", "ext", "xfs", "bitlocker", "swap", "lvm", ""
}

// Label is how the GUI names a partition: "Partition 3 (NTFS, 118.4 GB)".
func (p Partition) Label() string {
	fs := strings.ToUpper(p.FS)
	switch p.FS {
	case "":
		fs = p.Type
	case "bitlocker":
		fs = "BitLocker"
	case "ext":
		fs = "ext2/3/4"
	}
	if p.Number == 0 {
		return fmt.Sprintf("Whole disk (%s, %s)", fs, humanBytes(p.Size))
	}
	return fmt.Sprintf("Partition %d (%s, %s)", p.Number, fs, humanBytes(p.Size))
}

// HumanBytes formats a size in decimal units: "127.3 GB".
func HumanBytes(n int64) string { return humanBytes(n) }

func humanBytes(n int64) string {
	const unit = 1000
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "kMGTPE"[exp])
}

// ErrNoPartitionTable means the disk starts with neither a GPT nor an MBR
// partition table (it may be a bare file system; see Probe).
var ErrNoPartitionTable = errors.New("no partition table")

// ReadPartitions reads the GPT (preferred, 512-byte or 4K sectors) or the
// MBR, including logical partitions in an extended one, and probes each
// partition for its file system.
func ReadPartitions(r io.ReaderAt, diskSize int64) ([]Partition, error) {
	head := make([]byte, 8192)
	n, err := r.ReadAt(head, 0)
	if n < 1024 {
		if err == nil {
			err = io.ErrUnexpectedEOF
		}
		return nil, fmt.Errorf("read partition table: %w", err)
	}
	head = head[:n]
	var parts []Partition
	for _, sector := range []int64{512, 4096} {
		if int64(len(head)) >= sector+92 && string(head[sector:sector+8]) == "EFI PART" {
			parts, err = readGPT(r, head[sector:sector+92], sector, diskSize)
			if err != nil {
				return nil, err
			}
			break
		}
	}
	if parts == nil {
		// A file system straight on the disk, without a partition table (a
		// "superfloppy" stick, a loop device, a disk given whole to ext4).
		// FAT and NTFS boot sectors end in 55 AA too, so look first.
		if fs := Probe(r); fs != "" && fs != "lvm" && fs != "swap" {
			return []Partition{{Number: 0, Start: 0, Size: diskSize, Scheme: "none", Type: "whole disk", FS: fs}}, nil
		}
		if head[510] != 0x55 || head[511] != 0xAA {
			return nil, ErrNoPartitionTable
		}
		parts, err = readMBR(r, head[:512], diskSize)
		if err != nil {
			return nil, err
		}
	}
	for i := range parts {
		parts[i].FS = Probe(io.NewSectionReader(r, parts[i].Start, parts[i].Size))
	}
	return parts, nil
}

func readGPT(r io.ReaderAt, hdr []byte, sector, diskSize int64) ([]Partition, error) {
	entryLBA := int64(binary.LittleEndian.Uint64(hdr[72:80]))
	count := int64(binary.LittleEndian.Uint32(hdr[80:84]))
	size := int64(binary.LittleEndian.Uint32(hdr[84:88]))
	if size < 128 || count > 1024 {
		return nil, fmt.Errorf("GPT: unexpected entry table (%d entries of %d bytes)", count, size)
	}
	table := make([]byte, count*size)
	if _, err := r.ReadAt(table, entryLBA*sector); err != nil && err != io.EOF {
		return nil, fmt.Errorf("GPT: read entries: %w", err)
	}
	var parts []Partition
	for i := int64(0); i < count; i++ {
		e := table[i*size : (i+1)*size]
		typ := guidString(e[0:16])
		if typ == "00000000-0000-0000-0000-000000000000" {
			continue
		}
		first := int64(binary.LittleEndian.Uint64(e[32:40]))
		last := int64(binary.LittleEndian.Uint64(e[40:48]))
		if last < first || (diskSize > 0 && first*sector >= diskSize) {
			continue
		}
		parts = append(parts, Partition{
			Number: int(i) + 1,
			Start:  first * sector,
			Size:   (last - first + 1) * sector,
			Scheme: "gpt",
			Type:   gptTypeName(typ),
			Name:   utf16Name(e[56:128]),
		})
	}
	return parts, nil
}

func readMBR(r io.ReaderAt, mbr []byte, diskSize int64) ([]Partition, error) {
	var parts []Partition
	var extStart int64 = -1
	for i := 0; i < 4; i++ {
		e := mbr[446+16*i : 446+16*(i+1)]
		typ := e[4]
		start := int64(binary.LittleEndian.Uint32(e[8:12])) * 512
		size := int64(binary.LittleEndian.Uint32(e[12:16])) * 512
		if typ == 0 || size == 0 {
			continue
		}
		if typ == 0xEE {
			continue // protective MBR of a GPT disk whose GPT we could not read
		}
		if typ == 0x05 || typ == 0x0F || typ == 0x85 {
			extStart = start
			continue
		}
		parts = append(parts, Partition{Number: i + 1, Start: start, Size: size, Scheme: "mbr", Type: mbrTypeName(typ)})
	}
	// Logical partitions: a chain of EBRs, each giving one partition
	// (relative to that EBR) and the next EBR (relative to the extended one).
	if extStart >= 0 {
		ebr := make([]byte, 512)
		next, num := extStart, 5
		for guard := 0; guard < 128; guard++ {
			if _, err := r.ReadAt(ebr, next); err != nil || ebr[510] != 0x55 || ebr[511] != 0xAA {
				break
			}
			e := ebr[446:462]
			if typ, size := e[4], int64(binary.LittleEndian.Uint32(e[12:16]))*512; typ != 0 && size > 0 {
				start := next + int64(binary.LittleEndian.Uint32(e[8:12]))*512
				parts = append(parts, Partition{Number: num, Start: start, Size: size, Scheme: "mbr", Type: mbrTypeName(typ)})
				num++
			}
			l := ebr[462:478]
			if l[4] == 0 {
				break
			}
			next = extStart + int64(binary.LittleEndian.Uint32(l[8:12]))*512
		}
	}
	if diskSize > 0 {
		kept := parts[:0]
		for _, p := range parts {
			if p.Start < diskSize {
				kept = append(kept, p)
			}
		}
		parts = kept
	}
	return parts, nil
}

// Probe names the file system at the start of r (a partition): "ntfs",
// "bitlocker", "fat32", "fat" (FAT12/16), "exfat", "ext", "xfs", "swap",
// "lvm", or "".
func Probe(r io.ReaderAt) string {
	b := make([]byte, 4096)
	n, _ := r.ReadAt(b, 0)
	b = b[:n]
	has := func(off int, s string) bool { return len(b) >= off+len(s) && string(b[off:off+len(s)]) == s }
	switch {
	case has(3, "NTFS    "):
		return "ntfs"
	case has(3, "-FVE-FS-"):
		return "bitlocker"
	case has(82, "FAT32   "):
		return "fat32"
	case has(54, "FAT12   "), has(54, "FAT16   "), has(54, "FAT     "):
		return "fat"
	case has(3, "EXFAT   "):
		return "exfat"
	case has(0, "XFSB"):
		return "xfs"
	case has(512, "LABELONE"), has(0, "LABELONE"):
		return "lvm"
	case has(4086, "SWAPSPACE2"), has(4086, "SWAP-SPACE"):
		return "swap"
	case len(b) >= 1024+58 && binary.LittleEndian.Uint16(b[1024+56:]) == 0xEF53:
		return "ext"
	}
	return ""
}

// guidString formats a GPT GUID (first three fields little-endian).
func guidString(b []byte) string {
	return fmt.Sprintf("%08X-%04X-%04X-%X-%X",
		binary.LittleEndian.Uint32(b[0:4]), binary.LittleEndian.Uint16(b[4:6]),
		binary.LittleEndian.Uint16(b[6:8]), b[8:10], b[10:16])
}

func utf16Name(b []byte) string {
	u := make([]uint16, 0, len(b)/2)
	for i := 0; i+1 < len(b); i += 2 {
		c := binary.LittleEndian.Uint16(b[i:])
		if c == 0 {
			break
		}
		u = append(u, c)
	}
	return string(utf16.Decode(u))
}

var gptTypes = map[string]string{
	"C12A7328-F81F-11D2-BA4B-00A0C93EC93B": "EFI system",
	"E3C9E316-0B5C-4DB8-817D-F92DF00215AE": "Microsoft reserved",
	"EBD0A0A2-B9E5-4433-87C0-68B6B72699C7": "Microsoft basic data",
	"DE94BBA4-06D1-4D40-A16A-BFD50179D6AC": "Windows recovery",
	"5808C8AA-7E8F-42E0-85D2-E1E90434CFB3": "Windows LDM metadata",
	"AF9B60A0-1431-4F62-BC68-3311714A69AD": "Windows LDM data",
	"0FC63DAF-8483-4772-8E79-3D69D8477DE4": "Linux",
	"0657FD6D-A4AB-43C4-84E5-0933C84B4F4F": "Linux swap",
	"E6D6D379-F507-44C2-A23C-238F2A3DF928": "Linux LVM",
	"A19D880F-05FC-4D3B-A006-743F0F84911E": "Linux RAID",
	"4F68BCE3-E8CD-4DB1-96E7-FBCAF984B709": "Linux root (x86-64)",
	"BC13C2FF-59E6-4262-A352-B275FD6F7172": "Linux /boot",
	"21686148-6449-6E6F-744E-656564454649": "BIOS boot",
}

func gptTypeName(guid string) string {
	if n, ok := gptTypes[guid]; ok {
		return n
	}
	return guid
}

var mbrTypes = map[byte]string{
	0x07: "NTFS/exFAT", 0x0B: "FAT32", 0x0C: "FAT32 (LBA)", 0x06: "FAT16", 0x0E: "FAT16 (LBA)",
	0x27: "Windows recovery", 0x83: "Linux", 0x82: "Linux swap", 0x8E: "Linux LVM", 0xEF: "EFI system", 0xFD: "Linux RAID",
}

func mbrTypeName(t byte) string {
	if n, ok := mbrTypes[t]; ok {
		return n
	}
	return fmt.Sprintf("0x%02X", t)
}
