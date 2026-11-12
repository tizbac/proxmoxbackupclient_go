//go:build linux
// +build linux

package snapshot

import (
	"fmt"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// minCowFreePercent is how much of a file system must be free before a
// snapshot is taken. elastio-snap and dattobd keep the snapshot's
// copy-on-write file (CowFile) on the file system they snapshot, and it grows
// with the blocks that change while the backup reads. With less than 10% of
// the volume free the module's set-up fails (usually with a bare
// "no space left on device"), or the file fills up and the snapshot stops
// being usable part way through the backup; refusing first says why.
const minCowFreePercent = 10

// freeSpace returns the total and available bytes of the file system mounted
// at path (statfs, as df shows them), and whether they could be read.
func freeSpace(path string) (total, free uint64, ok bool) {
	var st unix.Statfs_t
	if err := unix.Statfs(path, &st); err != nil {
		return 0, 0, false
	}
	bsize := uint64(st.Bsize)
	return uint64(st.Blocks) * bsize, uint64(st.Bavail) * bsize, true
}

// checkCowFreeSpace returns ErrNotEnoughSpace, saying how much room is
// missing, when mountpoint has less than minCowFreePercent of its size free
// for the snapshot's copy-on-write file. A file system whose figures cannot
// be read is left to the module: refusing a backup over a failed statfs
// would block one that might have worked.
func checkCowFreeSpace(mountpoint, device, fstype string) error {
	total, free, ok := freeSpace(mountpoint)
	if !ok {
		return nil
	}
	return cowFreeSpaceProblem(mountpoint, device, fstype, total, free)
}

// hasCowFile reports whether a copy-on-write file of ours is on the file
// system already. One left by a crashed run holds the space the figures
// count as used; createOne removes it (destroyStaleTracers) and checks the
// free space again, so a check made before that would be too early.
func hasCowFile(mountpoint string) bool {
	matches, err := filepath.Glob(filepath.Join(mountpoint, ".pbs_snapshot_*.cow"))
	return err == nil && len(matches) > 0
}

// cowFreeSpaceProblem is checkCowFreeSpace on figures already read (a test
// passes them in).
func cowFreeSpaceProblem(mountpoint, device, fstype string, total, free uint64) error {
	if total == 0 {
		return nil
	}
	need := total / 100 * minCowFreePercent
	if free >= need {
		return nil
	}
	fs := ""
	if fstype != "" {
		fs = ", " + fstype
	}
	return fmt.Errorf("%w: %s (%s%s) has only %s of %s free (%.1f%%): the snapshot keeps its copy-on-write file on that file system, and elastio-snap and dattobd need at least %d%% of it (%s) free, or taking the snapshot fails. Free %s there (delete or move files away) and run the backup again, or read it without a snapshot (Use snapshots off; -novss for backup-folder, -liveread for backup-disk) and accept that the backup may be inconsistent",
		ErrNotEnoughSpace, mountpoint, device, fs, humanBytes(free), humanBytes(total),
		100*float64(free)/float64(total), minCowFreePercent, humanBytes(need), humanBytes(need-free))
}

// humanBytes writes a size in decimal units (1 GB = 1,000,000,000 bytes), as
// PBS and the rest of the backup tools show them.
func humanBytes(b uint64) string {
	switch {
	case b >= 1e12:
		return fmt.Sprintf("%.1f TB", float64(b)/1e12)
	case b >= 1e9:
		return fmt.Sprintf("%.1f GB", float64(b)/1e9)
	case b >= 1e6:
		return fmt.Sprintf("%.1f MB", float64(b)/1e6)
	case b >= 1e3:
		return fmt.Sprintf("%.1f kB", float64(b)/1e3)
	}
	return fmt.Sprintf("%d B", b)
}
