//go:build !windows

package main

import "os"

// hardenConfigDir makes the settings folder private to its owner. Files in it
// are already written 0600; the folder itself was created 0755, which let
// other local accounts list it.
func hardenConfigDir(dir string) error {
	return os.Chmod(dir, 0o700)
}
