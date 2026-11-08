package pbscommon

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// With SetRestoreTimes(false) extracted files keep the time they were
// written, in the sequential and the parallel extractor; by default they get
// the archive's time.
func TestPXARExtractRestoreTimesOff(t *testing.T) {
	old := time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC)
	f := pxarFile("old.txt", []byte("2001\n"))
	f.mtime = uint64(old.Unix())
	archive := makePXAR(t, f)

	for _, tc := range []struct {
		name     string
		parallel bool
		restore  bool
	}{
		{"sequential default", false, true},
		{"sequential off", false, false},
		{"parallel default", true, true},
		{"parallel off", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dest := t.TempDir()
			r := NewPXARReader(archive)
			if !tc.restore {
				r.SetRestoreTimes(false)
			}
			rewriter := func(p string) string { return filepath.Join(dest, filepath.FromSlash(p)) }
			start := time.Now().Add(-time.Minute)
			var err error
			if tc.parallel {
				_, err = r.ExtractWithRewriterParallel(rewriter, nil, true, 4)
			} else {
				_, err = r.ExtractWithRewriter(rewriter, nil, true)
			}
			if err != nil {
				t.Fatal(err)
			}
			info, err := os.Stat(filepath.Join(dest, "old.txt"))
			if err != nil {
				t.Fatal(err)
			}
			got := info.ModTime()
			if tc.restore && !got.Equal(old) {
				t.Errorf("mtime = %v, want %v", got.UTC(), old)
			}
			if !tc.restore && got.Before(start) {
				t.Errorf("mtime = %v, want the time of the restore", got.UTC())
			}
		})
	}
}
