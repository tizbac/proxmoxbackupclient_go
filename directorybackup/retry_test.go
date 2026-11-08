package directorybackup

import (
	"errors"
	"fmt"
	"retry"
	"strings"
	"testing"
)

var errBusy = errors.New("HTTP 503 - service unavailable")

// A request that fails in a way that passes is tried again and goes through.
func TestWithRetryPassingFailure(t *testing.T) {
	calls := 0
	err := withRetry("test request", chunkRetryTime, func() error {
		calls++
		if calls < 3 {
			return errBusy
		}
		return nil
	})
	if err != nil || calls != 3 {
		t.Errorf("err %v after %d calls, want nil after 3", err, calls)
	}
}

// Any other failure is returned at once, as the request gave it (no
// "non-retryable error:" in front, so the message reads as before).
func TestWithRetryOtherFailure(t *testing.T) {
	denied := errors.New("HTTP 403 - permission check failed")
	calls := 0
	err := withRetry("test request", chunkRetryTime, func() error { calls++; return denied })
	if err != denied || calls != 1 {
		t.Errorf("err %v after %d calls, want the request's own error after 1", err, calls)
	}
}

// A request that keeps failing gives up after 5 attempts with its own error
// still inside, so a lost session is still recognised as one.
func TestWithRetryGivesUp(t *testing.T) {
	if testing.Short() {
		t.Skip("waits about 15 s")
	}
	lost := fmt.Errorf("upload: %w", errors.New("unexpected EOF"))
	calls := 0
	err := withRetry("test request", chunkRetryTime, func() error { calls++; return lost })
	if calls != 5 || !errors.Is(err, lost) || !strings.Contains(err.Error(), "after 5 attempts") {
		t.Errorf("err %v after %d calls", err, calls)
	}
	if !retry.SessionLost(err) {
		t.Errorf("%v is no longer seen as a lost session", err)
	}
}
