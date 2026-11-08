//go:build windows

package main

import "syscall"

// File attributes a restore puts back from a disk image: read-only, hidden,
// system and archive. The others (compressed, encrypted, sparse, reparse)
// describe how NTFS stored the file and are not set this way.
const restorableAttrs = 0x1 | 0x2 | 0x4 | 0x20

func applyFileAttributes(path string, attrs uint32) {
	a := attrs & restorableAttrs
	if a == 0 {
		a = 0x80 // FILE_ATTRIBUTE_NORMAL
	}
	if p, err := syscall.UTF16PtrFromString(path); err == nil {
		_ = syscall.SetFileAttributes(p, a)
	}
}

// clearReadOnly lets an existing read-only file be replaced.
func clearReadOnly(path string) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return
	}
	if a, err := syscall.GetFileAttributes(p); err == nil && a&1 != 0 {
		_ = syscall.SetFileAttributes(p, a&^1)
	}
}
