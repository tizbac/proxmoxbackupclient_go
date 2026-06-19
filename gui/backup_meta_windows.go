//go:build windows
// +build windows

package main

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/sys/windows"
)

// NTFSMetaCollector implements pbscommon.MetaCollector for Windows. It captures
// owner/group/DACL (as an SDDL string) plus NTFS file attributes during the
// pxar walk and serializes everything into a single gzipped JSON file that
// lives as a virtual file inside the archive root.
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
		root:    absRoot,
		sddlIdx: make(map[string]int),
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

	// Get security descriptor (owner + group + DACL).
	// SACL is excluded — reading it requires SE_SECURITY_NAME privilege which
	// not every service account has, and SACLs describe audit policy which
	// generally should not be restored mechanically anyway.
	var sddl string
	sd, err := windows.GetNamedSecurityInfo(
		absPath,
		windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.GROUP_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION,
	)
	if err == nil && sd != nil {
		sddl = sd.String() // returns empty string on internal conversion failure
	}
	if err != nil || sddl == "" {
		c.incErr()
		// Continue with empty SDDL so we at least record the file attributes
	}

	// Get file attributes (hidden, system, readonly, archive, ...)
	var attrs uint32
	if pathW, pErr := windows.UTF16PtrFromString(absPath); pErr == nil {
		if a, aErr := windows.GetFileAttributes(pathW); aErr == nil {
			attrs = a
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()

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
	})
	return nil
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

	return SerializeFileMeta(c.meta)
}

// Stats returns entry count, unique SDDL count, and error count.
func (c *NTFSMetaCollector) Stats() (entries, uniqueSDDLs, errors int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.meta.Entries), len(c.meta.SDDLs), c.errors
}
