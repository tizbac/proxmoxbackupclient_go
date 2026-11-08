package main

import "testing"

func TestSnapshotTreeIndex(t *testing.T) {
	entries := []SnapshotEntry{
		{Path: "Data", IsDir: true},
		{Path: "Data/a.txt", Size: 10},
		{Path: "Data/sub", IsDir: true},
		{Path: "Data/sub/b.txt", Size: 100},
		{Path: "Data/empty", IsDir: true},
		{Path: "Orphan/deep/c.txt", Size: 1000}, // parents never emitted
		{Path: "top.txt", Size: 5},
	}
	idx := buildSnapshotTreeIndex(snapshotCacheKey{}, entries)

	if got := len(idx.children[""]); got != 2 {
		t.Errorf("root children = %d, want 2 (Data, top.txt): %v", got, idx.children[""])
	}
	if got := len(idx.children["Data"]); got != 3 {
		t.Errorf("Data children = %d, want 3", got)
	}
	if got := len(idx.children["Orphan/deep"]); got != 1 {
		t.Errorf("Orphan/deep children = %d, want 1", got)
	}

	cases := []struct {
		name  string
		paths []string
		want  uint64
	}{
		{"none", nil, 0},
		{"file", []string{"Data/a.txt"}, 10},
		{"dir", []string{"Data"}, 110},
		{"subdir", []string{"Data/sub"}, 100},
		{"empty dir", []string{"Data/empty"}, 0},
		{"dir plus its own child counted once", []string{"Data", "Data/a.txt", "Data/sub"}, 110},
		{"two siblings", []string{"Data/a.txt", "top.txt"}, 15},
		{"file under parentless dir", []string{"Orphan/deep/c.txt"}, 1000},
		{"unknown path", []string{"nope"}, 0},
	}
	for _, c := range cases {
		if got := idx.selectionBytes(c.paths); got != c.want {
			t.Errorf("%s: selectionBytes = %d, want %d", c.name, got, c.want)
		}
	}
	if idx.dirBytes[""] != 1115 {
		t.Errorf("root total = %d, want 1115", idx.dirBytes[""])
	}
}
