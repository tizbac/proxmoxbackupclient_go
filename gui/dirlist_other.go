//go:build !windows
// +build !windows

package main

// listRoots returns the tree-picker's root nodes on non-Windows platforms:
// just "/" — this fork's GUI targets Windows, but the CLI/backend also
// builds for linux (see disklist_linux.go's equivalent split), so this stub
// keeps that build working rather than special-casing Windows everywhere
// ListDirectory is reachable.
func listRoots() ([]DirEntry, error) {
	return []DirEntry{{Name: "/", Path: "/", IsDir: true}}, nil
}
