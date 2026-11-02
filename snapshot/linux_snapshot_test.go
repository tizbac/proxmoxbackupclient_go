//go:build linux
// +build linux

package snapshot

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestSnapshotEndToEnd(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("must run as root")
	}
	if _, ok := detectControl(); !ok {
		t.Skip("no snapshot control tool (elioctl/dbdctl) installed")
	}
	for _, bin := range []string{"mkfs.ext4", "losetup"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("%s not available", bin)
		}
	}

	work, err := os.MkdirTemp("", "pbs-snaptest-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(work)

	img := filepath.Join(work, "disk.img")
	if err := os.Truncate(img, 256<<20); err != nil {
		f, e := os.Create(img)
		if e != nil {
			t.Fatal(e)
		}
		if e := f.Truncate(256 << 20); e != nil {
			t.Fatal(e)
		}
		f.Close()
	}

	out, err := exec.Command("losetup", "--find", "--show", img).CombinedOutput()
	if err != nil {
		t.Fatalf("losetup: %v: %s", err, out)
	}
	loop := strings.TrimSpace(string(out))
	defer exec.Command("losetup", "-d", loop).Run()

	if out, err := exec.Command("mkfs.ext4", "-q", "-F", loop).CombinedOutput(); err != nil {
		t.Fatalf("mkfs.ext4: %v: %s", err, out)
	}

	mnt := filepath.Join(work, "mnt")
	if err := os.Mkdir(mnt, 0755); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mount(loop, mnt, "ext4", 0, ""); err != nil {
		t.Fatalf("mount %s: %v", loop, err)
	}
	defer syscall.Unmount(mnt, syscall.MNT_DETACH)

	const marker = "point-in-time-ok"
	markerPath := filepath.Join(mnt, "marker.txt")
	if err := os.WriteFile(markerPath, []byte(marker), 0644); err != nil {
		t.Fatal(err)
	}

	called := false
	err = CreateVSSSnapshot([]string{markerPath}, true, func(snaps map[string]SnapShot) error {
		called = true
		snap, ok := snaps[markerPath]
		if !ok || !snap.Valid {
			t.Fatalf("no valid snapshot returned: %+v", snaps)
		}

		if snap.ObjectPath == markerPath {
			t.Fatalf("snapshot fell back to pass-through (module/root issue): %+v", snap)
		}

		fi, err := os.Stat(snap.ObjectPath)
		if err != nil {
			t.Fatalf("stat ObjectPath %s: %v", snap.ObjectPath, err)
		}
		if fi.Mode()&os.ModeDevice == 0 {
			t.Fatalf("ObjectPath %s is not a device (mode %v)", snap.ObjectPath, fi.Mode())
		}
		t.Logf("ObjectPath (raw frozen device) = %s", snap.ObjectPath)

		got, err := os.ReadFile(snap.FullPath)
		if err != nil {
			t.Fatalf("read FullPath %s: %v", snap.FullPath, err)
		}
		if string(got) != marker {
			t.Fatalf("snapshot content = %q, want %q", got, marker)
		}
		t.Logf("FullPath (ro snapshot mount) = %s -> content OK", snap.FullPath)
		return nil
	})
	if err != nil {
		t.Fatalf("CreateVSSSnapshot: %v", err)
	}
	if !called {
		t.Fatal("backup callback was never invoked")
	}

	VSSCleanup()
	trackedMu.Lock()
	n := len(tracked)
	trackedMu.Unlock()
	if n != 0 {
		t.Fatalf("%d snapshot(s) left tracked after cleanup", n)
	}
}
