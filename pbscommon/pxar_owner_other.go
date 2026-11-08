//go:build !linux

package pbscommon

import "os"

func applyModeOwner(path string, mode, uid, gid uint32, owner bool) {
	_ = os.Chmod(path, os.FileMode(mode&0o777))
}

func chownLink(path string, uid, gid uint32) {}

func setLinkTime(path string, secs int64, nanos uint32) {}

func ApplyDirMetadata(path string, mode, uid, gid uint32) {}
