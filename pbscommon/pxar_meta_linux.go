//go:build linux

package pbscommon

import (
	"os"
	"syscall"
)

// posixFidelity: on Linux the archive records the real permission bits, owner
// and nanosecond mtime, stores symbolic links, and skips FIFOs, sockets and
// device nodes. Other platforms keep the historic fixed 0777 / uid 1000 shape.
const posixFidelity = true

// entryMeta returns the pxar ENTRY fields for a file of the given kind
// (IFREG, IFDIR or IFLNK) from its Lstat result.
func entryMeta(info os.FileInfo, kind uint64) (mode uint64, uid, gid uint32, secs uint64, nanos uint32) {
	t := info.ModTime()
	secs, nanos = uint64(t.Unix()), uint32(t.Nanosecond())
	if st, ok := info.Sys().(*syscall.Stat_t); ok {
		return kind | uint64(st.Mode&0o7777), st.Uid, st.Gid, secs, nanos
	}
	return kind | 0o777, 1000, 1000, secs, nanos
}
