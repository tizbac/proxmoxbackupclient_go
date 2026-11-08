//go:build !windows

package main

import "pbscommon"

// NTFSMetaCollector is the Linux xattr/POSIX ACL collector (a no-op on other
// non-Windows platforms); the name is shared with the Windows implementation.
type NTFSMetaCollector = pbscommon.XattrCollector

func NewNTFSMetaCollector(root, hostname string) *NTFSMetaCollector {
	return pbscommon.NewXattrCollector(root, hostname)
}
