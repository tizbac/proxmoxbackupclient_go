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

// A local folder can be snapshotted.
func TestPathSupportsSnapshotLocal(t *testing.T) {
	if dir := t.TempDir(); !PathSupportsSnapshot(dir) {
		t.Errorf("local folder %s: no snapshot", dir)
	}
}
