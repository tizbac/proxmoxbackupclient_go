//go:build !windows

package pbscommon

// NewFileMetaCollector returns the xattr/POSIX ACL collector (a no-op off
// Linux).
func NewFileMetaCollector(root, hostname string) FileMetaCollector {
	return NewXattrCollector(root, hostname)
}
