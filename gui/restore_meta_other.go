//go:build !windows && !linux
// +build !windows,!linux

package main

// applyNTFSMetadata is a no-op on platforms with no real implementation
// (Windows and Linux both have one — see restore_meta_windows.go and
// restore_meta_linux.go).
func applyNTFSMetadata(destPath string, entry FileMetaEntry, sddls []string) error {
	return nil
}
