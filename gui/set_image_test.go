package main

import (
	"testing"

	"github.com/tizbac/proxmoxbackupclient_go/gui/imagefs"
)

func TestParseImageBase(t *testing.T) {
	ib, ok := parseImageBase("drive-sata1.img.fidx@407896064:Users/Mick Beeby/Documents")
	if !ok || ib.archive != "drive-sata1.img.fidx" || ib.offset != 407896064 || ib.sub != "Users/Mick Beeby/Documents" {
		t.Fatalf("%+v %v", ib, ok)
	}
	if ib, ok := parseImageBase("drive-sata0.img.fidx@1048576:"); !ok || ib.sub != "" {
		t.Fatalf("volume root: %+v %v", ib, ok)
	}
	for _, bad := range []string{"data.pxar.didx", "drive-sata0.img.fidx", "drive-sata0.img.fidx@x:a", "drive-sata0.img.fidx@12"} {
		if _, ok := parseImageBase(bad); ok {
			t.Errorf("%q parsed", bad)
		}
	}
	// archiveFor matches a snapshot's "archive" to a folder base exactly.
	snap := SetSnapshot{Archives: []string{"drive-sata1.img.fidx@407896064:Users/x"}}
	if archiveFor(snap, "drive-sata1.img.fidx@407896064:Users/x") == "" {
		t.Fatal("archiveFor")
	}
}

func TestFolderEntries(t *testing.T) {
	all := []imagefs.Entry{
		{Path: "Users", IsDir: true},
		{Path: "Users/Bob", IsDir: true},
		{Path: "Users/bob/Documents", IsDir: true}, // other case than the set's folder
		{Path: "Users/bob/Documents/a.txt", Size: 3, MTime: 132224078450000000},
		{Path: "Users/bob/Documents/pagefile.sys", Size: 9},  // auto-excluded file
		{Path: "Users/bob/Documents/$RECYCLE.BIN", IsDir: true}, // auto-excluded folder...
		{Path: "Users/bob/Documents/$RECYCLE.BIN/x.txt", Size: 1}, // ...and what is in it
		{Path: "Users/bob/Documents/sub", IsDir: true},
		{Path: "Users/bob/Documents/sub/b.tmp", Size: 2}, // set pattern
		{Path: "Users/bob/Documents/sub/c.doc", Size: 4},
		{Path: "Users/bob/Documentsx/d.txt", Size: 5}, // a sibling with the same prefix
	}
	got := folderEntries(all, "Users/Bob/Documents", `C:\Users\Bob\Documents`, []string{"*.tmp"})
	want := map[string]bool{"a.txt": false, "sub": true, "sub/c.doc": false}
	if len(got) != len(want) {
		t.Fatalf("got %+v", got)
	}
	for _, e := range got {
		isDir, ok := want[e.Path]
		if !ok || isDir != e.IsDir {
			t.Errorf("unexpected %+v", e)
		}
		if e.Path == "a.txt" && e.MTime != 1577934245 {
			t.Errorf("a.txt mtime %d (Unix seconds expected)", e.MTime)
		}
	}
}
