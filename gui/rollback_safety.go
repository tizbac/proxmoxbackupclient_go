package main

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"pbscommon"
)

// Where Roll back keeps the files it replaces or removes, and how a record
// of a roll back is checked before Undo or Delete kept versions acts on it.
//
// The safety folder must be on the same volume as the set's folder, so
// moving a file into it is a rename and Undo is instant. A folder inside a
// user's own profile (C:\Users\<name>\..., /home/<name>/...) keeps it in that
// profile: private to that user, and a user rolling back their own files
// needs no administrator rights. Anything else keeps it at the root of the
// volume, which is an administrator's job. Either way the folder is locked
// down to SYSTEM, the administrators and the account that made it
// (secureSafetyFolder): it holds copies of the set's files.

// rollbackSafetyRoot is the safety folder for a set folder. A variable so
// tests can put it in a temporary folder.
var rollbackSafetyRoot = func(dir string) string {
	if home, ok := userHomeOf(dir); ok {
		return filepath.Join(home, pbscommon.RollbackSafetyFolder)
	}
	return filepath.Join(rollbackVolumeRoot(dir), pbscommon.RollbackSafetyFolder)
}

// rollbackSafetyRoots are the places a set folder's roll backs can be: the
// current one, and the volume root where versions before the profile folder
// kept them, so their roll backs are still listed and can be undone.
func rollbackSafetyRoots(dir string) []string {
	cur := rollbackSafetyRoot(dir)
	legacy := filepath.Join(rollbackVolumeRoot(dir), pbscommon.RollbackSafetyFolder)
	if samePath(cur, legacy) {
		return []string{cur}
	}
	return []string{cur, legacy}
}

// samePath compares two cleaned paths, ignoring case on Windows.
func samePath(a, b string) bool {
	a, b = filepath.Clean(a), filepath.Clean(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

// pathWithin reports whether p is root or inside it, after cleaning (so a
// ".." cannot climb out), ignoring case on Windows.
func pathWithin(p, root string) bool {
	p, root = filepath.Clean(p), filepath.Clean(root)
	if samePath(p, root) {
		return true
	}
	prefix := root
	if !strings.HasSuffix(prefix, string(filepath.Separator)) {
		prefix += string(filepath.Separator)
	}
	if runtime.GOOS == "windows" {
		return strings.HasPrefix(strings.ToLower(p), strings.ToLower(prefix))
	}
	return strings.HasPrefix(p, prefix)
}

// checkRollbackRecord makes sure a roll back record only names places that
// roll back of this set can have touched, before Undo deletes and moves
// files by it or Delete kept versions removes its folders: everything it
// restored, moved away, created or removed is inside one of the set's
// folders; everything it kept is inside its own safety folder; and its
// safety folders are where this set's roll backs are kept. Anyone who could
// change the record would otherwise choose what an administrator's Undo
// deletes.
func checkRollbackRecord(job ScheduledJob, manifestPath string, m *rollbackManifest) error {
	if m.JobID != job.ID {
		return fmt.Errorf("the roll back record %s belongs to another Backup Set", manifestPath)
	}
	folders := setFoldersOf(job)
	var bases []string
	for _, f := range folders {
		for _, r := range rollbackSafetyRoots(f.Dir) {
			bases = append(bases, filepath.Join(r, safetyName(job.Name)))
		}
	}
	safetyOK := func(root string) bool {
		for _, b := range bases {
			if samePath(filepath.Dir(root), b) {
				return true
			}
		}
		return false
	}
	if len(m.SafetyRoots) == 0 {
		return fmt.Errorf("the roll back record %s names no safety folder", manifestPath)
	}
	ownRecord := false
	for _, r := range m.SafetyRoots {
		if !safetyOK(r) {
			return fmt.Errorf("the roll back record %s names %s, which is not where this set's roll backs are kept", manifestPath, r)
		}
		if samePath(filepath.Dir(manifestPath), r) {
			ownRecord = true
		}
	}
	if !ownRecord {
		return fmt.Errorf("the roll back record %s is not in one of its own safety folders", manifestPath)
	}
	inSet := func(p string) bool {
		for _, f := range folders {
			if pathWithin(p, f.Dir) {
				return true
			}
		}
		return false
	}
	inSafety := func(p string) bool {
		for _, r := range m.SafetyRoots {
			if pathWithin(p, r) {
				return true
			}
		}
		return false
	}
	for _, p := range m.Restored {
		if !inSet(p) {
			return fmt.Errorf("the roll back record %s names %s, outside the set's folders", manifestPath, p)
		}
	}
	for _, mv := range m.Moved {
		if !inSet(mv.From) {
			return fmt.Errorf("the roll back record %s names %s, outside the set's folders", manifestPath, mv.From)
		}
		if !inSafety(mv.To) {
			return fmt.Errorf("the roll back record %s names %s, outside its safety folder", manifestPath, mv.To)
		}
	}
	for _, list := range [][]string{m.CreatedDirs, m.RemovedDirs} {
		for _, d := range list {
			if !inSet(d) {
				return fmt.Errorf("the roll back record %s names %s, outside the set's folders", manifestPath, d)
			}
		}
	}
	return nil
}
