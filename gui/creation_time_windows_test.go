//go:build windows

package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// setCreatedForTest sets only the creation time of path (file or folder).
func setCreatedForTest(t *testing.T, path string, when time.Time) {
	t.Helper()
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	h, err := windows.CreateFile(p, windows.FILE_WRITE_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil,
		windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer windows.CloseHandle(h)
	ft := windows.NsecToFiletime(when.UnixNano())
	if err := windows.SetFileTime(h, &ft, nil, nil); err != nil {
		t.Fatalf("set creation time on %s: %v", path, err)
	}
}

func readCreatedForTest(t *testing.T, path string) time.Time {
	t.Helper()
	fi, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	d := fi.Sys().(*syscall.Win32FileAttributeData)
	return time.Unix(0, d.CreationTime.Nanoseconds())
}

// collectForTest runs the collector over the given paths and returns the
// side file as a restore reads it (gzip, then JSON).
func collectForTest(t *testing.T, root string, paths map[string]bool) *BackupFileMeta {
	t.Helper()
	c := NewNTFSMetaCollector(root, "h")
	for p, isDir := range paths {
		fi, err := os.Lstat(p)
		if err != nil {
			t.Fatal(err)
		}
		if err := c.Collect(p, fi, isDir); err != nil {
			t.Fatal(err)
		}
	}
	blob, err := c.Finalize()
	if err != nil || len(blob) == 0 {
		t.Fatalf("Finalize: %d bytes, %v", len(blob), err)
	}
	gz, err := gzip.NewReader(bytes.NewReader(blob))
	if err != nil {
		t.Fatal(err)
	}
	var meta BackupFileMeta
	if err := json.NewDecoder(gz).Decode(&meta); err != nil {
		t.Fatal(err)
	}
	return &meta
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
	setCreatedForTest(t, file, created)
	setCreatedForTest(t, dir, created.Add(time.Hour))

	byPath := buildFileMetaIndex(collectForTest(t, src, map[string]bool{file: false, dir: true}))
	if byPath["doc.txt"].Created == 0 || byPath["sub"].Created == 0 {
		t.Fatalf("creation time not captured: %+v", byPath)
	}

	// "Restore": a fresh file and folder, then the captured times put back.
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
	if err := os.Chtimes(rd, mtime, mtime); err != nil {
		t.Fatal(err)
	}
	if err := applyCreationTime(rf, byPath["doc.txt"].Created); err != nil {
		t.Fatal(err)
	}
	if err := applyCreationTime(rd, byPath["sub"].Created); err != nil {
		t.Fatal(err)
	}
	if got := readCreatedForTest(t, rf); !got.Equal(created) {
		t.Errorf("file creation time = %v, want %v", got.UTC(), created)
	}
	if got := readCreatedForTest(t, rd); !got.Equal(created.Add(time.Hour)) {
		t.Errorf("folder creation time = %v, want %v", got.UTC(), created.Add(time.Hour))
	}
	for _, p := range []string{rf, rd} {
		if fi, _ := os.Lstat(p); !fi.ModTime().Equal(mtime) {
			t.Errorf("%s: modification time changed to %v, want %v", p, fi.ModTime().UTC(), mtime)
		}
	}
}

// A side file written before the field existed has no "c": it decodes with
// Created 0, and applyCreationTime then leaves the file alone.
func TestCreationTimeAbsentIsNoOp(t *testing.T) {
	var meta BackupFileMeta
	old := `{"version":1,"sddl":[""],"entries":[{"p":"doc.txt","s":0,"a":32}]}`
	if err := json.Unmarshal([]byte(old), &meta); err != nil {
		t.Fatal(err)
	}
	if meta.Entries[0].Created != 0 {
		t.Fatalf("Created = %d, want 0", meta.Entries[0].Created)
	}
	f := filepath.Join(t.TempDir(), "doc.txt")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := readCreatedForTest(t, f)
	if err := applyCreationTime(f, meta.Entries[0].Created); err != nil {
		t.Fatalf("0 (not captured) should be a no-op, got %v", err)
	}
	if got := readCreatedForTest(t, f); !got.Equal(before) {
		t.Errorf("creation time changed from %v to %v", before, got)
	}
}
