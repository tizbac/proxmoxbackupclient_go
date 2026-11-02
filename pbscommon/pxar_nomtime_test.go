package pbscommon

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// makeTimedTree creates top.txt, a/b/one.txt and a/c/two.txt under root.
func makeTimedTree(t *testing.T, root string) {
	t.Helper()
	for name, data := range map[string]string{
		"top.txt":     "top\n",
		"a/b/one.txt": "one\n",
		"a/c/two.txt": "two\n",
	} {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(data), 0644); err != nil {
			t.Fatal(err)
		}
	}
}

// setTreeTimes gives every file and folder under root (root included) the
// time ts, as a folder whose contents came and went would end up.
func setTreeTimes(t *testing.T, root string, ts time.Time) {
	t.Helper()
	for _, name := range []string{"top.txt", "a/b/one.txt", "a/c/two.txt", "a/b", "a/c", "a", "."} {
		if err := os.Chtimes(filepath.Join(root, filepath.FromSlash(name)), ts, ts); err != nil {
			t.Fatal(err)
		}
	}
}

// writeTestArchive backs root up as a classic archive, the CLI's default, and
// returns the PXAR stream and the catalog.
func writeTestArchive(t *testing.T, root string, noMTime bool) (pxar, catalog []byte) {
	t.Helper()
	var p, c bytes.Buffer
	archive := &PXARArchive{ArchiveName: "backup.pxar.didx", NoMTime: noMTime}
	archive.WriteCB = func(b []byte) error { p.Write(b); return nil }
	archive.CatalogWriteCB = func(b []byte) error { c.Write(b); return nil }
	archive.Create()
	if _, err := archive.WriteDir(root, "", true); err != nil {
		t.Fatalf("WriteDir: %v", err)
	}
	return p.Bytes(), c.Bytes()
}

// With NoMTime, two backups of the same contents are byte for byte the same
// however the times moved in between, so they share every chunk. Without it
// they differ, which shows the comparison would catch a leaked time.
func TestPXARNoMTimeArchiveIgnoresTimeChanges(t *testing.T) {
	for _, noMTime := range []bool{true, false} {
		root := t.TempDir()
		makeTimedTree(t, root)
		setTreeTimes(t, root, time.Unix(1600000000, 0))
		pxar1, cat1 := writeTestArchive(t, root, noMTime)
		setTreeTimes(t, root, time.Unix(1700000000, 0))
		pxar2, cat2 := writeTestArchive(t, root, noMTime)

		samePXAR, sameCatalog := bytes.Equal(pxar1, pxar2), bytes.Equal(cat1, cat2)
		if noMTime && (!samePXAR || !sameCatalog) {
			t.Errorf("NoMTime: archive same = %v, catalog same = %v; want both true", samePXAR, sameCatalog)
		}
		if !noMTime && (samePXAR || sameCatalog) {
			t.Errorf("real times: archive same = %v, catalog same = %v; want both false", samePXAR, sameCatalog)
		}
	}
}

// What NoMTime records reads back as a time of 0, and a restore then dates
// the files 1970-01-01.
func TestPXARNoMTimeRecordsEpoch(t *testing.T) {
	root := t.TempDir()
	makeTimedTree(t, root)
	setTreeTimes(t, root, time.Unix(1600000000, 0))
	pxar, catalog := writeTestArchive(t, root, true)

	entries, err := NewPXARReader(pxar).ListEntries()
	if err != nil {
		t.Fatalf("ListEntries: %v", err)
	}
	if len(entries) != 6 {
		t.Errorf("ListEntries: %d entries, want 6", len(entries))
	}
	for _, e := range entries {
		if e.ModTime != 0 || !e.ModTimeSet {
			t.Errorf("%s: ModTime = %d (set %v), want 0 (set true)", e.Path, e.ModTime, e.ModTimeSet)
		}
	}

	listed, err := ParseCatalog(catalog)
	if err != nil {
		t.Fatalf("ParseCatalog: %v", err)
	}
	for _, e := range listed {
		if !e.IsDir && e.ModTime != 0 {
			t.Errorf("catalog %s: ModTime = %d, want 0", e.Path, e.ModTime)
		}
	}

	dest := t.TempDir()
	if _, err := NewPXARReader(pxar).ExtractAll(dest); err != nil {
		t.Fatalf("ExtractAll: %v", err)
	}
	for _, name := range []string{"top.txt", "a/b/one.txt", "a/c/two.txt"} {
		info, err := os.Stat(filepath.Join(dest, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		if got := info.ModTime(); !got.Equal(time.Unix(0, 0)) {
			t.Errorf("restored %s mtime = %v, want 1970-01-01", name, got)
		}
	}
}

// Metadata change detection compares times, which NoMTime does not record:
// with it a file edited in place (same size) must still be read, so every
// file is read and nothing is reused from the previous snapshot.
func TestPXARNoMTimeTurnsOffReuse(t *testing.T) {
	root := reuseTree(t)
	store := map[[32]byte][]byte{}

	r1 := runSplitBackupNoMTime(t, root, store, nil)
	checkRestore(t, "run 1", r1, root)

	p := filepath.Join(root, "big", "b4.bin")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	b[0] ^= 0xff
	if err := os.WriteFile(p, b, 0644); err != nil {
		t.Fatal(err)
	}

	r2 := runSplitBackupNoMTime(t, root, store, r1)
	checkRestore(t, "run 2", r2, root)
	if st := r2.reuse.Stats; st.Files != 0 {
		t.Errorf("run 2 reused %d files with NoMTime, want 0", st.Files)
	}
	if r2.opened["b4.bin"] != 1 {
		t.Errorf("run 2: edited b4.bin opened %d times, want 1", r2.opened["b4.bin"])
	}
}

// runSplitBackupNoMTime is runSplitBackup with NoMTime set.
func runSplitBackupNoMTime(t *testing.T, root string, store map[[32]byte][]byte, prev *splitRun) *splitRun {
	t.Helper()
	splitNoMTime = true
	defer func() { splitNoMTime = false }()
	return runSplitBackup(t, root, store, prev)
}
