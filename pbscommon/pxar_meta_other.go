//go:build !linux

package pbscommon

import "os"

const posixFidelity = false

// entryMeta keeps the historic archive shape: fixed 0777 permissions, uid/gid
// 1000 (Windows has no POSIX modes or owners) and whole-second mtime.
func entryMeta(info os.FileInfo, kind uint64) (mode uint64, uid, gid uint32, secs uint64, nanos uint32) {
	return kind | 0o777, 1000, 1000, uint64(info.ModTime().Unix()), 0
}
