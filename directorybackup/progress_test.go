package directorybackup

import (
	"os"
	"path/filepath"
	"testing"
)

// The pre-pass counts the regular files the archive will read, leaving out
// what -exclude and the built-in skips leave out.
func TestCountBackupEntries(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"a.txt":          "12345",
		"sub/b.bin":      "abc",
		"sub/skip.tmp":   "ignored",
		"pagefile.sys":   "system file",
		"sub/deep/c.dat": "xy",
		"sub/other.dat":  "z",
	}
	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// A link is listed as a file but has no content bytes, like the archive writer.
	linked := true
	if err := os.Symlink("a.txt", filepath.Join(dir, "link")); err != nil {
		linked = false
		t.Logf("cannot make a symlink here, the rest of the test still counts: %v", err)
	}

	oldExcludes := excludeList
	excludeList = []string{"*.tmp"}
	defer func() { excludeList = oldExcludes }()
	oldBar := progressBar
	progressBar = nil
	defer func() { progressBar = oldBar }()

	gotFiles, gotBytes := countBackupEntries(dir, dir)
	// a.txt 5, sub/b.bin 3, sub/deep/c.dat 2, sub/other.dat 1
	wantFiles := 4
	if linked {
		wantFiles++ // the symlink is one entry, with no bytes
	}
	if gotFiles != wantFiles || gotBytes != 11 {
		t.Errorf("counted %d files, %d bytes; want %d files, 11 bytes", gotFiles, gotBytes, wantFiles)
	}
}
