//go:build windows

package snapshot

import (
	"errors"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/go-ole/go-ole"
	"github.com/st-matskevich/go-vss"
)

// statuses returns a QueryStatus stand-in that gives each status in turn,
// then the last one for good, and counts the calls.
func statuses(calls *int, list ...vss.HRESULT) func() (vss.HRESULT, error) {
	return func() (vss.HRESULT, error) {
		i := *calls
		*calls++
		if i >= len(list) {
			i = len(list) - 1
		}
		return list[i], nil
	}
}

func TestPollVSSFinishedAtOnce(t *testing.T) {
	calls := 0
	err := pollVSS("step", statuses(&calls, vss.VSS_S_ASYNC_FINISHED), time.Now().Add(time.Second), time.Second, 10*time.Millisecond)
	if err != nil || calls != 1 {
		t.Fatalf("got %v after %d calls", err, calls)
	}
}

// Wait returned with the step still running: wait on, and it succeeds once
// it is done (tizbac/proxmoxbackupclient_go#67).
func TestPollVSSPendingThenFinished(t *testing.T) {
	calls := 0
	p := vss.VSS_S_ASYNC_PENDING
	err := pollVSS("step", statuses(&calls, p, p, p, vss.VSS_S_ASYNC_FINISHED), time.Now().Add(5*time.Second), 5*time.Second, 10*time.Millisecond)
	if err != nil || calls != 4 {
		t.Fatalf("got %v after %d calls", err, calls)
	}
}

// Still pending when the step's time is up: fail, not wait for ever.
func TestPollVSSPendingPastDeadline(t *testing.T) {
	calls := 0
	start := time.Now()
	err := pollVSS("VSS_GATHER - GatherWriterMetadata", statuses(&calls, vss.VSS_S_ASYNC_PENDING), start.Add(100*time.Millisecond), 100*time.Millisecond, 10*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "still pending after 100ms") || !strings.Contains(err.Error(), "GatherWriterMetadata") {
		t.Fatalf("want a still-pending error, got %v", err)
	}
	if took := time.Since(start); took < 100*time.Millisecond || took > 2*time.Second || calls < 2 {
		t.Errorf("gave up after %v and %d calls", took, calls)
	}
}

// Already past the deadline when Wait returns (it waited the whole time):
// one look, then fail.
func TestPollVSSPendingAfterFullWait(t *testing.T) {
	calls := 0
	err := pollVSS("step", statuses(&calls, vss.VSS_S_ASYNC_PENDING), time.Now().Add(-time.Millisecond), time.Minute, 10*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "still pending") || calls != 1 {
		t.Fatalf("got %v after %d calls", err, calls)
	}
}

func TestPollVSSCancelled(t *testing.T) {
	calls := 0
	p := vss.VSS_S_ASYNC_PENDING
	err := pollVSS("step", statuses(&calls, p, vss.VSS_S_ASYNC_CANCELLED), time.Now().Add(5*time.Second), 5*time.Second, 10*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "cancelled") || calls != 2 {
		t.Fatalf("got %v after %d calls", err, calls)
	}
}

// The step failed: QueryStatus gives its error HRESULT as the status.
func TestPollVSSErrorStatus(t *testing.T) {
	calls := 0
	p := vss.VSS_S_ASYNC_PENDING
	err := pollVSS("step", statuses(&calls, p, vss.VSS_E_UNEXPECTED_WRITER_ERROR), time.Now().Add(5*time.Second), 5*time.Second, 10*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "bad status - 0x80042315") || calls != 2 {
		t.Fatalf("got %v after %d calls", err, calls)
	}
}

// QueryStatus itself fails.
func TestPollVSSQueryStatusFails(t *testing.T) {
	boom := errors.New("IVssAsync.QueryStatus: boom")
	err := pollVSS("step", func() (vss.HRESULT, error) { return 0, boom }, time.Now().Add(time.Second), time.Second, 10*time.Millisecond)
	if !errors.Is(err, boom) {
		t.Fatalf("want the QueryStatus error, got %v", err)
	}
}

// initCOM makes the thread multithreaded; a second call on it is counted
// (S_FALSE) and undone; on a thread someone else made single-threaded it
// works without undoing what is not ours.
func TestInitCOM(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	uninit, err := initCOM()
	if err != nil {
		t.Fatal(err)
	}
	again, err := initCOM()
	if err != nil {
		t.Fatalf("second initCOM on the same thread: %v", err)
	}
	again()
	// Still multithreaded: asking for a single-threaded apartment is refused.
	var oe *ole.OleError
	if err := ole.CoInitialize(0); !errors.As(err, &oe) || oe.Code() != comRPCEChangedMode {
		if err == nil || (oe != nil && oe.Code() == comSFalse) {
			ole.CoUninitialize()
		}
		t.Errorf("thread is not multithreaded after initCOM: CoInitialize gave %v", err)
	}
	uninit()

	if err := ole.CoInitialize(0); err != nil {
		t.Fatalf("CoInitialize: %v", err)
	}
	defer ole.CoUninitialize()
	keep, err := initCOM()
	if err != nil {
		t.Fatalf("initCOM on a single-threaded thread: %v", err)
	}
	keep()
	// The single-threaded COM is still there: initialising it again is S_FALSE.
	err = ole.CoInitialize(0)
	if !errors.As(err, &oe) || oe.Code() != comSFalse {
		t.Errorf("single-threaded COM was undone by initCOM: CoInitialize gave %v", err)
	}
	if err == nil || (errors.As(err, &oe) && oe.Code() == comSFalse) {
		ole.CoUninitialize()
	}
}
