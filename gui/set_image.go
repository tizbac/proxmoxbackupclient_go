package main

// Undelete and Roll back for machine (disk image) Backup Sets. A machine
// set's "folders" are folders on the disks it backs up (ImageFolders, by
// default the user's Desktop, Documents, Pictures, Music, Videos and
// Downloads). Each one maps to its NTFS volume in the disk image by the
// disk's number and the partition's offset, and its file list in a snapshot
// is the image's entries below it. From there the folder-set code (compare,
// preview, safety folder, Undo) runs unchanged; the files themselves come
// out of the image (restore_image.go).

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"pbscommon"

	"github.com/tizbac/proxmoxbackupclient_go/gui/imagefs"
)

// defaultImageFolders: the user's own folders, the ones that exist.
func defaultImageFolders() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	var out []string
	for _, n := range []string{"Desktop", "Documents", "Pictures", "Music", "Videos", "Downloads"} {
		p := filepath.Join(home, n)
		if fi, err := os.Stat(p); err == nil && fi.IsDir() {
			out = append(out, p)
		}
	}
	return out
}

var physicalDriveRe = regexp.MustCompile(`(?i)PhysicalDrive(\d+)$`)

// imageSetFolders maps a machine set's folders to their volumes: Base is
// "drive-sata<disk>.img.fidx@<partition offset>:<path in the volume>", which
// is also the "archive" name Undelete and Roll back key their lists by.
// Folders on a disk the set does not back up are left out (logged).
func imageSetFolders(job ScheduledJob) []setFolder {
	folders := job.ImageFolders
	if len(folders) == 0 {
		folders = defaultImageFolders()
	}
	var out []setFolder
	for _, dir := range folders {
		dir = filepath.Clean(strings.TrimSpace(dir))
		if dir == "" || dir == "." {
			continue
		}
		base, err := imageFolderBase(job, dir)
		if err != nil {
			writeDebugLog(fmt.Sprintf("Machine set %q: %s left out: %v", job.Name, dir, err))
			continue
		}
		out = append(out, setFolder{Dir: dir, Base: base})
	}
	return out
}

// imageBase is a parsed imageSetFolders Base.
type imageBase struct {
	archive string // drive-sata0.img.fidx
	offset  int64  // partition offset in that image
	sub     string // folder inside the volume, '/'-separated, "" for the root
}

func parseImageBase(base string) (imageBase, bool) {
	at := strings.Index(base, ".img.fidx@")
	if at < 0 {
		return imageBase{}, false
	}
	rest := base[at+len(".img.fidx@"):]
	colon := strings.IndexByte(rest, ':')
	if colon < 0 {
		return imageBase{}, false
	}
	off, err := strconv.ParseInt(rest[:colon], 10, 64)
	if err != nil {
		return imageBase{}, false
	}
	return imageBase{archive: base[:at+len(".img.fidx")], offset: off, sub: rest[colon+1:]}, true
}

func isImageBase(base string) bool {
	_, ok := parseImageBase(base)
	return ok
}

// imageSetIndex is loadSnapshotIndex for a machine set: for each of the
// snapshot's folder bases, the image entries below that folder (paths made
// relative to it), leaving out what a walk of the live folder leaves out, so
// nothing excluded there shows up as missing.
func imageSetIndex(cfg *Config, job ScheduledJob, snap SetSnapshot, cancel func() bool) (snapshotIndex, error) {
	opts := restoreOptsFor(cfg, setBackupID(job), snap.Unix)
	fillSnapshotKind(&opts)
	s, err := openImageSession(opts, cancel)
	if err != nil {
		return nil, err
	}
	defer s.close()

	type volKey struct {
		archive string
		offset  int64
	}
	lists := map[volKey][]imagefs.Entry{}
	caseless := map[volKey]bool{}
	roots := map[string]string{}
	for _, f := range imageSetFolders(job) {
		roots[f.Base] = f.Dir
	}
	idx := snapshotIndex{}
	for _, base := range snap.Archives {
		ib, ok := parseImageBase(base)
		if !ok {
			continue
		}
		k := volKey{ib.archive, ib.offset}
		entries, done := lists[k]
		if !done {
			ra := s.readers[ib.archive]
			if ra == nil {
				return nil, fmt.Errorf("%s is not in the snapshot", ib.archive)
			}
			parts, err := imagefs.ReadPartitions(ra, s.sizes[ib.archive])
			if err != nil {
				return nil, fmt.Errorf("%s: %w", ib.archive, err)
			}
			var part *imagefs.Partition
			for i := range parts {
				if parts[i].Start == ib.offset {
					part = &parts[i]
				}
			}
			if part == nil {
				return nil, fmt.Errorf("%s has no partition at %d in the backup of %s", ib.archive, ib.offset, snap.ID)
			}
			vol, err := imagefs.OpenVolume(part.FS, io.NewSectionReader(ra, part.Start, part.Size), part.Size)
			if err != nil {
				return nil, fmt.Errorf("%s partition at %d in the backup of %s: %w", ib.archive, ib.offset, snap.ID, err)
			}
			caseless[k] = vol.CaseInsensitive()
			if entries, err = vol.List(cancel, nil); err != nil {
				if err == imagefs.ErrCancelled {
					return nil, errCompareCancelled
				}
				return nil, err
			}
			lists[k] = entries
		}
		idx[base] = folderEntries(entries, ib.sub, roots[base], job.ExcludeList, caseless[k])
	}
	return idx, nil
}

// folderEntries picks a volume's entries below sub, relative to it, with the
// live walk's exclusions (walkLiveFolder): the set's patterns (root is the
// folder's real path, which they may name), automatically excluded folders
// with everything below them, and automatically excluded files.
func folderEntries(all []imagefs.Entry, sub, root string, excludes []string, caseless bool) []IndexEntry {
	prefix := ""
	if sub != "" {
		prefix = sub + "/"
		if caseless {
			prefix = strings.ToLower(prefix)
		}
	}
	norm := func(s string) string {
		if caseless {
			return strings.ToLower(s)
		}
		return s
	}
	excludedDirs := map[string]bool{} // rel paths of excluded folders
	under := func(rel string) bool {
		for i := strings.LastIndexByte(rel, '/'); i > 0; i = strings.LastIndexByte(rel[:i], '/') {
			if excludedDirs[rel[:i]] {
				return true
			}
		}
		return false
	}
	var out []IndexEntry
	// Folders first, so every excluded folder is known before its files.
	for pass := 0; pass < 2; pass++ {
		for _, e := range all {
			if e.IsDir != (pass == 0) {
				continue
			}
			// NTFS and FAT names are case-insensitive: match the folder that way.
			if prefix != "" && !strings.HasPrefix(norm(e.Path), prefix) {
				continue
			}
			if e.Symlink {
				continue // a link is not a file to compare or restore
			}
			rel := e.Path[len(prefix):]
			if rel == "" {
				continue
			}
			name := rel[strings.LastIndexByte(rel, '/')+1:]
			excluded := (len(excludes) > 0 && pbscommon.IsExcludedByPatterns(rel, name, root, excludes)) ||
				(e.IsDir && pbscommon.IsAutoExcludedFolderAt(name, !strings.Contains(e.Path, "/"))) ||
				(!e.IsDir && pbscommon.IsAutoExcludedFile(name))
			if excluded || under(rel) {
				if e.IsDir {
					excludedDirs[rel] = true
				}
				continue
			}
			if e.IsDir {
				out = append(out, IndexEntry{Path: rel, IsDir: true, MTime: imagefs.UnixSeconds(e.MTime)})
			} else {
				out = append(out, IndexEntry{Path: rel, Size: e.Size, MTime: imagefs.UnixSeconds(e.MTime)})
			}
		}
	}
	sortIndex(out)
	return out
}
