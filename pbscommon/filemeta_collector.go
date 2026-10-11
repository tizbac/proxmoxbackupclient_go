package pbscommon

// FileMetaCollector is the platform's metadata collector for a backup walk:
// NTFS ACLs, attributes and creation times on Windows, xattrs and POSIX ACLs
// on Linux, nothing elsewhere. Its result is the side-car blob a GUI restore
// re-applies.
type FileMetaCollector interface {
	MetaCollector
	FinalizeRaw() (*BackupFileMeta, error)
	Stats() (entries, uniqueSDDLs, errors int)
}
