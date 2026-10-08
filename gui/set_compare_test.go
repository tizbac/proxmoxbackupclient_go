//go:build !service

package main

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
	"time"
)

func writeAt(t *testing.T, path string, body string, mtime time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatal(err)
	}
}

func paths(es []IndexEntry) []string {
	var out []string
	for _, e := range es {
		out = append(out, e.Path)
	}
	sort.Strings(out)
	return out
}

func TestWalkLiveFolderSkipsWhatABackupSkips(t *testing.T) {
	root := t.TempDir()
	tm := time.Unix(1_700_000_000, 0)
	writeAt(t, filepath.Join(root, "keep.txt"), "a", tm)
	writeAt(t, filepath.Join(root, "sub", "deep", "keep2.txt"), "b", tm)
	writeAt(t, filepath.Join(root, "scratch.tmp"), "x", tm)
	writeAt(t, filepath.Join(root, "sub", "cache", "c.bin"), "x", tm)
	writeAt(t, filepath.Join(root, ".pbs-rollback", "old", "f.txt"), "x", tm)
	writeAt(t, filepath.Join(root, "$RECYCLE.BIN", "r.txt"), "x", tm)
	writeAt(t, filepath.Join(root, "pagefile.sys"), "x", tm)

	li, err := walkLiveFolder(root, []string{"*.tmp", "sub/cache"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := paths(li.Entries)
	want := []string{"keep.txt", "sub", "sub/deep", "sub/deep/keep2.txt"}
	if len(got) != len(want) {
		t.Fatalf("entries %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("entries %v, want %v", got, want)
		}
	}
}

func TestCompareIndexes(t *testing.T) {
	root := t.TempDir()
	old := time.Unix(1_700_000_000, 0)
	newer := old.Add(time.Hour)
	writeAt(t, filepath.Join(root, "same.txt"), "same", old)
	writeAt(t, filepath.Join(root, "edited.txt"), "edited now", newer)
	writeAt(t, filepath.Join(root, "touched.txt"), "1234", newer) // same size, new time
	writeAt(t, filepath.Join(root, "added.txt"), "new", newer)
	writeAt(t, filepath.Join(root, "newdir", "inside.txt"), "n", newer)
	writeAt(t, filepath.Join(root, "kept", "k.txt"), "k", old)
	writeAt(t, filepath.Join(root, "moved", "report.pdf"), "pdfpdf", old)

	snap := []IndexEntry{
		{Path: backupMetaFileName, Size: 10, MTime: old.Unix()},
		{Path: "same.txt", Size: 4, MTime: old.Unix()},
		{Path: "edited.txt", Size: 6, MTime: old.Unix()},
		{Path: "touched.txt", Size: 4, MTime: old.Unix()},
		{Path: "gone.txt", Size: 9, MTime: old.Unix()},
		{Path: "kept", IsDir: true},
		{Path: "kept/k.txt", Size: 1, MTime: old.Unix()},
		{Path: "olddir", IsDir: true},
		{Path: "olddir/report.pdf", Size: 6, MTime: old.Unix()},
	}
	if runtime.GOOS == "windows" {
		// Case differs only: still the same file on Windows.
		snap[1].Path = "SAME.txt"
	}
	sortIndex(snap)
	li, err := walkLiveFolder(root, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	r := compareIndexes(snap, li)
	if r.Unchanged != 2 {
		t.Errorf("unchanged %d, want 2 (same.txt, kept/k.txt)", r.Unchanged)
	}
	if got := paths(r.Missing); len(got) != 2 || got[0] != "gone.txt" || got[1] != "olddir/report.pdf" {
		t.Errorf("missing %v", got)
	}
	var changed []string
	for _, c := range r.Changed {
		changed = append(changed, c.Snap.Path)
	}
	sort.Strings(changed)
	if len(changed) != 2 || changed[0] != "edited.txt" || changed[1] != "touched.txt" {
		t.Errorf("changed %v", changed)
	}
	if got := paths(r.New); len(got) != 3 || got[0] != "added.txt" || got[1] != "moved/report.pdf" || got[2] != "newdir/inside.txt" {
		t.Errorf("new %v", got)
	}
	if got := paths(r.NewDirs); len(got) != 2 || got[0] != "moved" || got[1] != "newdir" {
		t.Errorf("new dirs %v", got)
	}
	hints := moveHints(r.Missing, li.Entries)
	if hints["olddir/report.pdf"] != "moved/report.pdf" {
		t.Errorf("move hints %v", hints)
	}
	if _, ok := hints["gone.txt"]; ok {
		t.Errorf("gone.txt has no match but got a hint %v", hints)
	}
}

func TestCompareUnreadableFolderIsUnknown(t *testing.T) {
	li := &LiveIndex{Unreadable: []string{"locked"}}
	snap := []IndexEntry{{Path: "locked", IsDir: true}, {Path: "locked/a.txt", Size: 1}, {Path: "b.txt", Size: 1}}
	sortIndex(snap)
	r := compareIndexes(snap, li)
	if r.Unknown != 1 || len(r.Missing) != 1 || r.Missing[0].Path != "b.txt" {
		t.Errorf("unknown %d missing %v", r.Unknown, paths(r.Missing))
	}
}

func TestUndoRollbackPutsEverythingBack(t *testing.T) {
	base := t.TempDir()
	set := filepath.Join(base, "set")
	safety := filepath.Join(base, ".pbs-rollback", "Set", "20260101-000000")
	tm := time.Unix(1_700_000_000, 0)

	// State after a roll back: changed.txt was replaced (old live version in
	// safety), added.txt was removed (in safety), restored.txt and
	// newfolder/r.txt were restored (did not exist), emptydir was removed.
	writeAt(t, filepath.Join(set, "changed.txt"), "snapshot version", tm)
	writeAt(t, filepath.Join(safety, "set", "changed.txt"), "live version", tm.Add(time.Hour))
	writeAt(t, filepath.Join(safety, "set", "added.txt"), "added since", tm.Add(time.Hour))
	writeAt(t, filepath.Join(set, "restored.txt"), "was missing", tm)
	writeAt(t, filepath.Join(set, "newfolder", "r.txt"), "was missing too", tm)
	m := &rollbackManifest{
		JobID: "j", SetName: "Set", Status: "done",
		Moved: []rollbackMove{
			{From: filepath.Join(set, "changed.txt"), To: filepath.Join(safety, "set", "changed.txt"), Kind: "replaced"},
			{From: filepath.Join(set, "added.txt"), To: filepath.Join(safety, "set", "added.txt"), Kind: "removed"},
		},
		Restored:    []string{filepath.Join(set, "changed.txt"), filepath.Join(set, "restored.txt"), filepath.Join(set, "newfolder", "r.txt")},
		CreatedDirs: []string{filepath.Join(set, "newfolder")},
		RemovedDirs: []string{filepath.Join(set, "emptydir")},
		SafetyRoots: []string{safety},
	}
	mp := filepath.Join(safety, "rollback.json")
	if err := writeRollbackManifest(mp, m); err != nil {
		t.Fatal(err)
	}
	a := &App{}
	msg, err := a.UndoRollback(mp)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(msg)
	read := func(p string) string {
		b, err := os.ReadFile(p)
		if err != nil {
			return "<" + err.Error() + ">"
		}
		return string(b)
	}
	if got := read(filepath.Join(set, "changed.txt")); got != "live version" {
		t.Errorf("changed.txt = %q", got)
	}
	if got := read(filepath.Join(set, "added.txt")); got != "added since" {
		t.Errorf("added.txt = %q", got)
	}
	for _, gone := range []string{"restored.txt", "newfolder"} {
		if _, err := os.Stat(filepath.Join(set, gone)); !os.IsNotExist(err) {
			t.Errorf("%s still exists after undo", gone)
		}
	}
	if fi, err := os.Stat(filepath.Join(set, "emptydir")); err != nil || !fi.IsDir() {
		t.Errorf("emptydir not made again: %v", err)
	}
	if _, err := os.Stat(safety); !os.IsNotExist(err) {
		t.Errorf("safety folder left behind after a clean undo")
	}
	if _, err := a.UndoRollback(mp); err == nil {
		t.Errorf("second undo should fail: the record is gone")
	}
}

func TestCheckManifestPath(t *testing.T) {
	for _, bad := range []string{`C:\Windows\system32\rollback.json`, filepath.Join("x", ".pbs-rollback", "a", "other.json")} {
		if checkManifestPath(bad) == nil {
			t.Errorf("%s accepted", bad)
		}
	}
	if err := checkManifestPath(filepath.Join(os.TempDir(), ".pbs-rollback", "Set", "20260101-000000", "rollback.json")); err != nil {
		t.Error(err)
	}
}
