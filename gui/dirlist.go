package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DirEntry is one row in the Explorer-style folder tree the frontend renders
// for directory-backup folder selection. Path is always the full path the
// frontend should pass back to a further ListDirectory call (to expand) or
// use directly as a backup root (when selected).
type DirEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"is_dir"`
}

// ListDirectory lists the immediate children of path for the tree-picker's
// lazy per-expand loading (never a full upfront drive scan — a C:\ with
// millions of files must stay instant to expand one level at a time).
//
// path == "" returns the tree ROOTS instead of a directory's children: the
// logical drives on Windows (C:\, D:\, ...), or "/" on other platforms. This
// lets the frontend bootstrap the tree with one call before anything has been
// expanded.
//
// Unreadable entries (permission-denied subfolders, broken junctions, etc.)
// are skipped rather than failing the whole listing — one bad child must not
// block browsing its siblings, mirroring the "skip and log" philosophy the
// backup path itself already uses for unreadable files.
func (a *App) ListDirectory(path string) ([]DirEntry, error) {
	if path == "" {
		roots, err := listRoots()
		if err != nil {
			writeDebugLog(fmt.Sprintf("ListDirectory(roots): %v", err))
			return nil, err
		}
		return roots, nil
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		writeDebugLog(fmt.Sprintf("ListDirectory(%q): %v", path, err))
		return nil, err
	}

	result := make([]DirEntry, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		info, err := e.Info()
		isDir := e.IsDir()
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			// A symlink/junction: resolve whether the TARGET is a directory so
			// it renders (and is selectable) correctly rather than as a file.
			if target, terr := os.Stat(filepath.Join(path, name)); terr == nil {
				isDir = target.IsDir()
			} else {
				// Broken link target — skip it, same as any other unreadable entry.
				continue
			}
		}
		result = append(result, DirEntry{
			Name:  name,
			Path:  filepath.Join(path, name),
			IsDir: isDir,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].IsDir != result[j].IsDir {
			return result[i].IsDir // directories first
		}
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})

	return result, nil
}
