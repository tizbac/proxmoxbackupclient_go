//go:build windows

package snapshot

import (
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
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
