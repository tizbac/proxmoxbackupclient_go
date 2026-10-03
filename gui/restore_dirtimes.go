package main

import (
	"os"
	"runtime"
	"sort"
	"time"

	"pbscommon"
)

// applyDirectoryTimes sets each restored folder's modification time from the
// snapshot. It runs after every file is written because writing into a folder
// changes that folder's own mtime. Deepest folders go first so the order never
// matters, and a failure on one folder is counted, never fatal: the content is
// already safely on disk. onProgress is called at most every 250 ms.
func applyDirectoryTimes(extracted []pbscommon.PXARExtractedFile, cancelled func() bool, onProgress func(done, total int)) (applied, failed int) {
	var dirs []pbscommon.PXARExtractedFile
	for _, f := range extracted {
		if f.IsDir && !f.Skipped && f.ModTime != 0 { // negative = pre-1970, still valid
			dirs = append(dirs, f)
		}
	}
	sort.SliceStable(dirs, func(i, j int) bool { return len(dirs[i].Path) > len(dirs[j].Path) })

	var last time.Time
	for n, d := range dirs {
		if cancelled != nil && cancelled() {
			break
		}
		t := time.Unix(d.ModTime, int64(d.ModNanos))
		if err := os.Chtimes(d.Path, t, t); err != nil {
			failed++
			writeBackupLog("Folder timestamp apply failed for " + d.Path + ": " + err.Error())
		} else {
			applied++
		}
		if onProgress != nil && time.Since(last) >= 250*time.Millisecond {
			last = time.Now()
			onProgress(n+1, len(dirs))
		}
	}
	return applied, failed
}

// applyDirectoryOwnership restores each folder's recorded owner and full mode
// bits. Only called when the restore runs as root on Linux (see
// restoreOwnershipEnabled). Deepest folders first, failures ignored: the
// content is already on disk.
func applyDirectoryOwnership(extracted []pbscommon.PXARExtractedFile) {
	var dirs []pbscommon.PXARExtractedFile
	for _, f := range extracted {
		if f.IsDir && !f.Skipped {
			dirs = append(dirs, f)
		}
	}
	sort.SliceStable(dirs, func(i, j int) bool { return len(dirs[i].Path) > len(dirs[j].Path) })
	for _, d := range dirs {
		pbscommon.ApplyDirMetadata(d.Path, d.RawMode, d.UID, d.GID)
	}
}

// restoreOwnershipEnabled: owners and setuid/setgid/sticky bits are restored
// only for a root restore on Linux. Archives written on Windows record a fixed
// 1000:1000 and 0777, so applying them for an ordinary user would only loosen
// permissions.
func restoreOwnershipEnabled() bool {
	return runtime.GOOS == "linux" && os.Geteuid() == 0
}
