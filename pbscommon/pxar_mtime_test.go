package pbscommon

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func mtimeOf(t *testing.T, path string) time.Time {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return info.ModTime()
}

// 1970-01-01 00:00:00 is stored as 0 and anything earlier as a negative
// number; both are real times and must be restored, not replaced by "now".
func TestPXARExtractRestoresPre1970AndEpochMtimes(t *testing.T) {
	pre1970 := int64(-315619200) // 1960-01-01 00:00:00 UTC
	archive := makePXAR(t,
		pxarEntry{name: "old.txt", mode: IFREG | 0644, mtime: uint64(pre1970), data: []byte("old\n")},
		pxarEntry{name: "epoch.txt", mode: IFREG | 0644, mtime: 0, data: []byte("epoch\n")},
	)
	dest := t.TempDir()
	if _, err := NewPXARReader(archive).ExtractAll(dest); err != nil {
		t.Fatalf("ExtractAll: %v", err)
	}
	if got := mtimeOf(t, filepath.Join(dest, "old.txt")); !got.Equal(time.Unix(pre1970, 0)) {
		t.Errorf("old.txt mtime = %v, want %v", got, time.Unix(pre1970, 0))
	}
	if got := mtimeOf(t, filepath.Join(dest, "epoch.txt")); !got.Equal(time.Unix(0, 0)) {
		t.Errorf("epoch.txt mtime = %v, want %v", got, time.Unix(0, 0))
	}
}

func TestPXARListEntriesReportsModTimeSet(t *testing.T) {
	entries, err := NewPXARReader(standardPXAR(t)).ListEntries()
	if err != nil {
		t.Fatalf("ListEntries: %v", err)
	}
	for _, e := range entries {
		if !e.ModTimeSet {
			t.Errorf("%s: ModTimeSet = false, want true", e.Path)
		}
	}
}

// Writing the files into a folder sets its mtime to now; the snapshot's time
// has to be put back after them, also on folders that end up empty.
func TestPXARExtractRestoresFolderMtimes(t *testing.T) {
	dest := t.TempDir()
	if _, err := NewPXARReader(standardPXAR(t)).ExtractAll(dest); err != nil {
		t.Fatalf("ExtractAll: %v", err)
	}
	want := time.Unix(1600000000, 0)
	for _, dir := range []string{"empty-dir", "sub", "sub/nested", "notes"} {
		if got := mtimeOf(t, filepath.Join(dest, filepath.FromSlash(dir))); !got.Equal(want) {
			t.Errorf("%s mtime = %v, want %v", dir, got, want)
		}
	}
}

// A selective restore creates the selection's ancestors only to reach it;
// they are not part of what was asked for, so they keep the time of the
// restore.
func TestPXARExtractSelectedFolderMtimeNotAncestors(t *testing.T) {
	dest := t.TempDir()
	if _, err := NewPXARReader(standardPXAR(t)).ExtractFiltered(dest, []string{"sub/nested"}, false); err != nil {
		t.Fatalf("ExtractFiltered: %v", err)
	}
	snapshot := time.Unix(1600000000, 0)
	if got := mtimeOf(t, filepath.Join(dest, "sub", "nested")); !got.Equal(snapshot) {
		t.Errorf("sub/nested mtime = %v, want %v", got, snapshot)
	}
	if got := mtimeOf(t, filepath.Join(dest, "sub")); got.Equal(snapshot) {
		t.Errorf("sub (ancestor of the selection) got the snapshot time %v", got)
	}
}

// A folder that already exists keeps its own time unless the restore may
// overwrite.
func TestPXARExtractExistingFolderMtimeFollowsOverwrite(t *testing.T) {
	snapshot := time.Unix(1600000000, 0)
	for _, overwrite := range []bool{false, true} {
		dest := t.TempDir()
		if err := os.Mkdir(filepath.Join(dest, "empty-dir"), 0755); err != nil {
			t.Fatal(err)
		}
		own := time.Unix(1700000000, 0)
		if err := os.Chtimes(filepath.Join(dest, "empty-dir"), own, own); err != nil {
			t.Fatal(err)
		}
		if _, err := NewPXARReader(standardPXAR(t)).ExtractFiltered(dest, []string{"empty-dir"}, overwrite); err != nil {
			t.Fatalf("ExtractFiltered: %v", err)
		}
		want := own
		if overwrite {
			want = snapshot
		}
		if got := mtimeOf(t, filepath.Join(dest, "empty-dir")); !got.Equal(want) {
			t.Errorf("overwrite=%v: empty-dir mtime = %v, want %v", overwrite, got, want)
		}
	}
}
