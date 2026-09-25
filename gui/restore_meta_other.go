//go:build !windows
// +build !windows

package main

// applyNTFSMetadata is a no-op on non-Windows platforms — there is nothing
// meaningful to apply an NTFS Security Descriptor or DOS attributes to.
func applyNTFSMetadata(destPath string, entry FileMetaEntry, sddls []string) error {
	return nil
}
