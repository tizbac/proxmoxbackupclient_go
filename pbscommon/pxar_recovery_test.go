package pbscommon

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestIsVolumeRoot(t *testing.T) {
	for path, want := range map[string]bool{
		`C:\`: true, `C:`: true, `c:/`: true, `/`: true,
		`\\?\GLOBALROOT\Device\HarddiskVolumeShadowCopy12\`: true,
		`\\?\GLOBALROOT\Device\HarddiskVolumeShadowCopy12`:  true,
		`\\server\share`: true, `\\server\share\`: true,
		`C:\Data`: false, `/home`: false, `\\server\share\dir`: false,
		`\\?\GLOBALROOT\Device\HarddiskVolumeShadowCopy12\Users`: false,
		``: false,
	} {
		if got := isVolumeRoot(path); got != want {
			t.Errorf("isVolumeRoot(%q) = %v, want %v", path, got, want)
		}
	}
}

// backedUpPaths writes root to an archive with ExcludeRoot set to logical (the
// folder as the user named it) and returns the paths read back from it.
func backedUpPaths(t *testing.T, root, logical string) map[string]bool {
	t.Helper()
	var p bytes.Buffer
	archive := &PXARArchive{ArchiveName: "backup.pxar.didx", ExcludeRoot: logical}
	archive.WriteCB = func(b []byte) error { p.Write(b); return nil }
	archive.Create()
	if _, err := archive.WriteDir(root, "", true); err != nil {
		t.Fatalf("WriteDir: %v", err)
	}
	entries, err := NewPXARReader(p.Bytes()).ListEntries()
	if err != nil {
		t.Fatalf("ListEntries: %v", err)
	}
	paths := make(map[string]bool, len(entries))
	for _, e := range entries {
		paths[e.Path] = true
	}
	return paths
}

// Windows keeps its recovery data in a Recovery folder at the root of the
// system volume. A folder called Recovery anywhere else is the user's own and
// must be backed up; the other system folders stay excluded wherever they are.
func TestRecoveryExcludedOnlyAtVolumeRoot(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{
		"Recovery/WindowsRE/winre.wim",
		"Data/Recovery/photo.jpg",
		"Data/keep.txt",
		"System Volume Information/tracking.log",
		"Data/$RECYCLE.BIN/deleted.txt",
	} {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(name), 0644); err != nil {
			t.Fatal(err)
		}
	}

	for _, c := range []struct {
		logical      string
		rootRecovery bool // Recovery/WindowsRE/winre.wim backed up
	}{
		{`C:\`, false},            // a whole drive
		{`/`, false},              // a whole Linux system
		{`\\server\share`, false}, // a whole share
		{`D:\Projects`, true},     // a folder: its Recovery is the user's
		{"", true},                // no logical root: the walked temp folder is not a volume
	} {
		got := backedUpPaths(t, root, c.logical)
		if !got["Data/keep.txt"] {
			t.Fatalf("ExcludeRoot %q: archive is missing Data/keep.txt; read back %v", c.logical, got)
		}
		if !got["Data/Recovery/photo.jpg"] {
			t.Errorf("ExcludeRoot %q: Data/Recovery (not at the root) was left out", c.logical)
		}
		if got["Recovery/WindowsRE/winre.wim"] != c.rootRecovery {
			t.Errorf("ExcludeRoot %q: Recovery at the root backed up = %v, want %v",
				c.logical, got["Recovery/WindowsRE/winre.wim"], c.rootRecovery)
		}
		if got["System Volume Information/tracking.log"] || got["Data/$RECYCLE.BIN/deleted.txt"] {
			t.Errorf("ExcludeRoot %q: a system folder was backed up", c.logical)
		}
	}
}
