package pbscommon

import (
	"os"
	"path/filepath"
	"testing"
)

// The archive reports the file content as it is written (OnData, including
// reused and padded bytes) and every file it starts (OnFile, OnFileDone), which
// is what the CLI's progress bar counts.
func TestArchiveProgressHooks(t *testing.T) {
	root := t.TempDir()
	sizes := map[string]int{"a.txt": 5, "sub/b.bin": 7, "sub/deep/c": 11}
	for name, size := range sizes {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, make([]byte, size), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	var started, done []string
	var data int
	a := &PXARArchive{
		WriteCB:        func([]byte) error { return nil },
		CatalogWriteCB: func([]byte) error { return nil },
		OnFile:         func(path string) { started = append(started, path) },
		OnFileDone:     func(path string) { done = append(done, path) },
		OnData:         func(n int) { data += n },
	}
	a.Create()
	if _, err := a.WriteDir(root, "", true); err != nil {
		t.Fatal(err)
	}
	wantData := 0
	for _, size := range sizes {
		wantData += size
	}
	if data != wantData {
		t.Errorf("OnData counted %d bytes, want %d", data, wantData)
	}
	if len(started) != len(sizes) || len(done) != len(sizes) {
		t.Errorf("OnFile %d paths, OnFileDone %d paths; want %d each", len(started), len(done), len(sizes))
	}
}
