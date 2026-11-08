//go:build !windows

package pbscommon

// WriteStreamsAndEAs: alternate data streams and NTFS extended attributes
// exist only on Windows, so a restore elsewhere writes none and counts the
// streams as skipped.
func ReadStreamsAndEAs(path string) ([]StreamEntry, []EAEntry, error) { return nil, nil, nil }

func WriteStreamsAndEAs(path string, streams []StreamEntry, eas []EAEntry) (written, skipped int, err error) {
	return 0, len(streams), nil
}
