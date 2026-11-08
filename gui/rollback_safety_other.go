//go:build !windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// homesDirectory is where the users' home folders are. A variable so tests
// can point it elsewhere.
var homesDirectory = "/home"

// userHomeOf returns the home folder dir is in (/home/<name>, or /root), if
// it is in a user's own home on the same filesystem (a safety folder there
// must be reachable by a rename).
func userHomeOf(dir string) (string, bool) {
	d := filepath.Clean(dir)
	var home string
	switch {
	case pathWithin(d, "/root") && d != "/root":
		home = "/root"
	case pathWithin(d, homesDirectory) && !samePath(d, homesDirectory):
		rel, err := filepath.Rel(homesDirectory, d)
		if err != nil {
			return "", false
		}
		home = filepath.Join(homesDirectory, strings.Split(rel, "/")[0])
	default:
		return "", false
	}
	hd, ok1 := deviceOf(home)
	dd, ok2 := deviceOf(d)
	if !ok1 || !ok2 || hd != dd {
		return "", false
	}
	return home, true
}

// secureSafetyFolder makes the safety folder its owner's alone (0700), also
// when an earlier version made it with looser permissions: it holds copies
// of the set's files.
func secureSafetyFolder(path string) error {
	if err := os.Chmod(path, 0o700); err != nil {
		return fmt.Errorf("securing %s: %w", path, err)
	}
	return nil
}
