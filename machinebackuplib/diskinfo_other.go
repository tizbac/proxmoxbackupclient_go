//go:build !windows && !linux

package machinebackuplib

// DiskModel is not known on this platform.
func DiskModel(dev string) string { return "" }
