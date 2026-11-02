//go:build windows

package snapshot

import (
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
	"unsafe"

	"github.com/go-ole/go-ole"
	"golang.org/x/sys/windows"
)

// Opt-in (PBS_VSS_VOL = a volume to snapshot, e.g. "T:\"; run elevated):
// takes a snapshot, uses it, and checks the shadow copy is gone once
// CreateVSSSnapshot returns, while this process is still alive. The GUI
// stays running between backups, so a shadow that lingered until the
// process exited stayed for hours, kept copy-on-write running on the
// volume and showed up as used space in the next machine backup.
func TestLiveVSSSnapshotReleased(t *testing.T) {
	vol := os.Getenv("PBS_VSS_VOL")
	if vol == "" {
		t.Skip("set PBS_VSS_VOL")
	}
	var id string
	err := CreateVSSSnapshot([]string{vol}, false, func(sn map[string]SnapShot) error {
		for _, s := range sn {
			id = s.Id
		}
		if !shadowListed(t, id) {
			t.Errorf("shadow %s not listed while in use", id)
		}
		// The machine backup reads on other goroutines and returns from
		// one; do the same so the release runs after a thread change.
		done := make(chan struct{})
		go func() { time.Sleep(200 * time.Millisecond); close(done) }()
		<-done
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if id == "" {
		t.Fatal("no snapshot id")
	}
	// Deletion of an auto-release shadow may lag the release a little.
	start := time.Now()
	for shadowListed(t, id) {
		if time.Since(start) > 20*time.Second {
			t.Fatalf("shadow %s still exists %v after CreateVSSSnapshot returned", id, time.Since(start).Round(time.Second))
		}
		time.Sleep(time.Second)
	}
	t.Logf("shadow %s released (gone %v after return)", id, time.Since(start).Round(100*time.Millisecond))
}

// Opt-in (PBS_VSS_VOL and PBS_VSS_HANDLES=<snapshots>; run elevated): takes
// that many snapshots one after another and reports how many handles the
// process gained, for the whole snapshot and its parts, so a leak per
// snapshot shows. The GUI and the service run for weeks, a snapshot or more
// a day. 2026-10-21 on Server 2022: 33 per snapshot with go-vss's own
// calls, 21 with createShadow, all in create and release (none in delete,
// in the VSS object itself, or in vssadmin).
func TestLiveVSSHandles(t *testing.T) {
	vol := os.Getenv("PBS_VSS_VOL")
	n, _ := strconv.Atoi(os.Getenv("PBS_VSS_HANDLES"))
	if vol == "" || n <= 0 {
		t.Skip("set PBS_VSS_VOL and PBS_VSS_HANDLES")
	}
	snap := func() {
		if err := CreateVSSSnapshot([]string{vol}, false, func(map[string]SnapShot) error { return nil }); err != nil {
			t.Fatal(err)
		}
	}
	getCount := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetProcessHandleCount")
	count := func() uint32 {
		var c uint32
		if r, _, err := getCount.Call(uintptr(windows.CurrentProcess()), uintptr(unsafe.Pointer(&c))); r == 0 {
			t.Fatal(err)
		}
		return c
	}
	snap() // the first loads the VSS libraries
	snap()
	measure := func(what string, step func()) {
		before := count()
		for i := 0; i < n; i++ {
			step()
		}
		after := count()
		t.Logf("%s x%d: handles %d -> %d (%+.1f each)", what, n, before, after, float64(int(after)-int(before))/float64(n))
	}
	measure("whole snapshot", snap)

	// The parts, to find a leak. COM on one locked thread, as in
	// CreateVSSSnapshot.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ole.CoInitialize(0)
	defer ole.CoUninitialize()
	measure("vssadmin list writers", func() {
		exec.Command("vssadmin", "list", "writers").CombinedOutput()
	})
	var ids []string
	measure("create and release", func() {
		s, err := createShadow(vol, vssTimeoutMs)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, s.Id)
		if err := s.release(); err != nil {
			t.Fatal(err)
		}
	})
	measure("delete", func() {
		id := ids[0]
		ids = ids[1:]
		if !deleteShadow(id) {
			t.Fatalf("could not delete %s", id)
		}
	})
	measure("components only", func() {
		c, err := newVSSComponents()
		if err != nil {
			t.Fatal(err)
		}
		c.Release()
	})
}

// Opt-in (PBS_VSS_CLEANUP=1; run elevated): VSSCleanup, the startup sweep,
// deletes the shadows this client left behind (those with a live marker in
// <appData>\VSS) and leaves every other shadow alone.
func TestLiveVSSCleanup(t *testing.T) {
	if os.Getenv("PBS_VSS_CLEANUP") != "1" {
		t.Skip("set PBS_VSS_CLEANUP=1")
	}
	before := shadowCount(t)
	if err := VSSCleanup(); err != nil {
		t.Fatal(err)
	}
	t.Logf("shadows before %d, after %d", before, shadowCount(t))
}

func shadowCount(t *testing.T) int {
	out, _ := exec.Command("vssadmin", "list", "shadows").CombinedOutput()
	return strings.Count(string(out), "Shadow Copy ID:")
}

func shadowListed(t *testing.T, id string) bool {
	out, err := exec.Command("vssadmin", "list", "shadows").CombinedOutput()
	if err != nil && !strings.Contains(string(out), "No items found") {
		t.Fatalf("vssadmin list shadows: %v %s", err, out)
	}
	return strings.Contains(strings.ToLower(string(out)), strings.ToLower(strings.Trim(id, "{}")))
}
