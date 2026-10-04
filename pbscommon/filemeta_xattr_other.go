//go:build !linux

package pbscommon

import "os"

// XattrCollector is a no-op off Linux (Windows has its own collector in the
// GUI package).
type XattrCollector struct{}

func NewXattrCollector(root, hostname string) *XattrCollector { return &XattrCollector{} }

func (c *XattrCollector) Collect(absPath string, info os.FileInfo, isDir bool) error { return nil }

func (c *XattrCollector) Finalize() ([]byte, error) { return nil, nil }

func (c *XattrCollector) FinalizeRaw() (*BackupFileMeta, error) { return nil, nil }

func (c *XattrCollector) Stats() (entries, uniqueSDDLs, errors int) { return 0, 0, 0 }
