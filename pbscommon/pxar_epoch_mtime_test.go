package pbscommon

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A file and a folder whose mtime is exactly the Unix epoch (1970-01-01
// 00:00:00 UTC) get that time back on restore. The archive always records an
// mtime, so 0 is a real value; it used to be taken for "no mtime", leaving
// the restore time instead (e2e finding 2026-10-02).
func TestPXARExtractEpochModTime(t *testing.T) {
	f := pxarFile("epoch.txt", []byte("1970\n"))
	f.mtime = 0
	d := pxarDir("epoch-dir")
	d.mtime = 0
	archive := makePXAR(t, f, d, pxarFile("epoch-dir/inner.txt", []byte("x")))

	dest := t.TempDir()
	extracted, err := NewPXARReader(archive).ExtractAll(dest)
	if err != nil {
		t.Fatalf("ExtractAll: %v", err)
	}
	info, err := os.Stat(filepath.Join(dest, "epoch.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(time.Unix(0, 0)) {
		t.Errorf("epoch.txt mtime = %v, want 1970-01-01", info.ModTime().UTC())
	}
	var dirSeen bool
	for _, e := range extracted {
		if e.IsDir && filepath.Base(e.Path) == "epoch-dir" {
			dirSeen = true
			if !e.ModTimeSet || e.ModTime != 0 {
				t.Errorf("epoch-dir: ModTime %d, ModTimeSet %v; want 0, true", e.ModTime, e.ModTimeSet)
			}
		}
	}
	if !dirSeen {
		t.Error("epoch-dir not in the extracted list")
	}
}
