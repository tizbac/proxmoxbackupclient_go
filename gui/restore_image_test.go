package main

import (
	"testing"

	"github.com/tizbac/proxmoxbackupclient_go/gui/imagefs"
)

func TestImageListingHelpers(t *testing.T) {
	entries := []imagefs.Entry{
		{Path: "Documents", IsDir: true},
		{Path: "Documents/a.txt"},
		{Path: "Documents/sub", IsDir: true},
		{Path: "Documents/sub/b.txt"},
		{Path: "DocumentsOld/c.txt"},
		{Path: "System Volume Information", IsDir: true},
		{Path: "System Volume Information/{3808876b}"},
		{Path: "system volume information/tracking.log"},
		{Path: "System Volume Informationx.txt"},
	}
	kept := withoutSystemFolders(append([]imagefs.Entry(nil), entries...))
	var paths []string
	for _, e := range kept {
		paths = append(paths, e.Path)
	}
	want := []string{"Documents", "Documents/a.txt", "Documents/sub", "Documents/sub/b.txt", "DocumentsOld/c.txt", "System Volume Informationx.txt"}
	if len(paths) != len(want) {
		t.Fatalf("withoutSystemFolders = %v", paths)
	}
	for i := range want {
		if paths[i] != want[i] {
			t.Fatalf("withoutSystemFolders = %v", paths)
		}
	}

	got := pickEntries(kept, "Documents")
	if len(got) != 4 || got[0].Path != "Documents" || got[3].Path != "Documents/sub/b.txt" {
		t.Errorf("pickEntries(Documents) = %v", got)
	}
	if got := pickEntries(kept, ""); len(got) != len(kept) {
		t.Errorf("pickEntries(\"\") = %d entries", len(got))
	}
}
