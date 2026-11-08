//go:build windows

package main

import "path/filepath"

// rollbackVolumeRoot returns the root of the volume holding dir ("F:\" or
// "\\server\share\"), where Roll back keeps its safety copies so that moving a
// file there is a rename, not a copy.
func rollbackVolumeRoot(dir string) string {
	v := filepath.VolumeName(filepath.Clean(dir))
	if v == "" {
		return filepath.Dir(filepath.Clean(dir))
	}
	return v + `\`
}
