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

// A drive or file system root keeps one separator: C:\Users, not C:\\Users.
func TestUnmapSnapshotPathRoot(t *testing.T) {
	snap := `C:\Users\u\AppData\Roaming\PBSBackupGO\VSS\{ID}`
	if got := unmapSnapshotPathString(snap+`\Users\f`, snap, `C:\`); got != `C:\Users\f` {
		t.Errorf("drive root: %q", got)
	}
	if got := unmapSnapshotPathString("open "+snap, snap, `C:\`); got != `open C:\` {
		t.Errorf("the root itself: %q", got)
	}
	if got := unmapSnapshotPathString("/tmp/.snap/1/var/log", "/tmp/.snap/1", "/"); got != "/var/log" {
		t.Errorf("Linux root: %q", got)
	}
}

// The status sidecar names unreadable files by their real paths, not the
// shadow copy's; without a snapshot the messages are left as they are.
func TestByRealPath(t *testing.T) {
	snap := `C:\Users\u\AppData\Roaming\PBSBackupGO\VSS\{ID}\data`
	msgs := []string{"Cannot open file " + snap + `\locked.db: Access is denied.`, "zero-filled " + snap + `\a\b.txt`}
	got := byRealPath(msgs, true, snap, `C:\data`)
	if got[0] != `Cannot open file C:\data\locked.db: Access is denied.` || got[1] != `zero-filled C:\data\a\b.txt` {
		t.Errorf("through VSS: %q", got)
	}
	if msgs[0] == got[0] {
		t.Error("the input was changed in place")
	}
	if got := byRealPath(msgs, false, snap, `C:\data`); got[0] != msgs[0] {
		t.Errorf("without VSS: %q", got)
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
