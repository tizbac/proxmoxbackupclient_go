//go:build linux

package pbscommon

import (
	"os"
	"syscall"
	"unsafe"
)

// applyModeOwner sets ownership (when owner is true and the process is root)
// and then the permission bits, in that order because chown clears setuid and
// setgid. With owner false only the plain 0777 bits are applied, which is what
// restores have always done.
func applyModeOwner(path string, mode, uid, gid uint32, owner bool) {
	if !owner {
		_ = os.Chmod(path, os.FileMode(mode&0o777))
		return
	}
	if os.Geteuid() == 0 {
		_ = os.Lchown(path, int(uid), int(gid))
	}
	_ = os.Chmod(path, posixFileMode(mode))
}

// posixFileMode converts st_mode permission, setuid, setgid and sticky bits
// into an os.FileMode.
func posixFileMode(mode uint32) os.FileMode {
	m := os.FileMode(mode & 0o777)
	if mode&uint32(ISUID) != 0 {
		m |= os.ModeSetuid
	}
	if mode&uint32(ISGID) != 0 {
		m |= os.ModeSetgid
	}
	if mode&uint32(ISVTX) != 0 {
		m |= os.ModeSticky
	}
	return m
}

// chownLink sets a symlink's own owner without following it.
func chownLink(path string, uid, gid uint32) {
	if os.Geteuid() == 0 {
		_ = os.Lchown(path, int(uid), int(gid))
	}
}

// setLinkTime sets a symlink's own mtime without following it.
func setLinkTime(path string, secs int64, nanos uint32) {
	ts := [2]syscall.Timespec{
		{Sec: secs, Nsec: int64(nanos)},
		{Sec: secs, Nsec: int64(nanos)},
	}
	p, err := syscall.BytePtrFromString(path)
	if err != nil {
		return
	}
	dirfd := -100 // AT_FDCWD
	const atSymlinkNofollow = 0x100
	_, _, _ = syscall.Syscall6(syscall.SYS_UTIMENSAT, uintptr(dirfd), uintptr(unsafe.Pointer(p)),
		uintptr(unsafe.Pointer(&ts[0])), atSymlinkNofollow, 0, 0)
}

// ApplyDirMetadata applies a restored folder's recorded owner and full mode
// bits. Run after the folder's contents are written, because a restrictive
// mode would otherwise block the restore itself.
func ApplyDirMetadata(path string, mode, uid, gid uint32) {
	applyModeOwner(path, mode, uid, gid, true)
}
