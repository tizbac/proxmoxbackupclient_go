//go:build linux
// +build linux

package machinebackuplib

import (
	"errors"
	"os"
	"strings"
	"testing"

	"snapshot"
)

// A disk with nothing mounted needs no snapshot, so it is read even where
// asking the snapshot package would fail: no elastio-snap or dattobd module,
// or (as when this test runs as a normal user) no root.
func TestWithSnapshotsNothingMounted(t *testing.T) {
	read := false
	err := withSnapshots(nil, false, func(sn map[string]snapshot.SnapShot) error {
		read = true
		if len(sn) != 0 {
			t.Errorf("got %d snapshots, want none", len(sn))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("withSnapshots(nothing mounted) = %v, want the disk read without a snapshot", err)
	}
	if !read {
		t.Fatal("the disk was not read")
	}
}

// The reader's error comes back unchanged.
func TestWithSnapshotsNothingMountedReadError(t *testing.T) {
	want := errors.New("read failed")
	if err := withSnapshots(nil, false, func(map[string]snapshot.SnapShot) error { return want }); !errors.Is(err, want) {
		t.Fatalf("withSnapshots() = %v, want %v", err, want)
	}
}

// With the set's Use snapshots off and accepted, a mounted disk is read
// live, with no snapshot taken (nothing here needs root or a module).
func TestWithSnapshotsLiveRead(t *testing.T) {
	read := false
	err := withSnapshots([]string{"/"}, true, func(sn map[string]snapshot.SnapShot) error {
		read = true
		if len(sn) != 0 {
			t.Errorf("got %d snapshots, want none", len(sn))
		}
		return nil
	})
	if err != nil || !read {
		t.Fatalf("live read: err %v, read %v", err, read)
	}
}

// Without that, a mounted disk that cannot be snapshotted is not read, and
// the message says how to read it live instead.
func TestWithSnapshotsRefusesWithoutSnapshot(t *testing.T) {
	if os.Geteuid() == 0 {
		if _, ok := snapshot.DetectControl(); ok {
			t.Skip("root with a snapshot module: a snapshot can be taken here")
		}
	}
	read := false
	err := withSnapshots([]string{"/"}, false, func(map[string]snapshot.SnapShot) error {
		read = true
		return nil
	})
	if err == nil || read {
		t.Fatalf("mounted without a snapshot: err %v, read %v; want refused", err, read)
	}
	if !strings.Contains(err.Error(), "turn off Use snapshots") || !strings.Contains(err.Error(), "-liveread") {
		t.Errorf("message %q does not say how to read live, in the app and on the command line", err)
	}
}
