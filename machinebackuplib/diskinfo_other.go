//go:build !windows && !linux

package machinebackuplib

// DiskModel is not known on this platform.
func DiskModel(dev string) string { return "" }

// ListPartitions is not supported here: the disk picker shows no partitions.
func ListPartitions(dev string) ([]PartitionInfo, error) { return nil, nil }
