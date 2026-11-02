package pbscommon

import (
	"os"
	"path/filepath"
	"testing"
)

// folderKeepTimes maps each restored folder's archive path to KeepOwnTime.
func folderKeepTimes(files []PXARExtractedFile) map[string]bool {
	out := map[string]bool{}
	for _, f := range files {
		if f.IsDir && !f.Skipped {
			out[f.ArchivePath] = f.KeepOwnTime
		}
	}
	return out
}

// A selective restore creates the selection's ancestors only to reach them,
// so they keep the time of the restore; the selection's own folders take the
// snapshot's. A full restore into an empty folder marks nothing. Both
// extractors behave the same.
func TestExtractMarksAncestorFoldersKeepOwnTime(t *testing.T) {
	for _, parallel := range []bool{false, true} {
		extract := func(includes []string) []PXARExtractedFile {
			t.Helper()
			dest := t.TempDir()
			rewriter := func(p string) string { return filepath.Join(dest, filepath.FromSlash(p)) }
			r := NewPXARReader(standardPXAR(t))
			var files []PXARExtractedFile
			var err error
			if parallel {
				files, err = r.ExtractWithRewriterParallel(rewriter, includes, false, 4)
			} else {
				files, err = r.ExtractWithRewriter(rewriter, includes, false)
			}
			if err != nil {
				t.Fatalf("parallel=%v: extract: %v", parallel, err)
			}
			return files
		}

		keep := folderKeepTimes(extract([]string{"sub/nested"}))
		if v, ok := keep["sub"]; !ok || !v {
			t.Errorf("parallel=%v: sub (ancestor of the selection) KeepOwnTime = %v (listed %v), want true", parallel, v, ok)
		}
		if v, ok := keep["sub/nested"]; !ok || v {
			t.Errorf("parallel=%v: sub/nested (the selection) KeepOwnTime = %v (listed %v), want false", parallel, v, ok)
		}

		for path, v := range folderKeepTimes(extract(nil)) {
			if v {
				t.Errorf("parallel=%v: full restore marked %s KeepOwnTime", parallel, path)
			}
		}
	}
}

// A folder that is already there keeps its own time unless the restore may
// overwrite.
func TestExtractExistingFolderKeepOwnTimeFollowsOverwrite(t *testing.T) {
	for _, overwrite := range []bool{false, true} {
		dest := t.TempDir()
		if err := os.Mkdir(filepath.Join(dest, "empty-dir"), 0755); err != nil {
			t.Fatal(err)
		}
		files, err := NewPXARReader(standardPXAR(t)).ExtractFiltered(dest, []string{"empty-dir"}, overwrite)
		if err != nil {
			t.Fatalf("ExtractFiltered: %v", err)
		}
		v, ok := folderKeepTimes(files)["empty-dir"]
		if !ok || v != !overwrite {
			t.Errorf("overwrite=%v: empty-dir KeepOwnTime = %v (listed %v), want %v", overwrite, v, ok, !overwrite)
		}
	}
}
