//go:build !windows

package main

import (
	"fmt"
	"os"
)

// restrictKeyAccess makes path readable by its owner only (0600, or 0700 for
// the keys folder), keeping the administrators' group read access inside the
// service's shared settings folder (sharedFileMode). SaveKeyFile already
// writes 0600; this matters for a key file the user picked.
func restrictKeyAccess(path string, isDir bool) error {
	mode := os.FileMode(0o600)
	if isDir {
		mode = 0o700
	}
	return os.Chmod(path, sharedFileMode(path, mode))
}

// keyFileOpenToOthers reports whether other users can read the key file at
// path: everyone, or a group outside the shared settings folder (inside it
// the administrators' group reading is by design).
func keyFileOpenToOthers(path string) (bool, error) {
	st, err := os.Stat(path)
	if err != nil {
		return false, fmt.Errorf("reading the permissions of %s: %w", path, err)
	}
	perm := st.Mode().Perm()
	return perm&0o004 != 0 || (perm&0o040 != 0 && !inSharedConfigDir(path)), nil
}
