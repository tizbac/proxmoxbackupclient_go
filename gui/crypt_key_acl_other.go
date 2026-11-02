//go:build !windows

package main

import (
	"fmt"
	"os"
)

// restrictKeyAccess makes path readable by its owner only (0600, or 0700 for
// the keys folder). SaveKeyFile already writes 0600; this matters for a key
// file the user picked.
func restrictKeyAccess(path string, isDir bool) error {
	mode := os.FileMode(0o600)
	if isDir {
		mode = 0o700
	}
	return os.Chmod(path, mode)
}

// keyFileOpenToOthers reports whether other users can read the key file at
// path: its group or everyone.
func keyFileOpenToOthers(path string) (bool, error) {
	st, err := os.Stat(path)
	if err != nil {
		return false, fmt.Errorf("reading the permissions of %s: %w", path, err)
	}
	return st.Mode().Perm()&0o044 != 0, nil
}
