//go:build windows

package main

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// creationTimesKept: this OS has a settable creation time.
const creationTimesKept = true

func setCreated(t *testing.T, path string, when time.Time) {
	t.Helper()
	ft := windows.NsecToFiletime(when.UnixNano())
	created := int64(ft.HighDateTime)<<32 | int64(ft.LowDateTime)
	if err := applyCreationTime(path, created); err != nil {
		t.Fatalf("set creation time on %s: %v", path, err)
	}
}

func readCreated(t *testing.T, path string) time.Time {
	t.Helper()
	fi, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	d := fi.Sys().(*syscall.Win32FileAttributeData)
	return time.Unix(0, d.CreationTime.Nanoseconds())
}

// The collector records the creation time of files and folders, and
// applyCreationTime puts it back exactly, leaving the modification time alone.
func TestCreationTimeRoundTrip(t *testing.T) {
	src := t.TempDir()
	file := filepath.Join(src, "doc.txt")
	dir := filepath.Join(src, "sub")
	if err := os.WriteFile(file, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	created := time.Date(2003, 4, 5, 6, 7, 8, 123456700, time.UTC)
	setCreated(t, file, created)
	setCreated(t, dir, created.Add(time.Hour))

	c := NewNTFSMetaCollector(src, "h")
	for _, p := range []struct {
		path  string
		isDir bool
	}{{file, false}, {dir, true}} {
		fi, err := os.Lstat(p.path)
		if err != nil {
			t.Fatal(err)
		}
		if err := c.Collect(p.path, fi, p.isDir); err != nil {
			t.Fatal(err)
		}
	}
	meta, err := c.FinalizeRaw()
	if err != nil || meta == nil {
		t.Fatalf("FinalizeRaw: %v %v", meta, err)
	}
	byPath := buildFileMetaIndex(meta)
	if byPath["doc.txt"].Created == 0 || byPath["sub"].Created == 0 {
		t.Fatalf("creation time not captured: %+v", meta.Entries)
	}

	// "Restore": fresh file and folder, then put the captured times back.
	dst := t.TempDir()
	rf := filepath.Join(dst, "doc.txt")
	rd := filepath.Join(dst, "sub")
	if err := os.WriteFile(rf, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(rd, 0o755); err != nil {
		t.Fatal(err)
	}
	mtime := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	if err := os.Chtimes(rf, mtime, mtime); err != nil {
		t.Fatal(err)
	}
	if err := applyCreationTime(rf, byPath["doc.txt"].Created); err != nil {
		t.Fatal(err)
	}
	if err := applyCreationTime(rd, byPath["sub"].Created); err != nil {
		t.Fatal(err)
	}
	if got := readCreated(t, rf); !got.Equal(created) {
		t.Errorf("file creation time = %v, want %v", got.UTC(), created)
	}
	if got := readCreated(t, rd); !got.Equal(created.Add(time.Hour)) {
		t.Errorf("folder creation time = %v, want %v", got.UTC(), created.Add(time.Hour))
	}
	fi, _ := os.Lstat(rf)
	if !fi.ModTime().Equal(mtime) {
		t.Errorf("modification time changed to %v, want %v", fi.ModTime().UTC(), mtime)
	}
	if err := applyCreationTime(rf, 0); err != nil {
		t.Errorf("0 (not captured) should be a no-op, got %v", err)
	}
}
