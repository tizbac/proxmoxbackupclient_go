package main

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tizbac/proxmoxbackupclient_go/gui/api"
)

// App struct contains the application state
type App struct {
	ctx              context.Context
	config           *Config
	stopScheduler    chan struct{}
	apiClient        *api.Client
	mode             api.ExecutionMode
	callbacksMap     map[string]*progressCallbacks
	callbacksMutex   sync.RWMutex
	isServiceProcess bool // True if running as Windows Service (never re-detect mode)

	// currentScheduledJobName lets executeScheduledJob (scheduler.go) tell
	// startBackupDirect/startMachineBackupDirect's OnComplete history-writing
	// (main.go) which scheduled job's real name to use instead of the generic
	// "Manual backup - X"/"Backup machine - X" label, without changing the
	// Wails-exposed StartBackup/StartMachineBackup signatures (which the
	// frontend also calls directly for one-off backups, where this field
	// stays empty). Safe as a plain field, not a map keyed by job/request:
	// operation_queue.go already serializes every backup/restore process-wide
	// to one at a time, so there is never more than one in-flight value to
	// track.
	//
	// Set immediately before calling StartBackup/StartMachineBackup, but NOT
	// cleared via a defer right after that call: in standalone/GUI mode that
	// call is fire-and-forget (the real backup runs in a goroutine and
	// executeScheduledJob returns immediately), so a defer there would clear
	// the name before the backup even starts, let alone finishes — found
	// live 2026-09-26 when two named Backup Sets both showed up in Reports
	// under the generic label. Instead, whichever of the three genuinely
	// final points actually applies clears it: OnComplete (main.go, the
	// normal standalone-mode case, once the real outcome is known), or one
	// of executeScheduledJob's own two synchronous branches (service mode;
	// or standalone's immediate pre-goroutine failure, where OnComplete
	// never runs at all). Safe against a following run's set racing this
	// clear: operation_queue.go's slot release is deferred around the same
	// RunBackupInline call that invokes OnComplete synchronously, so the
	// next backup can't acquire the slot until this one's clear has already
	// happened.
	currentScheduledJobNameMu sync.Mutex
	currentScheduledJobName   string

	// currentScheduledJobPostActions/currentScheduledJobTrigger used to live
	// here as single shared fields, same as currentScheduledJobName above —
	// REMOVED 2026-09-28. Unlike the name (which genuinely needs a
	// shared-field fallback: a one-off backup has no post-actions/trigger
	// concept at all, so there's nothing to pass through for that case
	// anyway), these two were ONLY ever read by startBackupDirect/
	// startMachineBackupDirect's OnComplete closures (main.go). A single
	// shared field has a real race: RunScheduledJobNow fires each Run Now
	// click as its own unserialized goroutine, so goroutine A's set here
	// could be overwritten by goroutine B's own set before A's own
	// OnComplete (which runs much later, once the real backup actually
	// finishes) gets around to reading it back — misapplying job B's
	// shutdown/email/run-app settings to job A's actual outcome. Unlike the
	// name-only version of this bug (cosmetic — a wrong label in Reports), a
	// misattributed ShutdownAfter is a real, irreversible consequence.
	// Never actually observed live; found by auditing this field for the
	// same shape of bug immediately after fixing the name one.
	//
	// Can't just pass *ScheduledJob straight through as a new StartBackup/
	// StartMachineBackup parameter either (the obvious next idea): those
	// methods' signatures are constrained by api.BackupHandler (gui/api,
	// implemented by both this file's App and app_service_stubs.go's), and
	// package api cannot import ScheduledJob from package main without a
	// cycle. Fixed instead with pendingPostActions below: each
	// executeScheduledJob call generates its own unique key, so there is no
	// single value for concurrent goroutines to race on — the interface gap
	// (a plain string) stays exactly as small as comment's already is.
	pendingPostActions sync.Map // key string -> *pendingPostActionsEntry
}

// pendingPostActionsEntry is what pendingPostActions actually stores — see
// its doc comment on the App struct above.
type pendingPostActionsEntry struct {
	job     *ScheduledJob
	trigger string
	// result is the run's structured outcome, set by the service build's
	// OnResult so the scheduler's history write can include it.
	result atomic.Pointer[BackupStatus]
}

// setRunResult stores status for the run under key, if it is registered.
func (a *App) setRunResult(key string, status *BackupStatus) {
	if key == "" || status == nil {
		return
	}
	if v, ok := a.pendingPostActions.Load(key); ok {
		v.(*pendingPostActionsEntry).result.Store(status)
	}
}

// takeRunResult returns the stored outcome of the run under key (nil when
// none was stored). The entry itself stays for takePendingPostActions.
func (a *App) takeRunResult(key string) *BackupStatus {
	if key == "" {
		return nil
	}
	v, ok := a.pendingPostActions.Load(key)
	if !ok {
		return nil
	}
	return v.(*pendingPostActionsEntry).result.Load()
}

// registerPendingPostActions stores job/trigger under a fresh, unique key and
// returns it — see the App struct's pendingPostActions doc comment. Call
// immediately before StartBackup/StartMachineBackup; the returned key is what
// gets passed as their postActionsKey parameter.
func (a *App) registerPendingPostActions(job ScheduledJob, trigger string) string {
	key := fmt.Sprintf("%s-%d", job.ID, time.Now().UnixNano())
	a.pendingPostActions.Store(key, &pendingPostActionsEntry{job: &job, trigger: trigger})
	return key
}

// isUnattendedRun reports whether postActionsKey belongs to a run nobody is
// watching (a timer or startup trigger), as opposed to a Run Now click. Unlike
// takePendingPostActions it leaves the entry in place.
func (a *App) isUnattendedRun(key string) bool {
	if key == "" {
		return false
	}
	v, ok := a.pendingPostActions.Load(key)
	if !ok {
		return false
	}
	t := v.(*pendingPostActionsEntry).trigger
	return t == "scheduled" || t == "startup"
}

// takePendingPostActions looks up and removes a key's entry — nil if key is
// empty (the normal one-off case) or already consumed. Call exactly once, from
// the OnComplete closure that actually knows the real outcome.
func (a *App) takePendingPostActions(key string) *pendingPostActionsEntry {
	if key == "" {
		return nil
	}
	v, ok := a.pendingPostActions.LoadAndDelete(key)
	if !ok {
		return nil
	}
	return v.(*pendingPostActionsEntry)
}

// progressCallbacks stores the callback functions for a backup operation
type progressCallbacks struct {
	onProgress func(jobID string, percent float64, message string)
	onComplete func(jobID string, success bool, message string)
}

// setScheduledJobName/scheduledJobNameOr are the accessors for
// currentScheduledJobName — see its doc comment on the App struct.
func (a *App) setScheduledJobName(name string) {
	a.currentScheduledJobNameMu.Lock()
	a.currentScheduledJobName = name
	a.currentScheduledJobNameMu.Unlock()
}

// scheduledJobNameOr returns the current scheduled job's name, or fallback
// if this backup/restore wasn't triggered by a scheduled job (the normal
// one-off case).
func (a *App) scheduledJobNameOr(fallback string) string {
	a.currentScheduledJobNameMu.Lock()
	defer a.currentScheduledJobNameMu.Unlock()
	if a.currentScheduledJobName != "" {
		return a.currentScheduledJobName
	}
	return fallback
}

// GetLogsFolder returns the directory holding backup-gui.log/service-gui.log
// (and their rotated .gz siblings) — GetServiceLogPath/GetBackupLogPath give
// the individual files, but the frontend's "View Logs" button opens the
// containing folder via the same BrowserOpenURL/xdg-open path it already
// uses for external links, so a browsable directory is what it needs. Both
// build tags (logging_gui.go/logging_service.go) define GetServiceLogPath
// identically, so either resolves the same real path.
func (a *App) GetLogsFolder() string {
	return filepath.Dir(GetServiceLogPath())
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		config:        LoadConfig(),
		stopScheduler: make(chan struct{}),
		apiClient:     api.NewClient(getAPITokenPath()),
		callbacksMap:  make(map[string]*progressCallbacks),
	}
}

// NewAppForService creates an App instance for Windows Service (no Wails runtime)
func NewAppForService(ctx context.Context) *App {
	return &App{
		ctx:              ctx,
		config:           LoadConfig(),
		stopScheduler:    make(chan struct{}),
		apiClient:        api.NewClient(getAPITokenPath()),
		mode:             api.ModeStandalone, // Service executes directly
		callbacksMap:     make(map[string]*progressCallbacks),
		isServiceProcess: true, // Prevent mode re-detection
	}
}
