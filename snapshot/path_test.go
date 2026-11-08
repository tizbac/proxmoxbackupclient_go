package snapshot

import "testing"

func TestVolumeRoot(t *testing.T) {
	for _, c := range []struct{ path, want string }{
		{`C:\Users\alice`, `C:\`},
		{`d:`, `d:\`},
		{`\\nas\share\folder\sub`, `\\nas\share\`},
		{`\\nas\share`, `\\nas\share\`},
		{`\\nas`, ""},
		{`\\`, ""},
		{`/home/alice`, ""},
		{`relative\path`, ""},
	} {
		if got := volumeRoot(c.path); got != c.want {
			t.Errorf("volumeRoot(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}

func TestIsNetworkPath(t *testing.T) {
	for _, c := range []struct {
		path string
		want bool
	}{
		{`\\nas\share\folder`, true},
		{`\\nas\share`, true},
		{`//nas/share/folder`, true},
		{`\\?\UNC\nas\share\folder`, true},
		{`\\?\unc\nas\share`, true},
		{`\\?\C:\very\long\path`, false},
		{`\\.\PhysicalDrive0`, false},
		{`C:\Users\alice`, false},
		{`/home/alice`, false},
		{`relative\path`, false},
	} {
		if got := isNetworkPath(c.path); got != c.want {
			t.Errorf("isNetworkPath(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

// A local folder can be snapshotted.
func TestPathSupportsSnapshotLocal(t *testing.T) {
	if dir := t.TempDir(); !PathSupportsSnapshot(dir) {
		t.Errorf("local folder %s: no snapshot", dir)
	}
}
