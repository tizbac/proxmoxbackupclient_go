package directorybackup

// Progress display of a folder backup (-progress): the bar is drawn from
// clientcommon (the FastCopy one), with the folder walked once beforehand to
// count its files and bytes. Standard input has no size to count, so its bar
// is indeterminate.

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"clientcommon"
	"pbscommon"
)

// progressBar is the -progress bar of this run, or nil. Backup jobs run one at
// a time in a process, and a run with several folders draws them one after the
// other, so a run-wide value is enough.
var progressBar *clientcommon.Progress

// notice prints an informational line. With the bar on it goes above the box,
// where it does not push the bar down the screen.
func notice(format string, args ...any) {
	msg := strings.TrimRight(fmt.Sprintf(format, args...), "\r\n")
	if progressBar != nil && progressBar.Active() {
		progressBar.Line("%s", msg)
		return
	}
	fmt.Println(msg)
}

// chunkNotice is the per-chunk trace (New chunk[...], Reuse chunk[...]). It is
// left out while the bar is on: it is one line every 4 MiB at most, and the
// bar shows the same progress.
func chunkNotice(format string, args ...any) {
	if progressBar == nil {
		fmt.Printf(format, args...)
	}
}

// countBackupEntries walks root as the archive writer will and counts the
// entries WriteFile will see and the bytes of the regular files, leaving out
// what the writer leaves out: system files and folders and the -exclude
// patterns. A link or special file is one entry with no content bytes.
// excludeRoot is the folder as the user named it (the exclude patterns are
// taken from it, also when root is a shadow copy). Walk errors only shorten
// the count; the backup itself will report them.
func countBackupEntries(root, excludeRoot string) (files int, bytes int64) {
	if progressBar != nil {
		progressBar.BeginScan("counting files")
	}
	rootIsVolume := pbscommon.IsVolumeRoot(excludeRoot)
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// An unreadable folder is skipped whole, as the writer does.
			if d != nil && !d.IsDir() {
				return nil
			}
			return filepath.SkipDir
		}
		if path == root {
			return nil
		}
		info, ierr := d.Info()
		if ierr != nil {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		rel := relativeEntryPath(root, path)
		if d.IsDir() {
			// Links and junctions are never followed, as in the archive.
			if pbscommon.IsLinkEntry(path, info) {
				return filepath.SkipDir
			}
			if pbscommon.IsAutoExcludedFolderAt(name, filepath.Dir(path) == root && rootIsVolume) {
				return filepath.SkipDir
			}
			if len(excludeList) > 0 && pbscommon.IsExcludedByPatterns(rel, name, excludeRoot, excludeList, true) {
				return filepath.SkipDir
			}
			return nil
		}
		if pbscommon.IsAutoExcludedFile(name) {
			return nil
		}
		if len(excludeList) > 0 && pbscommon.IsExcludedByPatterns(rel, name, excludeRoot, excludeList, false) {
			return nil
		}
		// Every non-directory entry gets a WriteFile call (a link too), so it
		// counts as one file; only regular files contribute bytes.
		files++
		if info.Mode().IsRegular() {
			bytes += info.Size()
		}
		if progressBar != nil {
			progressBar.ScanStep(path)
		}
		return nil
	})
	return files, bytes
}

// relativeEntryPath is path relative to the walked root, in the form the
// exclusion patterns use (forward slashes, no leading separator).
func relativeEntryPath(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == "." {
		return ""
	}
	return filepath.ToSlash(rel)
}

// progressPath shows a walked path as the user named the folder, also when
// the walk is of a shadow copy.
func progressPath(readRoot, logicalRoot, path string) string {
	rel, err := filepath.Rel(readRoot, path)
	if err != nil || rel == "." {
		return logicalRoot
	}
	return filepath.Join(logicalRoot, rel)
}
