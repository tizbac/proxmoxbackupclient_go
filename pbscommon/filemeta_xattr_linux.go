//go:build linux

package pbscommon

import (
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// XattrCollector implements MetaCollector on Linux. It captures every
// extended attribute of each entry, which includes POSIX ACLs (stored by the
// kernel as the xattrs system.posix_acl_access / system.posix_acl_default).
type XattrCollector struct {
	root string

	mu     sync.Mutex
	meta   *BackupFileMeta
	errors int
}

func NewXattrCollector(root, hostname string) *XattrCollector {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		absRoot = root
	}
	return &XattrCollector{
		root: absRoot,
		meta: &BackupFileMeta{
			Version:  FileMetaVersion,
			Root:     absRoot,
			Captured: time.Now().UTC().Format(time.RFC3339),
			Host:     hostname,
			SDDLs:    []string{},
			Entries:  []FileMetaEntry{},
		},
	}
}

// Listxattr/Getxattr retry with a growing buffer on ERANGE.
func listxattrBuf(path string) ([]byte, error) {
	for size := 256; ; size *= 4 {
		buf := make([]byte, size)
		n, err := syscall.Listxattr(path, buf)
		if err == syscall.ERANGE {
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
		n, err := syscall.Getxattr(path, name, buf)
		if err == syscall.ERANGE {
			continue
		}
		if err != nil {
			return nil, err
		}
		return append([]byte(nil), buf[:n]...), nil
	}
}

func splitXattrNames(raw []byte) []string {
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

// Collect is best-effort: errors are counted, never returned.
func (c *XattrCollector) Collect(absPath string, info os.FileInfo, isDir bool) error {
	rel, err := filepath.Rel(c.root, absPath)
	if err != nil {
		c.incErr()
		return nil
	}
	rel = filepath.ToSlash(rel)

	rawNames, err := listxattrBuf(absPath)
	if err != nil {
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

func (c *XattrCollector) incErr() {
	c.mu.Lock()
	c.errors++
	c.mu.Unlock()
}

// Finalize serializes the collected metadata to gzipped JSON.
func (c *XattrCollector) Finalize() ([]byte, error) {
	c.mu.Lock()
	c.meta.Collected = len(c.meta.Entries)
	c.meta.Errors = c.errors
	c.mu.Unlock()
	return SerializeFileMeta(c.meta)
}

// FinalizeRaw returns the raw metadata, or nil if nothing was collected.
func (c *XattrCollector) FinalizeRaw() (*BackupFileMeta, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.meta.Collected = len(c.meta.Entries)
	c.meta.Errors = c.errors
	if len(c.meta.Entries) == 0 {
		return nil, nil
	}
	return c.meta, nil
}

// Stats returns entry count, unique SDDL count (always 0 on Linux) and errors.
func (c *XattrCollector) Stats() (entries, uniqueSDDLs, errors int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.meta.Entries), 0, c.errors
}
