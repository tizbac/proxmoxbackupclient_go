//go:build windows
// +build windows

package main

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// listRoots returns the tree-picker's root nodes on Windows: every logical
// drive letter that's actually present (GetLogicalDrives, not a fixed A-Z
// scan — avoids the multi-second timeout Windows can impose probing an empty
// floppy/optical drive letter).
func listRoots() ([]DirEntry, error) {
	mask, err := windows.GetLogicalDrives()
	if err != nil {
		return nil, fmt.Errorf("GetLogicalDrives failed: %w", err)
	}

	roots := make([]DirEntry, 0, 8)
	for i := 0; i < 26; i++ {
		if mask&(1<<uint(i)) == 0 {
			continue
		}
		letter := string(rune('A' + i))
		path := letter + `:\`
		roots = append(roots, DirEntry{
			Name:  letter + ":",
			Path:  path,
			IsDir: true,
		})
	}
	return roots, nil
}
