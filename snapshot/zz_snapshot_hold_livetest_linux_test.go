//go:build linux

package snapshot

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// Opt-in, as root with elastio-snap or dattobd loaded: PBS_SNAP_HOLD=<a
// folder on a snapshottable filesystem>. Two backups snapshot the same
// filesystem at once, as two Backup Sets running side by side do. The second
// must wait for the first's snapshot to end (one per block device) instead
// of destroying it, each must read the file as it was when its own snapshot
// was taken, and a third stopped while it waits must end at once.
// PBS_SNAP_HOLD2 (a folder on another block device) is snapshotted while the
// first holds its device, and must not wait.
func TestLiveSnapshotHold(t *testing.T) {
	dir := os.Getenv("PBS_SNAP_HOLD")
	if dir == "" {
		t.Skip("set PBS_SNAP_HOLD to a folder on a snapshottable filesystem (as root)")
	}
	work, err := os.MkdirTemp(dir, "snaphold-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(work)
	file := filepath.Join(work, "data.bin")
	oldData := randomBytes(t, 4<<20)
	if err := os.WriteFile(file, oldData, 0o600); err != nil {
		t.Fatal(err)
	}
	oldSum := sum(oldData)

	var mu sync.Mutex
	var firstReads []string
	var firstEnded, secondEntered time.Time
	firstIn := make(chan struct{})
	firstDone := make(chan error, 1)
	go func() {
		firstDone <- CreateVSSSnapshotCtx(context.Background(), []string{work}, true, func(sn map[string]SnapShot) error {
			p := filepath.Join(sn[work].FullPath, "data.bin")
			mu.Lock()
			firstReads = append(firstReads, readSum(t, p))
			mu.Unlock()
			close(firstIn)
			// Hold the snapshot while the others ask for theirs, then read
			// through it again: it must still be there.
			time.Sleep(6 * time.Second)
			mu.Lock()
			firstReads = append(firstReads, readSum(t, p))
			firstEnded = time.Now()
			mu.Unlock()
			return nil
		})
	}()
	<-firstIn

	// The live file changes after the first snapshot was taken.
	newData := randomBytes(t, 4<<20)
	if err := os.WriteFile(file, newData, 0o600); err != nil {
		t.Fatal(err)
	}
	newSum := sum(newData)

	secondAsked := time.Now()
	var secondRead string
	secondDone := make(chan error, 1)
	go func() {
		secondDone <- CreateVSSSnapshotCtx(context.Background(), []string{work}, true, func(sn map[string]SnapShot) error {
			mu.Lock()
			secondEntered = time.Now()
			mu.Unlock()
			secondRead = readSum(t, filepath.Join(sn[work].FullPath, "data.bin"))
			return nil
		})
	}()

	// A third, stopped while it waits.
	ctx, cancel := context.WithCancel(context.Background())
	thirdDone := make(chan error, 1)
	go func() {
		thirdDone <- CreateVSSSnapshotCtx(ctx, []string{work}, true, func(map[string]SnapShot) error {
			t.Error("the stopped snapshot ran its backup")
			return nil
		})
	}()
	time.Sleep(time.Second)
	stopAt := time.Now()
	cancel()
	select {
	case err := <-thirdDone:
		if err == nil || !strings.Contains(err.Error(), "stopped while waiting") {
			t.Errorf("stopped snapshot returned %v", err)
		}
		t.Logf("stopped while waiting: ended in %s", time.Since(stopAt).Round(time.Millisecond))
	case <-time.After(3 * time.Second):
		t.Error("a snapshot stopped while waiting did not end")
	}

	// Another device is not held up.
	if other := os.Getenv("PBS_SNAP_HOLD2"); other != "" {
		start := time.Now()
		err := CreateVSSSnapshotCtx(context.Background(), []string{other}, true, func(map[string]SnapShot) error { return nil })
		if err != nil {
			t.Errorf("snapshot of another device: %v", err)
		} else if d := time.Since(start); d > 4*time.Second {
			t.Errorf("snapshot of another device took %s: it waited for the first", d)
		} else {
			mu.Lock()
			held := firstEnded.IsZero()
			mu.Unlock()
			if !held {
				t.Errorf("the first snapshot ended before the other device's was taken: no proof it did not wait")
			}
			t.Logf("another device snapshotted in %s while the first held its own", d.Round(time.Millisecond))
		}
	}

	if err := <-firstDone; err != nil {
		t.Fatalf("first snapshot: %v", err)
	}
	if err := <-secondDone; err != nil {
		t.Fatalf("second snapshot: %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	for i, s := range firstReads {
		if s != oldSum {
			t.Errorf("first backup's read %d through its snapshot = %s, want the file as it was (%s)", i+1, s[:12], oldSum[:12])
		}
	}
	if secondRead != newSum {
		t.Errorf("second backup read %s through its snapshot, want the changed file (%s)", secondRead[:12], newSum[:12])
	}
	if secondEntered.Before(firstEnded) {
		t.Errorf("second snapshot was in use at %s, before the first ended at %s", secondEntered, firstEnded)
	}
	t.Logf("second snapshot waited %s for the first; both read their own copy", secondEntered.Sub(secondAsked).Round(time.Millisecond))
	trackedMu.Lock()
	left := len(tracked)
	trackedMu.Unlock()
	devMu.Lock()
	held := len(devHeld)
	devMu.Unlock()
	if left != 0 || held != 0 {
		t.Errorf("%d snapshots and %d held devices left behind", left, held)
	}
}

func randomBytes(t *testing.T, n int) []byte {
	t.Helper()
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		t.Fatal(err)
	}
	return b
}

func sum(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func readSum(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Errorf("read %s: %v", p, err)
		return strings.Repeat("0", 64)
	}
	if bytes.Equal(b, nil) {
		return sum(nil)
	}
	return sum(b)
}
