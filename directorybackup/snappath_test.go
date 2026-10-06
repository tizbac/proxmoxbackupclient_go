package main

import (
	"errors"
	"io/fs"
	"testing"
)

func TestUnmapSnapshotPath(t *testing.T) {
	snap := `C:\Users\u\AppData\Roaming\PBSBackupGO\VSS\{ID}\data\a`
	orig := `C:\data\a`

	inner := fs.ErrNotExist
	err := unmapSnapshotPath(errors.New("stat "+snap+`\sub\f.txt: `+inner.Error()), snap, orig)
	if got, want := err.Error(), `stat C:\data\a\sub\f.txt: `+inner.Error(); got != want {
		t.Fatalf("got %q, want %q", got, want)
	}

	wrapped := unmapSnapshotPath(errors.Join(errors.New("open "+snap), inner), snap, orig)
	if !errors.Is(wrapped, inner) {
		t.Fatal("the original error chain must stay reachable")
	}

	if got := unmapSnapshotPathString("open "+snap+`\f`, snap, orig); got != `open C:\data\a\f` {
		t.Fatalf("string mapping wrong: %q", got)
	}
}

func TestUnmapSnapshotPathNoop(t *testing.T) {
	if unmapSnapshotPath(nil, "x", "y") != nil {
		t.Fatal("nil error must stay nil")
	}
	e := errors.New("boom")
	if unmapSnapshotPath(e, "", "/data") != e {
		t.Fatal("empty snapshot dir must leave the error alone")
	}
	if unmapSnapshotPath(e, "/data", "/data") != e {
		t.Fatal("same path must leave the error alone")
	}
	if got := unmapSnapshotPathString("abc", "", "/data"); got != "abc" {
		t.Fatalf("empty snapshot dir must not alter the string: %q", got)
	}
}
