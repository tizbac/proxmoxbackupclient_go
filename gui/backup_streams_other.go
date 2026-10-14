//go:build !windows
// +build !windows

package main

// writeStreamsAndEAs: alternate data streams and NTFS extended attributes
// exist only on Windows, so a restore elsewhere writes none and counts the
// streams as skipped.
func writeStreamsAndEAs(path string, streams []StreamEntry, eas []EAEntry) (written, skipped int, err error) {
	return 0, len(streams), nil
}
