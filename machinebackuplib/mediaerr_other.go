//go:build !windows

package machinebackuplib

// isMediaError treats every read error of a block device as a bad sector, as
// the Linux reader always has: the kernel reports them as EIO, and a device
// that vanished fails the next whole read anyway.
func isMediaError(err error) bool {
	return err != nil
}
