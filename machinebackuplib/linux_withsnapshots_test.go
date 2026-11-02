//go:build linux
// +build linux

package machinebackuplib

import (
	"errors"
	"testing"

	"snapshot"
)

// A disk with nothing mounted needs no snapshot, so it is read even where
// asking the snapshot package would fail: no elastio-snap or dattobd module,
// or (as when this test runs as a normal user) no root.
func TestWithSnapshotsNothingMounted(t *testing.T) {
	read := false
	err := withSnapshots(nil, func(sn map[string]snapshot.SnapShot) error {
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
	if err := withSnapshots(nil, func(map[string]snapshot.SnapShot) error { return want }); !errors.Is(err, want) {
		t.Fatalf("withSnapshots() = %v, want %v", err, want)
	}
}
