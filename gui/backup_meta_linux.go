//go:build linux
// +build linux

package main

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/sys/unix"
)

// NTFSMetaCollector implements pbscommon.MetaCollector for Linux (the name is
// shared with the Windows implementation — see backup_meta_windows.go's doc
// comment on why — even though there's nothing NTFS-specific about it here).
// It captures every extended attribute on each file/dir during the pxar
// walk, including POSIX ACLs: on Linux, `setfacl`-applied access and default
// ACLs are themselves stored by the kernel as ordinary extended attributes
// under the reserved names "system.posix_acl_access"/"system.posix_acl_default"
// (a stable kernel ABI — see include/uapi/linux/posix_acl_xattr.h), so a
// generic xattr walk captures them for free without needing to parse or
// understand that binary format at all.
type NTFSMetaCollector struct {
	root string

	mu     sync.Mutex
	meta   *BackupFileMeta
	errors int
}

func NewNTFSMetaCollector(root, hostname string) *NTFSMetaCollector {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		absRoot = root
	}
	return &NTFSMetaCollector{
		root: absRoot,
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

// listxattrBuf and getxattrBuf retry with a growing buffer on ERANGE (the
// attribute list/value grew between the sizing call and the read call — rare,
// but possible under concurrent modification) instead of assuming a single
// fixed-size buffer is always enough.
func listxattrBuf(path string) ([]byte, error) {
	for size := 256; ; size *= 4 {
		buf := make([]byte, size)
		n, err := unix.Listxattr(path, buf)
		if err == unix.ERANGE {
			continue
		}
		if err != nil {
			return nil, err
		}
		return buf[:n], nil
	}
}

func getxattrBuf(path, name string) ([]byte, error) {
	for size := 256; ; size *= 4 {
		buf := make([]byte, size)
		n, err := unix.Getxattr(path, name, buf)
		if err == unix.ERANGE {
			continue
		}
		if err != nil {
			return nil, err
		}
		return append([]byte(nil), buf[:n]...), nil
	}
}

// splitXattrNames splits Listxattr's NUL-separated name list into individual
// strings, dropping the trailing empty element after the last NUL.
func splitXattrNames(raw []byte) []string {
	if len(raw) == 0 {
		return nil
	}
	var names []string
	start := 0
	for i, b := range raw {
		if b == 0 {
			if i > start {
				names = append(names, string(raw[start:i]))
			}
			start = i + 1
		}
	}
	return names
}

// Collect captures every extended attribute for a single entry. Best-effort:
// any error increments the internal error counter but never fails the walk —
// matching NTFSMetaCollector's Windows counterpart exactly.
func (c *NTFSMetaCollector) Collect(absPath string, info os.FileInfo, isDir bool) error {
	rel, err := filepath.Rel(c.root, absPath)
	if err != nil {
		c.incErr()
		return nil
	}
	rel = filepath.ToSlash(rel)

	rawNames, err := listxattrBuf(absPath)
	if err != nil {
		// ENOTSUP/ENODATA-class errors are routine on filesystems or files
		// with no xattr support at all — not worth counting as a real error.
		c.mu.Lock()
		c.meta.Entries = append(c.meta.Entries, FileMetaEntry{Path: rel, IsDir: isDir})
		c.mu.Unlock()
		return nil
	}

	names := splitXattrNames(rawNames)
	var xattrs map[string][]byte
	for _, name := range names {
		val, gerr := getxattrBuf(absPath, name)
		if gerr != nil {
			c.incErr()
			continue
		}
		if xattrs == nil {
			xattrs = make(map[string][]byte, len(names))
		}
		xattrs[name] = val
	}

	c.mu.Lock()
	c.meta.Entries = append(c.meta.Entries, FileMetaEntry{Path: rel, IsDir: isDir, Xattrs: xattrs})
	c.mu.Unlock()
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
	c.meta.Collected = len(c.meta.Entries)
	c.meta.Errors = c.errors
	c.mu.Unlock()
	return SerializeFileMeta(c.meta)
}

// FinalizeRaw is the multi-archive-path counterpart to Finalize — see
// NTFSMetaCollector's Windows implementation for the full doc comment.
func (c *NTFSMetaCollector) FinalizeRaw() (*BackupFileMeta, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.meta.Collected = len(c.meta.Entries)
	c.meta.Errors = c.errors
	if len(c.meta.Entries) == 0 {
		return nil, nil
	}
	return c.meta, nil
}

// Stats returns entry count, unique SDDL count (always 0 on Linux — no
// dedup dictionary here), and error count.
func (c *NTFSMetaCollector) Stats() (entries, uniqueSDDLs, errors int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.meta.Entries), 0, c.errors
}
