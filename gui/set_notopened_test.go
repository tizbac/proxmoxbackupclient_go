package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// refuseWalk makes walkLive refuse the folders in shut the way an access
// denied open does, whatever account the test runs as.
func refuseWalk(t *testing.T, shut ...string) {
	t.Helper()
	orig := walkLive
	t.Cleanup(func() { walkLive = orig })
	walkLive = func(root string, excludes []string, keepLinks bool, cancel func() bool, progress func(n int)) (*LiveIndex, error) {
		for _, s := range shut {
			if root == s {
				return nil, &fs.PathError{Op: "open", Path: root, Err: os.ErrPermission}
			}
		}
		return orig(root, excludes, keepLinks, cancel, progress)
	}
}

func TestLiveFoldersSkipsAFolderNotOpened(t *testing.T) {
	root := t.TempDir()
	open, shut := filepath.Join(root, "open"), filepath.Join(root, "shut")
	for _, d := range []string{open, shut} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "a.txt"), []byte("a"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	refuseWalk(t, shut)
	job := ScheduledJob{BackupType: "directory", BackupDirs: []string{open, shut}}
	live, err := (&App{}).liveFolders(job, snapshotIndex{}, SetSnapshot{}, nil)
	if err != nil {
		t.Fatalf("one folder that opens should be enough: %v", err)
	}
	folders := setFoldersOf(job)
	if li := live[folders[0].Base]; li == nil || li.NotOpened || !li.HasFile("a.txt") {
		t.Fatalf("open folder: %+v", li)
	}
	if li := live[folders[1].Base]; li == nil || !li.NotOpened {
		t.Fatalf("refused folder should be NotOpened: %+v", li)
	}
	if got := notOpenedDirs(folders, live); len(got) != 1 || got[0] != shut {
		t.Fatalf("notOpenedDirs = %v", got)
	}

	// Nothing in it is missing, only unknown, so a roll back leaves it alone.
	c := compareIndexes([]IndexEntry{{Path: "a.txt", Size: 1}, {Path: "b/c.txt", Size: 1}}, live[folders[1].Base])
	if len(c.Missing) != 0 || c.Unknown != 2 {
		t.Fatalf("compare with a folder not opened: missing %d unknown %d", len(c.Missing), c.Unknown)
	}
}

func TestLiveFoldersFailsWhenNoFolderOpens(t *testing.T) {
	shut := filepath.Join(t.TempDir(), "shut")
	if err := os.MkdirAll(shut, 0o755); err != nil {
		t.Fatal(err)
	}
	refuseWalk(t, shut)
	job := ScheduledJob{BackupType: "directory", BackupDirs: []string{shut}}
	_, err := (&App{}).liveFolders(job, snapshotIndex{}, SetSnapshot{}, nil)
	if err == nil || !strings.Contains(err.Error(), "Couldn't open "+shut+" as the logged in user") {
		t.Fatalf("err = %v", err)
	}
}

func TestLiveFoldersStillFailsOnAMissingFolder(t *testing.T) {
	gone := filepath.Join(t.TempDir(), "gone")
	job := ScheduledJob{BackupType: "directory", BackupDirs: []string{gone, t.TempDir()}}
	_, err := (&App{}).liveFolders(job, snapshotIndex{}, SetSnapshot{}, nil)
	if err == nil || !strings.Contains(err.Error(), "cannot read "+gone) {
		t.Fatalf("a folder that is not there is still an error: %v", err)
	}
}

func TestOpenedSincePreview(t *testing.T) {
	if got := openedSincePreview([]string{`\\nas\a`, `\\nas\b`}, []string{`\\nas\b`}); len(got) != 1 || got[0] != `\\nas\a` {
		t.Fatalf("opened since preview = %v", got)
	}
	if got := openedSincePreview([]string{`\\nas\a`}, []string{`\\nas\a`, `\\nas\c`}); len(got) != 0 {
		t.Fatalf("a folder that stopped opening is left alone, not an error: %v", got)
	}
}
