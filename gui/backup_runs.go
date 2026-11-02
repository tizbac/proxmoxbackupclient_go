package main

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"pbscommon"
)

// backupRun is one backup in this process: what its Stop cancels, its own
// Pause gate and its own log file. Several can run at once now
// (operation_queue.go), so none of this may be process-wide: one global
// cancel and one global pause meant starting a second backup stopped the
// first, and Pause or Stop hit whichever had started last.
type backupRun struct {
	id      string // "run-N": how the window and the tray name it
	name    string // the Backup Set's name, or the one-off's
	kind    string // "directory" or "machine"
	backup  string // its backup-id, so the window can match a split part
	apiID   string // its entry on the service's API (service process only)
	started time.Time

	ctx    context.Context
	cancel context.CancelFunc
	pause  *pbscommon.PauseGate
	queued atomic.Bool // still waiting for its turn
	logger atomic.Pointer[RotatingLogger]
}

// logCtx is a context for writing to the run's log file from outside the
// backup (Pause and Stop say so in it).
func (r *backupRun) logCtx() context.Context {
	return withRun(context.Background(), r, r.logger.Load())
}

var (
	runsMu   sync.Mutex
	runsByID = map[string]*backupRun{}
	runSeq   atomic.Uint64
)

// newBackupRun registers a run, before it queues, so Stop works on it while it
// waits. Call end when it is over.
func newBackupRun(name, kind, backupID string) *backupRun {
	ctx, cancel := context.WithCancel(context.Background())
	r := &backupRun{
		id:      fmt.Sprintf("run-%d", runSeq.Add(1)),
		name:    name,
		kind:    kind,
		backup:  backupID,
		started: time.Now(),
		ctx:     ctx,
		cancel:  cancel,
		pause:   &pbscommon.PauseGate{},
	}
	runsMu.Lock()
	runsByID[r.id] = r
	runsMu.Unlock()
	return r
}

// end takes the run off the list. Its context is cancelled, which ends
// anything of the run's still waiting on it.
func (r *backupRun) end() {
	if r == nil {
		return
	}
	runsMu.Lock()
	delete(runsByID, r.id)
	runsMu.Unlock()
	r.pause.Resume()
	r.cancel()
}

// backupRunByID returns the run with the window's id, or the one reporting to
// the service API entry apiID; nil if neither is running.
func backupRunByID(id string) *backupRun {
	runsMu.Lock()
	defer runsMu.Unlock()
	if r, ok := runsByID[id]; ok {
		return r
	}
	for _, r := range runsByID {
		if r.apiID != "" && r.apiID == id {
			return r
		}
	}
	return nil
}

// activeBackupRuns lists the runs in this process, oldest first.
func activeBackupRuns() []*backupRun {
	runsMu.Lock()
	out := make([]*backupRun, 0, len(runsByID))
	for _, r := range runsByID {
		out = append(out, r)
	}
	runsMu.Unlock()
	sort.Slice(out, func(i, j int) bool { return out[i].started.Before(out[j].started) })
	return out
}

// localBackupRunning reports whether this process runs (or queues) a backup.
func localBackupRunning() bool {
	runsMu.Lock()
	defer runsMu.Unlock()
	return len(runsByID) > 0
}

// runsFor returns the run named by id, or every run when id is "".
func runsFor(id string) []*backupRun {
	if id == "" {
		return activeBackupRuns()
	}
	if r := backupRunByID(id); r != nil {
		return []*backupRun{r}
	}
	return nil
}

// runCtxKey carries a run and its log file in the run's context, so the
// backup code (and every goroutine it starts with that context) logs to the
// right file and waits on the right Pause.
type runCtxKey struct{}

type runCtxValue struct {
	run *backupRun
	log *RotatingLogger
}

// withRun returns ctx carrying run and its log file.
func withRun(ctx context.Context, run *backupRun, log *RotatingLogger) context.Context {
	return context.WithValue(ctx, runCtxKey{}, runCtxValue{run: run, log: log})
}

func runFromCtx(ctx context.Context) (runCtxValue, bool) {
	if ctx == nil {
		return runCtxValue{}, false
	}
	v, ok := ctx.Value(runCtxKey{}).(runCtxValue)
	return v, ok
}

// runLog writes a line to the log file of the run ctx belongs to, or to the
// shared backup log outside a run.
func runLog(ctx context.Context, message string) {
	if v, ok := runFromCtx(ctx); ok && v.log != nil {
		writeLogToLogger(v.log, "BACKUP", message)
		return
	}
	writeBackupLog(message)
}

// neverPaused is the gate for backup code running outside a registered run
// (tests, the command line): nothing pauses it.
var neverPaused = &pbscommon.PauseGate{}

// runPause returns the Pause gate of the run ctx belongs to.
func runPause(ctx context.Context) *pbscommon.PauseGate {
	if v, ok := runFromCtx(ctx); ok && v.run != nil {
		return v.run.pause
	}
	return neverPaused
}

// refreshRunningTooltip shows in the tray what is running ("Backup running:
// Data and Music"), or the next scheduled run when nothing is.
func (a *App) refreshRunningTooltip() {
	if label := currentOperationLabel(); label != "" {
		a.UpdateTrayTooltip("Backup running: " + label)
		return
	}
	a.RefreshIdleTooltip()
}
