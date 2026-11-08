package pbscommon

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// A source disk that briefly drops off the bus (a USB disk on a hub that
// re-enumerates, seen live on 2026-10-07 at 79% of a 448 GB backup) used to
// fail the whole job on the first failed read, throwing hours of work away.
// The disk was back 14 seconds later. So when a file operation fails with a
// "device went away" error, the walk now waits for the source to come back
// and carries on:
//
//   - the source returns within sourceWaitWindow: the operation is retried
//     (a file being read is reopened and read from where it stopped), and the
//     backup continues with nothing lost. A notice is logged.
//   - the file's folder is reachable again but the file itself keeps failing
//     (a bad sector, say): only that file is affected. A read in progress is
//     zero-padded to its declared size and flagged, exactly like a file that
//     shrank; a file that cannot be opened is skipped and flagged.
//   - the source does not come back within the window: the backup fails, as
//     before, since carrying on would only record every remaining file as
//     unreadable.
//
// Errors that do not point at the device (access denied, file deleted, a
// sharing violation) are not waited for: they keep their old handling.

// sourceWaitWindow is how long the walk waits for a vanished source to come
// back before giving up on the backup.
var sourceWaitWindow = 2 * time.Minute

// sourceRetryDelays are the pauses between attempts; the last one repeats.
var sourceRetryDelays = []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 5 * time.Second}

// sourceReachableAttempts is how many times an operation is retried once the
// file's folder is reachable again before the failure is put down to the file
// itself rather than the device.
const sourceReachableAttempts = 4

// maxReadRecoveries caps how many times one file's read is resumed after the
// source dropped out; a file that keeps failing is zero-padded and flagged.
const maxReadRecoveries = 5

// Test seams.
var (
	openForRead = func(path string) (io.ReadSeekCloser, error) { return os.Open(path) }
	sourceStat  = os.Stat
	sourceSleep = func(ctx context.Context, d time.Duration) error {
		if ctx == nil {
			time.Sleep(d)
			return nil
		}
		t := time.NewTimer(d)
		defer t.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			return nil
		}
	}
	sourceNow = time.Now
)

// SourceLostError fails the backup: the source stayed unavailable for the
// whole wait window.
type SourceLostError struct {
	Path   string
	Err    error
	Window time.Duration
}

func (e *SourceLostError) Error() string {
	return fmt.Sprintf("source became unavailable at %s and did not come back within %s: %v", e.Path, e.Window, e.Err)
}

func (e *SourceLostError) Unwrap() error { return e.Err }

// notice logs an informational message through OnNotice, if set.
func (a *PXARArchive) notice(msg string) {
	if a.OnNotice != nil {
		a.OnNotice(msg)
	}
}

// sourceUnavailable reports whether err (from an operation on path) means the
// device holding path has gone, rather than a problem with the file.
func sourceUnavailable(path string, err error) bool {
	if err == nil || errors.Is(err, io.EOF) {
		return false
	}
	if isDeviceGoneError(err) {
		return true
	}
	// "Path not found" style errors only count when the drive itself has gone.
	if errors.Is(err, os.ErrNotExist) {
		if root := driveRoot(path); root != "" {
			if _, serr := sourceStat(root); serr != nil {
				return true
			}
		}
	}
	return false
}

// driveRoot returns "X:\" for a drive-letter path and "" otherwise (UNC and
// shadow-copy paths, Unix paths), where the drive root check does not apply.
func driveRoot(path string) string {
	v := filepath.VolumeName(path)
	if len(v) == 2 && v[1] == ':' {
		return v + string(filepath.Separator)
	}
	return ""
}

// retrySource handles a failed operation on path. If the failure does not
// point at the device it returns firstErr straight away. Otherwise it waits
// for the source to come back and retries op, returning nil once op succeeds,
// op's last error when the source is back but op keeps failing (the file is
// the problem), or a *SourceLostError when the source never came back.
func (a *PXARArchive) retrySource(path string, firstErr error, what string, op func() error) error {
	if !sourceUnavailable(path, firstErr) {
		return firstErr
	}
	started := sourceNow()
	deadline := started.Add(sourceWaitWindow)
	a.notice(fmt.Sprintf("Source became unavailable while %s %s (%v), waiting up to %s for it to come back", what, path, firstErr, sourceWaitWindow))
	dir := filepath.Dir(path)
	lastErr := firstErr
	reachableFails := 0
	for attempt := 0; ; attempt++ {
		delay := sourceRetryDelays[len(sourceRetryDelays)-1]
		if attempt < len(sourceRetryDelays) {
			delay = sourceRetryDelays[attempt]
		}
		if err := sourceSleep(a.Ctx, delay); err != nil {
			return err
		}
		if _, err := sourceStat(dir); err != nil {
			if sourceNow().After(deadline) {
				return &SourceLostError{Path: path, Err: firstErr, Window: sourceWaitWindow}
			}
			continue
		}
		err := op()
		if err == nil {
			a.notice(fmt.Sprintf("Source back after %s, continued %s %s", sourceNow().Sub(started).Round(time.Second), what, path))
			return nil
		}
		lastErr = err
		reachableFails++
		if reachableFails >= sourceReachableAttempts || sourceNow().After(deadline) {
			return lastErr
		}
	}
}

// walkMustStop reports whether err, returned by retrySource, has to end the
// backup: the source never came back, or the job was stopped while waiting.
func (a *PXARArchive) walkMustStop(err error) bool {
	if err == nil {
		return false
	}
	var sl *SourceLostError
	return errors.As(err, &sl) || (a.Ctx != nil && a.Ctx.Err() != nil)
}
