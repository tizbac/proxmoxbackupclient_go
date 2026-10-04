package main

import (
	"reflect"
	"testing"
)

func TestConfigDirs(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
		want []string
	}{
		{"none", Config{}, nil},
		{"single legacy", Config{BackupSourceDir: "/a"}, []string{"/a"}},
		{"list only", Config{BackupSourceDirs: []string{"/a", "/b"}}, []string{"/a", "/b"}},
		{"both, dedup, order", Config{BackupSourceDir: "/a", BackupSourceDirs: []string{"/b", "/a", ""}}, []string{"/a", "/b"}},
	}
	for _, c := range cases {
		if got := c.cfg.Dirs(); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: Dirs() = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestDirListFlag(t *testing.T) {
	var f dirListFlag
	for _, v := range []string{"/a", "/b"} {
		if err := f.Set(v); err != nil {
			t.Fatal(err)
		}
	}
	if !reflect.DeepEqual([]string(f), []string{"/a", "/b"}) || f.String() != "/a,/b" {
		t.Errorf("dirListFlag = %v", f)
	}
}
