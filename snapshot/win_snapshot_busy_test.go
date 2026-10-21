//go:build windows

package snapshot

import (
	"errors"
	"testing"
	"time"

	"github.com/st-matskevich/go-vss"
)

func TestIsShadowAlreadyInProgress(t *testing.T) {
	for _, c := range []struct {
		err  error
		want bool
	}{
		{errors.New("VSS_START - Shadow copy creation failed: StartSnapshotSet, err VSS_E_SNAPSHOT_SET_IN_PROGRESS - Another shadow copy creation is already in progress. Wait a few moments and try again."), true},
		{errors.New("IVssBackupComponents.StartSnapshotSet: 0x80042316"), true},
		{errors.New("VSS_E_BAD_STATE - A function call was made when the object was in an incorrect state."), false},
		{errors.New("VSS_SNAPSHOT - Shadow copy creation failed: DoSnapshotSet didn't finish properly, err: async operation is pending"), false},
		{nil, false},
	} {
		if got := isShadowAlreadyInProgress(c.err); got != c.want {
			t.Errorf("%v: got %v, want %v", c.err, got, c.want)
		}
	}
}

func shortBusyWait(t *testing.T, wait, retry time.Duration) {
	t.Helper()
	ow, or := vssBusyWait, vssBusyRetry
	vssBusyWait, vssBusyRetry = wait, retry
	t.Cleanup(func() { vssBusyWait, vssBusyRetry = ow, or })
}

var errBusy = errors.New("VSS_E_SNAPSHOT_SET_IN_PROGRESS - Another shadow copy creation is already in progress. Wait a few moments and try again.")

// VSS busy with another shadow copy: wait and try again, and the snapshot
// made once it is free is the one returned.
func TestRetryWhileBusyWaitsForVSS(t *testing.T) {
	shortBusyWait(t, time.Second, 10*time.Millisecond)
	calls := 0
	snap, err := retryWhileBusy(func() (*vssShadow, error) {
		calls++
		if calls < 4 {
			return nil, errBusy
		}
		return &vssShadow{Snapshot: vss.Snapshot{Id: "ok"}}, nil
	})
	if err != nil || snap == nil || snap.Id != "ok" || calls != 4 {
		t.Fatalf("got %v %v after %d calls", snap, err, calls)
	}
}

// Still busy when the wait is up: give up with the busy error, not hang.
func TestRetryWhileBusyGivesUp(t *testing.T) {
	shortBusyWait(t, 100*time.Millisecond, 20*time.Millisecond)
	calls := 0
	start := time.Now()
	_, err := retryWhileBusy(func() (*vssShadow, error) { calls++; return nil, errBusy })
	if !isShadowAlreadyInProgress(err) {
		t.Fatalf("want the busy error, got %v", err)
	}
	if took := time.Since(start); took > time.Second || calls < 2 {
		t.Errorf("gave up after %v and %d calls", took, calls)
	}
}

// Any other failure is not retried.
func TestRetryWhileBusyOtherErrorAtOnce(t *testing.T) {
	shortBusyWait(t, time.Second, 10*time.Millisecond)
	calls := 0
	_, err := retryWhileBusy(func() (*vssShadow, error) { calls++; return nil, errors.New("VSS_E_BAD_STATE") })
	if err == nil || calls != 1 {
		t.Fatalf("got %v after %d calls", err, calls)
	}
}
