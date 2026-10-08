package main

// Comparing a snapshot of a folder Backup Set with the live folders, for
// Undelete (files in a snapshot that are gone from disk) and Roll back (put
// the folders back as a snapshot has them). Only names, sizes and
// modification times are compared; no file is opened.

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"pbscommon"
)

// IndexEntry is one file or folder of a snapshot archive or of a live folder:
// the path relative to the archive root (or backed-up folder) with forward
// slashes, and the size and modification time in whole seconds.
type IndexEntry struct {
	Path  string `json:"path"`
	IsDir bool   `json:"isDir"`
	Size  uint64 `json:"size"`
	MTime int64  `json:"mtime"`
}

// errCompareCancelled is returned when the caller's cancel check fires.
var errCompareCancelled = errors.New("cancelled")

// backupMetaFileName is the file the client adds at the root of every folder
// archive (BackupMeta); it never exists on disk, so comparisons ignore it.
const backupMetaFileName = ".proxmox_backup_client_meta.json"

// pathKey is how two paths are matched: case-insensitively on Windows (NTFS
// treats "Report.docx" and "report.docx" as the same file), exactly elsewhere.
func pathKey(p string) string {
	if runtime.GOOS == "windows" {
		return strings.ToLower(p)
	}
	return p
}

// sortIndex sorts entries by pathKey so two indexes can be merged in one pass.
// Each key is made once: lower-casing inside the comparison took about a
// minute per snapshot on a slow CPU with 868,000 entries.
func sortIndex(entries []IndexEntry) {
	keys := make([]string, len(entries))
	for i := range entries {
		keys[i] = pathKey(entries[i].Path)
	}
	sort.Sort(keyedEntries{entries, keys})
}

type keyedEntries struct {
	e []IndexEntry
	k []string
}

func (s keyedEntries) Len() int           { return len(s.e) }
func (s keyedEntries) Less(i, j int) bool { return s.k[i] < s.k[j] }
func (s keyedEntries) Swap(i, j int) {
	s.e[i], s.e[j] = s.e[j], s.e[i]
	s.k[i], s.k[j] = s.k[j], s.k[i]
}

// LiveIndex is a walked live folder: every file and folder a backup of it
// would include, plus the folders that could not be read (anything a snapshot
// has under those is "unknown", never "missing").
type LiveIndex struct {
	Root       string
	Entries    []IndexEntry // sorted by pathKey
	Unreadable []string     // relative paths of folders that could not be listed

	files map[string]struct{} // pathKey of every file, built on first HasFile
}

// HasFile reports whether the live folder holds a FILE at rel.
func (li *LiveIndex) HasFile(rel string) bool {
	if li.files == nil {
		li.files = make(map[string]struct{}, len(li.Entries))
		for _, e := range li.Entries {
			if !e.IsDir {
				li.files[pathKey(e.Path)] = struct{}{}
			}
		}
	}
	_, ok := li.files[pathKey(rel)]
	return ok
}

// walkLiveFolder lists root the way a backup of it would see it: the same
// automatic exclusions (system folders and files, the Roll back safety
// folder), the Backup Set's own exclusion patterns, and junctions/symlinked
// folders below the top not followed. Only directory listings are read.
// progress, when set, is called with the number of entries found so far,
// at most a few times a second.
func walkLiveFolder(root string, excludes []string, cancel func() bool, progress func(n int)) (*LiveIndex, error) {
	root = filepath.Clean(root)
	if fi, err := os.Stat(root); err != nil {
		return nil, err
	} else if !fi.IsDir() {
		return nil, errors.New(root + " is not a folder")
	}
	li := &LiveIndex{Root: root}
	var lastReport time.Time
	var walk func(dir, rel string) error
	walk = func(dir, rel string) error {
		if cancel != nil && cancel() {
			return errCompareCancelled
		}
		if progress != nil && time.Since(lastReport) >= 250*time.Millisecond {
			lastReport = time.Now()
			progress(len(li.Entries))
		}
		items, err := os.ReadDir(dir)
		if err != nil {
			if rel != "" {
				li.Unreadable = append(li.Unreadable, rel)
				return nil
			}
			return err
		}
		for _, it := range items {
			name := it.Name()
			childRel := name
			if rel != "" {
				childRel = rel + "/" + name
			}
			childPath := filepath.Join(dir, name)
			if len(excludes) > 0 && pbscommon.IsExcludedByPatterns(childRel, name, root, excludes) {
				continue
			}
			info, ierr := it.Info()
			if ierr != nil {
				// Vanished between the listing and the stat: treat as absent.
				continue
			}
			if it.IsDir() || (info.Mode()&os.ModeSymlink != 0 && isDirLink(childPath)) {
				if pbscommon.IsAutoExcludedFolder(name) {
					continue
				}
				if info.Mode()&os.ModeSymlink != 0 {
					// A backup skips junctions and folder links below the top.
					continue
				}
				li.Entries = append(li.Entries, IndexEntry{Path: childRel, IsDir: true, MTime: info.ModTime().Unix()})
				if err := walk(childPath, childRel); err != nil {
					return err
				}
				continue
			}
			if pbscommon.IsAutoExcludedFile(name) {
				continue
			}
			li.Entries = append(li.Entries, IndexEntry{Path: childRel, Size: uint64(info.Size()), MTime: info.ModTime().Unix()})
		}
		return nil
	}
	if err := walk(root, ""); err != nil {
		return nil, err
	}
	if progress != nil {
		progress(len(li.Entries))
	}
	sortIndex(li.Entries)
	return li, nil
}

// isDirLink reports whether a symlink or junction points at a folder.
func isDirLink(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

// underAny reports whether rel is one of prefixes or inside one of them.
func underAny(rel string, prefixes []string) bool {
	k := pathKey(rel)
	for _, p := range prefixes {
		pk := pathKey(p)
		if k == pk || strings.HasPrefix(k, pk+"/") {
			return true
		}
	}
	return false
}

// CompareResult sorts one archive's files against its live folder.
type CompareResult struct {
	Unchanged int
	Missing   []IndexEntry // in the snapshot, not on disk
	Changed   []ChangedFile
	New       []IndexEntry // on disk, not in the snapshot (files only)
	NewDirs   []IndexEntry // folders on disk that the snapshot does not have
	Unknown   int          // snapshot files under folders that could not be read
}

// ChangedFile is a file present in both, with a different size or time.
type ChangedFile struct {
	Snap IndexEntry
	Live IndexEntry
}

// compareIndexes merges a snapshot archive's entries with a live folder's,
// both sorted by pathKey. Folders only matter for NewDirs (an exact roll back
// removes them when they end up empty); everything else is about files.
func compareIndexes(snap []IndexEntry, live *LiveIndex) CompareResult {
	var r CompareResult
	i, j := 0, 0
	for i < len(snap) || j < len(live.Entries) {
		var s, l *IndexEntry
		var sk, lk string
		if i < len(snap) {
			s = &snap[i]
			sk = pathKey(s.Path)
		}
		if j < len(live.Entries) {
			l = &live.Entries[j]
			lk = pathKey(l.Path)
		}
		switch {
		case l == nil || (s != nil && sk < lk):
			i++
			if s.IsDir || s.Path == backupMetaFileName {
				continue
			}
			if underAny(s.Path, live.Unreadable) {
				r.Unknown++
				continue
			}
			r.Missing = append(r.Missing, *s)
		case s == nil || lk < sk:
			j++
			if l.IsDir {
				r.NewDirs = append(r.NewDirs, *l)
			} else {
				r.New = append(r.New, *l)
			}
		default:
			i++
			j++
			switch {
			case s.IsDir && l.IsDir:
			case s.IsDir != l.IsDir:
				// A file became a folder or the other way round: the snapshot's
				// version is missing and the live one is new.
				if !s.IsDir {
					r.Missing = append(r.Missing, *s)
				}
				if l.IsDir {
					r.NewDirs = append(r.NewDirs, *l)
				} else {
					r.New = append(r.New, *l)
				}
			case s.Size != l.Size || s.MTime != l.MTime:
				r.Changed = append(r.Changed, ChangedFile{Snap: *s, Live: *l})
			default:
				r.Unchanged++
			}
		}
	}
	return r
}

// moveHints finds, for files missing from disk, a live file elsewhere with the
// same name, size and time: most likely the same file, moved or renamed folder.
// Returns missing path -> live path.
func moveHints(missing []IndexEntry, live []IndexEntry) map[string]string {
	if len(missing) == 0 {
		return nil
	}
	type sig struct {
		name  string
		size  uint64
		mtime int64
	}
	want := make(map[sig][]string, len(missing))
	for _, m := range missing {
		s := sig{pathKey(baseOf(m.Path)), m.Size, m.MTime}
		want[s] = append(want[s], m.Path)
	}
	hints := make(map[string]string)
	for _, l := range live {
		if l.IsDir {
			continue
		}
		s := sig{pathKey(baseOf(l.Path)), l.Size, l.MTime}
		if paths, ok := want[s]; ok {
			for _, p := range paths {
				if _, done := hints[p]; !done {
					hints[p] = l.Path
				}
			}
			delete(want, s)
		}
	}
	return hints
}

func baseOf(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}
