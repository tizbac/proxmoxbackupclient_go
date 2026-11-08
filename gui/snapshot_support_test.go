package main

import (
	"os"
	"testing"
)

func TestSnapshotSupportLocalFolder(t *testing.T) {
	dir := t.TempDir()
	got := (&App{}).SnapshotSupport([]string{dir, os.TempDir()})
	if len(got) != 2 || !got[dir] {
		t.Fatalf("local temp folder should support a snapshot: %v", got)
	}
}
