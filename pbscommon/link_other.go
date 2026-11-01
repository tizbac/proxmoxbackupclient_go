//go:build !windows

package pbscommon

import "os"

// isLinkEntry reports whether fi (from os.Lstat) is a symbolic link. On
// Windows junctions count too (link_windows.go).
func isLinkEntry(_ string, fi os.FileInfo) bool {
	return fi.Mode()&os.ModeSymlink != 0
}
