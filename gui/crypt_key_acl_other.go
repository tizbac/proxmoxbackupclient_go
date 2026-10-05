//go:build !windows

package main

// restrictKeyAccess is a no-op off Windows: SaveKeyFile already writes 0600 and
// the keys directory is created 0700.
func restrictKeyAccess(path string, isDir bool) error { return nil }
