package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"pbscommon"
)

func TestApplyDirectoryTimes(t *testing.T) {
	root := t.TempDir()
	outer := filepath.Join(root, "outer")
	inner := filepath.Join(outer, "inner")
	if err := os.MkdirAll(inner, 0755); err != nil {
		t.Fatal(err)
	}
	// A file written after the folders exist moves their mtime to "now".
	if err := os.WriteFile(filepath.Join(inner, "f.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	pre1970 := time.Date(1969, 12, 31, 23, 59, 59, 0, time.UTC).Unix()
	extracted := []pbscommon.PXARExtractedFile{
		{Path: outer, IsDir: true, ModTime: 1000000000},
		{Path: inner, IsDir: true, ModTime: pre1970},
		{Path: filepath.Join(root, "skipped"), IsDir: true, Skipped: true, ModTime: 5},
		{Path: filepath.Join(root, "nomtime"), IsDir: true, ModTime: 0},
		{Path: filepath.Join(inner, "f.txt"), ModTime: 42},
	}
	applied, failed := applyDirectoryTimes(extracted, nil, nil)
	if applied != 2 || failed != 0 {
		t.Fatalf("applied=%d failed=%d, want 2/0", applied, failed)
	}
	for path, want := range map[string]int64{outer: 1000000000, inner: pre1970} {
		fi, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := fi.ModTime().Unix(); got != want {
			t.Errorf("%s mtime = %d, want %d", path, got, want)
		}
	}
	if fi, _ := os.Stat(filepath.Join(inner, "f.txt")); fi.ModTime().Unix() == 42 {
		t.Error("file mtime must not be touched by the folder pass")
	}

	_, failed = applyDirectoryTimes([]pbscommon.PXARExtractedFile{{Path: filepath.Join(root, "gone"), IsDir: true, ModTime: 9}}, nil, nil)
	if failed != 1 {
		t.Errorf("missing folder: failed=%d, want 1", failed)
	}
	applied, _ = applyDirectoryTimes(extracted, func() bool { return true }, nil)
	if applied != 0 {
		t.Errorf("cancelled run applied %d, want 0", applied)
	}
}
