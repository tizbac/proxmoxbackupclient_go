//go:build windows

package pbscommon

import (
	"os"
	"syscall"
)

// isLinkEntry reports whether fi (from os.Lstat of path) is a link the walk
// must not follow or open: a symbolic link, or a junction (mount point).
//
// Go 1.23 and later no longer report a junction as os.ModeSymlink on
// Windows: it is os.ModeIrregular, which the ModeSymlink checks here missed.
// A junction was then opened as a file, and Windows' own deny-listed ones
// (Documents\My Music, My Pictures, My Videos, AppData's "Application
// Data") failed with "Access is denied" as read errors, making the run
// partial. Other irregular entries (OneDrive placeholders, deduplicated
// files) hold data and are still read: only a reparse tag that names
// another place (a name surrogate, as junctions and symbolic links have) is
// a link.
func isLinkEntry(path string, fi os.FileInfo) bool {
	if fi.Mode()&os.ModeSymlink != 0 {
		return true
	}
	if fi.Mode()&os.ModeIrregular == 0 {
		return false
	}
	d, ok := fi.Sys().(*syscall.Win32FileAttributeData)
	if !ok || d.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT == 0 {
		return false
	}
	return reparseNameSurrogate(path)
}

// reparseNameSurrogate reads path's reparse tag (FindFirstFile gives it in
// dwReserved0) and reports whether it is a name surrogate (bit 29), as
// IO_REPARSE_TAG_MOUNT_POINT and IO_REPARSE_TAG_SYMLINK are.
func reparseNameSurrogate(path string) bool {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return false
	}
	var fd syscall.Win32finddata
	h, err := syscall.FindFirstFile(p, &fd)
	if err != nil {
		return false
	}
	syscall.FindClose(h)
	return fd.Reserved0&0x20000000 != 0
}
