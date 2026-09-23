package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"time"
)

// This watchdog exists because of a real, twice-seen bug (2026-09-22 and
// 2026-09-23): a restore's chunk fetch can wedge on a single stuck HTTP/2
// stream — the TCP connection stays open, the PBS server's own task list
// still shows it "running", and neither the per-chunk 60s context timeout
// (see GetChunkData's doc comment) nor an explicit user Cancel actually
// unwinds it. Guessing at the root cause a third time from source reading
// alone hasn't worked, so instead of that: if a restore goes quiet for too
// long, dump every goroutine's stack to disk automatically, before anyone
// has to remember to do it (or kills the process and loses the evidence).

const (
	restoreStallThreshold     = 90 * time.Second
	restoreStallCheckInterval = 15 * time.Second
)

var (
	restoreActive          atomic.Bool
	lastRestoreProgressAt  atomic.Int64 // UnixNano; 0 = no restore has reported progress yet
	restoreStallDumpWriten atomic.Bool  // avoid re-dumping every tick once a stall is already captured
)

// markRestoreStarted flags a restore as in-flight and resets the stall clock.
// Call once, right before launching the restore goroutine.
func markRestoreStarted() {
	restoreStallDumpWriten.Store(false)
	lastRestoreProgressAt.Store(time.Now().UnixNano())
	restoreActive.Store(true)
}

// markRestoreProgress resets the stall clock. Call from every progress/stats
// callback so genuine activity (even slow activity) never triggers a dump.
func markRestoreProgress() {
	lastRestoreProgressAt.Store(time.Now().UnixNano())
}

// markRestoreDone clears the in-flight flag once RestoreSnapshotInline
// returns (success, failure, or cancellation — any way it can end).
func markRestoreDone() {
	restoreActive.Store(false)
}

// startStallWatchdog runs for the lifetime of the process (both the GUI and
// the Windows service binary embed this file) and checks in periodically on
// whichever restore is currently marked active.
func startStallWatchdog() {
	go func() {
		ticker := time.NewTicker(restoreStallCheckInterval)
		defer ticker.Stop()
		for range ticker.C {
			if !restoreActive.Load() {
				continue
			}
			last := lastRestoreProgressAt.Load()
			if last == 0 {
				continue
			}
			idle := time.Since(time.Unix(0, last))
			if idle < restoreStallThreshold {
				continue
			}
			if restoreStallDumpWriten.Swap(true) {
				continue // already captured this stall episode
			}
			reason := fmt.Sprintf("restore: no progress for %s (threshold %s)", idle.Round(time.Second), restoreStallThreshold)
			path, err := dumpGoroutines(reason)
			if err != nil {
				writeDebugLog(fmt.Sprintf("STALL WATCHDOG: %s — failed to write goroutine dump: %v", reason, err))
				continue
			}
			writeDebugLog(fmt.Sprintf("STALL WATCHDOG: %s — goroutine dump written to %s", reason, path))
		}
	}()
}

// dumpGoroutines writes every goroutine's current stack trace to a timestamped
// file in the log directory (same format a SIGQUIT/os signal dump would
// produce) and returns its path.
func dumpGoroutines(reason string) (string, error) {
	name := fmt.Sprintf("stall-dump-%s.txt", time.Now().Format("20060102-150405"))
	path := filepath.Join(logDir, name)

	f, err := os.Create(path) // #nosec G304 -- logDir is our own fixed ProgramData path, name is a timestamp we generated
	if err != nil {
		return "", err
	}
	defer f.Close()

	fmt.Fprintf(f, "=== Goroutine dump: %s ===\nReason: %s\n\n", time.Now().Format(time.RFC3339), reason)

	buf := make([]byte, 1<<20) // 1MB; grown below if the real dump doesn't fit
	for {
		n := runtime.Stack(buf, true)
		if n < len(buf) {
			buf = buf[:n]
			break
		}
		buf = make([]byte, 2*len(buf))
	}
	if _, err := f.Write(buf); err != nil {
		return "", err
	}
	return path, nil
}
