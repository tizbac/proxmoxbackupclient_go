//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"syscall"
)

// rollbackVolumeRoot returns the top folder of the filesystem holding dir (its mount
// point), where Roll back keeps its safety copies so that moving a file there
// is a rename, not a copy.
func rollbackVolumeRoot(dir string) string {
	cur := filepath.Clean(dir)
	dev, ok := deviceOf(cur)
	if !ok {
		return filepath.Dir(cur)
	}
	for {
		parent := filepath.Dir(cur)
		if parent == cur {
			return cur
		}
		pdev, ok := deviceOf(parent)
		if !ok || pdev != dev {
			return cur
		}
		cur = parent
	}
}

func deviceOf(p string) (uint64, bool) {
	fi, err := os.Stat(p)
	if err != nil {
		return 0, false
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, false
	}
	return uint64(st.Dev), true
}
