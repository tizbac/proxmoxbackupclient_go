package machinebackuplib

import (
	"fmt"
	"slices"
)

// planDiskReads puts the partitions in disk order and fills the space before,
// between and after them with raw sections, so the disk is read once, front
// to back, and the image has exactly total bytes.
//
// Windows does not always list partitions in disk order. On an MBR disk it
// lists the primary partitions first and the logical ones (inside the
// extended partition) after them. Installing Linux alongside Windows on MBR
// puts it in logical partitions in the space freed by shrinking C:, before a
// primary partition at the end of the disk (the recovery partition). Taken in
// that order, the section before the first logical partition started after it
// ended, nothing was read for it and the backup failed with "failed to read
// partition entirely".
//
// Partitions that overlap, or that end past the end of the disk, are refused:
// they would be read twice, or read short.
func planDiskReads(parts []Partition, total uint64) ([]Partition, error) {
	sorted := slices.Clone(parts)
	slices.SortStableFunc(sorted, func(a, b Partition) int {
		switch {
		case a.StartByte < b.StartByte:
			return -1
		case a.StartByte > b.StartByte:
			return 1
		default:
			return 0
		}
	})

	plan := make([]Partition, 0, 2*len(sorted)+1)
	var curpos uint64
	for _, P := range sorted {
		if P.EndByte < P.StartByte {
			return nil, fmt.Errorf("partition at offset %d ends before it starts (%d)", P.StartByte, P.EndByte)
		}
		if P.StartByte < curpos {
			return nil, fmt.Errorf("partition at offset %d overlaps the one before it, which ends at %d", P.StartByte, curpos)
		}
		if P.EndByte > total {
			return nil, fmt.Errorf("partition at offset %d ends at %d, past the end of the disk (%d)", P.StartByte, P.EndByte, total)
		}
		if P.StartByte > curpos { // raw data between partitions
			plan = append(plan, Partition{StartByte: curpos, EndByte: P.StartByte})
		}
		plan = append(plan, P)
		curpos = P.EndByte
	}
	if curpos < total {
		plan = append(plan, Partition{StartByte: curpos, EndByte: total})
	}
	return plan, nil
}
