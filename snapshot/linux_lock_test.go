//go:build linux

package snapshot

import (
	"bufio"
	"context"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"
)

func flockNB(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
}

func TestLockFileFor(t *testing.T) {
	saved := lockDir
	lockDir = "/run"
	defer func() { lockDir = saved }()
	if got := lockFileFor("/dev/sda3"); got != "/run/salvanza-snap-dev-sda3.lock" {
		t.Errorf("lockFileFor(/dev/sda3) = %s", got)
	}
	if got := lockFileFor("/dev/mapper/vg0-root"); got != "/run/salvanza-snap-dev-mapper-vg0-root.lock" {
		t.Errorf("lockFileFor(/dev/mapper/vg0-root) = %s", got)
	}
}

// A device whose lock another program holds is busy; the other devices of
// the same call are let go again, so two programs never each hold one the
// other waits for. flock locks belong to the open file, so a second open in
// this process stands in for the other program.
func TestTryLockDevices(t *testing.T) {
	saved := lockDir
	lockDir = t.TempDir()
	defer func() { lockDir = saved }()

	other, err := os.OpenFile(lockFileFor("/dev/sdb1"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if err := flockNB(other); err != nil {
		t.Fatal(err)
	}

	busy, err := tryLockDevices([]string{"/dev/sda1", "/dev/sdb1"})
	if err != nil || busy != "/dev/sdb1" {
		t.Fatalf("busy %q, err %v; want /dev/sdb1 busy", busy, err)
	}
	if len(devLocks) != 0 {
		t.Errorf("held %d locks after a busy device; want none", len(devLocks))
	}

	// Waiting ends when the other program lets go.
	go func() { time.Sleep(150 * time.Millisecond); other.Close() }()
	savedPoll := lockPoll
	lockPoll = 20 * time.Millisecond
	defer func() { lockPoll = savedPoll }()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := lockDevicesAcrossPrograms(ctx, []string{"/dev/sda1", "/dev/sdb1"}); err != nil {
		t.Fatal(err)
	}
	if len(devLocks) != 2 {
		t.Errorf("holds %d locks; want 2", len(devLocks))
	}
	pid, _ := os.ReadFile(lockFileFor("/dev/sda1"))
	if strings.TrimSpace(string(pid)) == "" {
		t.Error("the lock file does not say who holds it")
	}
	unlockDevices([]string{"/dev/sda1", "/dev/sdb1"})
	if len(devLocks) != 0 {
		t.Errorf("still holds %d locks after unlockDevices", len(devLocks))
	}

	// A cancelled wait ends with the context's error.
	again, _ := os.OpenFile(lockFileFor("/dev/sdb1"), os.O_RDWR, 0o600)
	defer again.Close()
	if err := flockNB(again); err != nil {
		t.Fatal(err)
	}
	short, cancel2 := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel2()
	if err := lockDevicesAcrossPrograms(short, []string{"/dev/sdb1"}); err == nil {
		t.Error("lockDevicesAcrossPrograms returned without the lock and without an error")
	}
}

func TestParseMountsOf(t *testing.T) {
	info := `22 1 8:3 / / rw,relatime shared:1 - ext4 /dev/sda3 rw
95 22 252:0 / /tmp/pbs-snap-2402894724 ro,relatime - ext4 /dev/elastio-snap0 ro
96 22 252:1 / /tmp/pbs-snap-77 ro,relatime - ext4 /dev/elastio-snap1 ro
97 22 252:0 / /mnt/with\040space ro,relatime - ext4 /dev/elastio-snap0 ro
`
	got := parseMountsOf(bufio.NewScanner(strings.NewReader(info)), "/dev/elastio-snap0")
	want := []string{"/tmp/pbs-snap-2402894724", "/mnt/with space"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("mounts of /dev/elastio-snap0 = %q, want %q", got, want)
	}
	if got := parseMountsOf(bufio.NewScanner(strings.NewReader(info)), "/dev/elastio-snap9"); len(got) != 0 {
		t.Errorf("mounts of an unmounted device = %q", got)
	}
}

func TestOnSnapshot(t *testing.T) {
	mounts := []string{"/tmp/pbs-snap-1"}
	devices := []string{"/dev/elastio-snap0"}
	for path, want := range map[string]bool{
		"/tmp/pbs-snap-1":            true,
		"/tmp/pbs-snap-1/home/x.txt": true,
		"/tmp/pbs-snap-10/x":         false,
		"/dev/elastio-snap0":         true,
		"/dev/elastio-snap01":        false,
		"/home/x.txt":                false,
	} {
		if got := onSnapshot(path, mounts, devices); got != want {
			t.Errorf("onSnapshot(%s) = %v, want %v", path, got, want)
		}
	}
}
