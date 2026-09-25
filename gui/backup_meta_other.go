//go:build !windows && !linux
// +build !windows,!linux

package main

import "os"

// NTFSMetaCollector is a no-op on platforms with no real implementation
// (Windows and Linux both have one — see backup_meta_windows.go and
// backup_meta_linux.go). It still satisfies the pbscommon.MetaCollector
// interface so callers can unconditionally wire it up without build-tag
// branching.
type NTFSMetaCollector struct{}

func NewNTFSMetaCollector(root, hostname string) *NTFSMetaCollector { return &NTFSMetaCollector{} }

func (c *NTFSMetaCollector) Collect(absPath string, info os.FileInfo, isDir bool) error {
	return nil
}

func (c *NTFSMetaCollector) Finalize() ([]byte, error) { return nil, nil }

func (c *NTFSMetaCollector) FinalizeRaw() (*BackupFileMeta, error) { return nil, nil }

func (c *NTFSMetaCollector) Stats() (entries, uniqueSDDLs, errors int) { return 0, 0, 0 }
