//go:build linux
// +build linux

package snapshot

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testGB = uint64(1000 * 1000 * 1000)

// A snapshot is refused under the 10% the modules need for the copy-on-write
// file, and allowed at 10% and above.
func TestCowFreeSpaceNeedsTenPercent(t *testing.T) {
	cases := []struct {
		name       string
		total      uint64
		free       uint64
		wantRefuse bool
	}{
		{"half free", 100 * testGB, 50 * testGB, false},
		{"exactly ten percent", 100 * testGB, 10 * testGB, false},
		{"just under ten percent", 100 * testGB, 10*testGB - 1, true},
		{"nine percent", 100 * testGB, 9 * testGB, true},
		{"nearly full", 100 * testGB, 1 * testGB, true},
		{"size unknown", 0, 0, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := cowFreeSpaceProblem("/home", "/dev/sda2", "ext4", c.total, c.free)
			if (err != nil) != c.wantRefuse {
				t.Fatalf("total %d free %d: err = %v, want refuse %v", c.total, c.free, err, c.wantRefuse)
			}
			if err != nil && !errors.Is(err, ErrNotEnoughSpace) {
				t.Errorf("error is not ErrNotEnoughSpace: %v", err)
			}
		})
	}
}

// The message names the volume, how full it is, what is needed and what to
// do, in the terms the app and the command line use.
func TestCowFreeSpaceMessage(t *testing.T) {
	err := cowFreeSpaceProblem("/home", "/dev/sda2", "ext4", 100*testGB, 3*testGB)
	if err == nil {
		t.Fatal("3% free was not refused")
	}
	msg := err.Error()
	for _, want := range []string{
		"/home",
		"/dev/sda2",
		"ext4",
		"3.0 GB",
		"100.0 GB",
		"3.0%",
		"10%",
		"10.0 GB",
		"7.0 GB",
		"delete or move files away",
		"without a snapshot",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("message %q does not contain %q", msg, want)
		}
	}
}

// A copy-on-write file already on the file system makes the free-space
// figure misleading until createOne has removed a stale one, so the
// pre-flight check steps aside when it sees one.
func TestHasCowFile(t *testing.T) {
	dir := t.TempDir()
	if hasCowFile(dir) {
		t.Fatal("an empty directory has a copy-on-write file")
	}
	cow := filepath.Join(dir, ".pbs_snapshot_3.cow")
	if err := os.WriteFile(cow, nil, 0600); err != nil {
		t.Fatal(err)
	}
	if !hasCowFile(dir) {
		t.Fatalf("%s was not seen", cow)
	}
}

// The file system figures are read from statfs, not guessed.
func TestFreeSpaceReadsStatfs(t *testing.T) {
	dir := t.TempDir()
	total, free, ok := freeSpace(dir)
	if !ok {
		t.Fatal("statfs of a temporary directory failed")
	}
	if total == 0 || free > total {
		t.Fatalf("total %d, free %d: not the file system's figures", total, free)
	}
	// A temporary directory with a sane amount of room passes the check, so
	// an ordinary backup is not blocked by it.
	if total/10 < free {
		if err := checkCowFreeSpace(dir, "/dev/test", "ext4"); err != nil {
			t.Errorf("a file system with %s of %s free was refused: %v", humanBytes(free), humanBytes(total), err)
		}
	} else {
		t.Logf("only %s of %s free here: a real backup would be refused", humanBytes(free), humanBytes(total))
	}
}
