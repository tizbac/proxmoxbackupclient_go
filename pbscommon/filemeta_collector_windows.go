//go:build windows

package pbscommon

// NewFileMetaCollector returns the NTFS collector.
func NewFileMetaCollector(root, hostname string) FileMetaCollector {
	return NewNTFSMetaCollector(root, hostname)
}
