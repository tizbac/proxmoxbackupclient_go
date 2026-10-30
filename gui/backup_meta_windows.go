//go:build windows
// +build windows

package main

import (
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

const (
	// maxStreamBytes is the largest alternate data stream recorded with its
	// content. Nearly all are a few hundred bytes (Zone.Identifier).
	maxStreamBytes = 1 << 20
	// maxStreamBytesTotal bounds the stream content one archive records, so
	// the side file stays well inside the PBS blob size limit.
	maxStreamBytesTotal = 8 << 20
	// maxFileMetaBlobBytes is the size the serialized side file must stay
	// under (PBS refuses blobs over 16 MiB).
	maxFileMetaBlobBytes = 15 << 20
)

// serializeFileMetaCapped is SerializeFileMeta, but if the result would be
// larger than limit the recorded stream contents are dropped (each marked
// Skipped) and it is serialized again, so permissions and attributes are never
// lost to a large stream. dropped counts the streams whose content was dropped.
func serializeFileMetaCapped(meta *BackupFileMeta, limit int) (blob []byte, dropped int, err error) {
	blob, err = SerializeFileMeta(meta)
	if err != nil || len(blob) <= limit {
		return blob, 0, err
	}
	for i := range meta.Entries {
		for j := range meta.Entries[i].Streams {
			s := &meta.Entries[i].Streams[j]
			if s.Skipped == "" {
				s.Data, s.Skipped = nil, "metadata record too large"
				dropped++
			}
		}
	}
	if dropped == 0 {
		return blob, 0, nil
	}
	blob, err = SerializeFileMeta(meta)
	return blob, dropped, err
}

// NTFSMetaCollector implements pbscommon.MetaCollector for Windows. It captures
// owner/group/DACL (as an SDDL string) plus NTFS file attributes, the creation
// time, alternate data streams and extended attributes during the pxar walk
// and serializes everything into a single gzipped JSON file that lives as a
// virtual file inside the archive root.
//
// SDDLs are deduplicated via an index: in practice most files in a tree share
// a handful of unique descriptors (inherited from parent), so the dedup cuts
// output size by 5-20x.
type NTFSMetaCollector struct {
	root string

	mu      sync.Mutex
	sddlIdx map[string]int
	meta    *BackupFileMeta
	errors  int

	// streamBudget is how much alternate data stream content this archive
	// may still record (maxStreamBytesTotal); the counters feed the log.
	streamBudget   int64
	streams        int
	streamsSkipped int
	eas            int
	dropped        int
}

// NewNTFSMetaCollector creates a collector rooted at the given filesystem path.
// Paths passed to Collect must live under this root; the collector records them
// as relative paths (forward slashes) in the output.
func NewNTFSMetaCollector(root, hostname string) *NTFSMetaCollector {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		absRoot = root
	}
	return &NTFSMetaCollector{
		root:         absRoot,
		sddlIdx:      make(map[string]int),
		streamBudget: maxStreamBytesTotal,
		meta: &BackupFileMeta{
			Version:  FileMetaFormatVers,
			Root:     absRoot,
			Captured: time.Now().UTC().Format(time.RFC3339),
			Host:     hostname,
			SDDLs:    []string{},
			Entries:  []FileMetaEntry{},
		},
	}
}

// Collect captures ACL + attributes for a single entry. Best-effort: any error
// increments the internal error counter but never fails the walk.
func (c *NTFSMetaCollector) Collect(absPath string, info os.FileInfo, isDir bool) error {
	rel, err := filepath.Rel(c.root, absPath)
	if err != nil {
		c.incErr()
		return nil
	}
	rel = filepath.ToSlash(rel)

	// Creation time and the entry's own attributes: already in the directory
	// listing the walk read, so this costs no extra disk access.
	var created int64
	var listedAttrs uint32
	if info != nil {
		if d, ok := info.Sys().(*syscall.Win32FileAttributeData); ok && d != nil {
			created = int64(d.CreationTime.HighDateTime)<<32 | int64(d.CreationTime.LowDateTime)
			listedAttrs = d.FileAttributes
		}
	}

	// The stream budget is shared by the whole archive, so it is read and
	// updated under the lock; the streams themselves are read outside it.
	c.mu.Lock()
	budget := c.streamBudget
	c.mu.Unlock()
	start := budget

	// One handle, opened without data access, for the security descriptor
	// (owner + group + DACL), attributes (hidden, system, readonly, archive,
	// ...), alternate data streams and extended attributes. A second handle
	// for the streams and EAs nearly doubled the cost per file. The SACL is
	// excluded: reading it requires SE_SECURITY_NAME privilege which not every
	// service account has, and SACLs describe audit policy which generally
	// should not be restored mechanically anyway.
	var sddl string
	var attrs uint32
	var streams []StreamEntry
	var eas []EAEntry
	opened := false
	if listedAttrs&windows.FILE_ATTRIBUTE_REPARSE_POINT == 0 {
		if h, herr := openMetaHandle(absPath); herr == nil {
			opened = true
			sd, err := windows.GetSecurityInfo(h, windows.SE_FILE_OBJECT,
				windows.OWNER_SECURITY_INFORMATION|windows.GROUP_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
			if err == nil && sd != nil {
				sddl = sd.String() // returns empty string on internal conversion failure
			}
			var bi windows.ByHandleFileInformation
			if windows.GetFileInformationByHandle(h, &bi) == nil {
				attrs = bi.FileAttributes
			}
			streams, eas = streamsAndEAsOf(h, absPath, &budget)
			windows.CloseHandle(h)
		}
	}
	if !opened {
		// A reparse point (link, junction): the lookups by name follow it, as
		// they always did, and its streams are not recorded. Or a file that
		// would not open this way.
		sddl, attrs = securityByPath(absPath)
		if listedAttrs&windows.FILE_ATTRIBUTE_REPARSE_POINT == 0 {
			streams, eas, _ = readStreamsAndEAs(absPath, &budget)
		}
	}
	if sddl == "" {
		c.incErr()
		// Continue with empty SDDL so we at least record the file attributes
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.streamBudget -= start - budget
	for _, s := range streams {
		if s.Skipped == "" {
			c.streams++
		} else {
			c.streamsSkipped++
		}
	}
	c.eas += len(eas)

	idx, ok := c.sddlIdx[sddl]
	if !ok {
		idx = len(c.meta.SDDLs)
		c.meta.SDDLs = append(c.meta.SDDLs, sddl)
		c.sddlIdx[sddl] = idx
	}

	c.meta.Entries = append(c.meta.Entries, FileMetaEntry{
		Path:    rel,
		IsDir:   isDir,
		SDDLIdx: idx,
		Attrs:   attrs,
		Created: created,
		Streams: streams,
		EAs:     eas,
	})
	return nil
}

// securityByPath reads a path's SDDL and attributes by name; the security
// lookup follows a link to its target.
func securityByPath(absPath string) (sddl string, attrs uint32) {
	sd, err := windows.GetNamedSecurityInfo(absPath, windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.GROUP_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err == nil && sd != nil {
		sddl = sd.String()
	}
	if pathW, pErr := windows.UTF16PtrFromString(absPath); pErr == nil {
		if a, aErr := windows.GetFileAttributes(pathW); aErr == nil {
			attrs = a
		}
	}
	return sddl, attrs
}

// StreamStats returns how many alternate data streams were recorded with
// their content, how many only by name (too large, over the budget, or
// unreadable), how many extended attributes were recorded, and how many
// stream contents Finalize dropped to keep the side file under the PBS blob
// limit.
func (c *NTFSMetaCollector) StreamStats() (streams, skipped, eas, dropped int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.streams, c.streamsSkipped, c.eas, c.dropped
}

func (c *NTFSMetaCollector) incErr() {
	c.mu.Lock()
	c.errors++
	c.mu.Unlock()
}

// Finalize serializes the collected metadata to gzipped JSON. Returns nil if
// nothing was collected (e.g. empty directory).
func (c *NTFSMetaCollector) Finalize() ([]byte, error) {
	c.mu.Lock()
	// Update finalized counters before releasing the lock on meta
	c.meta.Collected = len(c.meta.Entries)
	c.meta.Errors = c.errors
	c.mu.Unlock()

	blob, dropped, err := serializeFileMetaCapped(c.meta, maxFileMetaBlobBytes)
	c.mu.Lock()
	c.dropped = dropped
	c.mu.Unlock()
	return blob, err
}

// Stats returns entry count, unique SDDL count, and error count.
func (c *NTFSMetaCollector) Stats() (entries, uniqueSDDLs, errors int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.meta.Entries), len(c.meta.SDDLs), c.errors
}
