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

	// Command-line Undelete / Roll back (set_cli.go): no window, no toasts;
	// progress lines and the result go to the console.
	headless    bool
	cliProgress func(pct float64, msg string)
	cliDone     chan setOpResult

	// currentScheduledJobName used to live here: the name of the set being
	// run, read back when a run had no name of its own. It was only safe
	// while one backup ran at a time; with several at once a one-off run
	// could take another set's name. Removed 2026-10-31: the scheduler
	// passes the set's name as the run's comment, and a one-off without one
	// is "Manual backup - <id>" / "Backup machine - <id>".

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

	// runReporter is the service's API server, in the service process only:
	// each run reports its progress and outcome to its entry there, so a GUI
	// can show and control it (service.go).
	runReporter serviceRunReporter
}

// serviceRunReporter is the part of api.Server a service run reports to.
type serviceRunReporter interface {
	BeginRun(name string) string
	ReportProgress(jobID string, percent float64, message string)
	ReportStats(jobID string, stats map[string]interface{})
	ReportFile(jobID string, path string)
	FinishRun(jobID string, success bool, message string)
}

// pendingPostActionsEntry is what pendingPostActions actually stores — see
// its doc comment on the App struct above.
type pendingPostActionsEntry struct {
	job     *ScheduledJob
	trigger string
	// result is the run's structured outcome, set by the service build's
	// OnResult so the scheduler's history write can include it.
	result atomic.Pointer[BackupStatus]
	// runID is the run's entry on the service's API (service process only).
	runID string
	// noter can add a note to the snapshot the run made (set by the
	// backup's OnSnapshot), for the post-backup actions.
	noter atomic.Pointer[noterBox]
	// serviceDone receives the outcome of a run the app handed to the
	// background service (startBackupViaService), so the scheduler can wait
	// for it before writing history and running the post-backup actions.
	serviceDone chan serviceRunOutcome
	// runDone is closed when a run this app does itself (startBackupDirect,
	// a goroutine) has ended, so the scheduler can hold the set as running
	// until then: the set is not started again while it runs or queues.
	runDone chan struct{}
}

// serviceRunOutcome is how a run handed to the service ended.
type serviceRunOutcome struct {
	success bool
	message string
}

// watchServiceRun gives the run under key a channel for its service outcome
// (nil for a one-off run, which has no entry and nobody waiting).
func (a *App) watchServiceRun(key string) chan serviceRunOutcome {
	if key == "" {
		return nil
	}
	v, ok := a.pendingPostActions.Load(key)
	if !ok {
		return nil
	}
	ch := make(chan serviceRunOutcome, 1)
	v.(*pendingPostActionsEntry).serviceDone = ch
	return ch
}

// serviceRunDone returns the channel watchServiceRun gave the run under key.
func (a *App) serviceRunDone(key string) chan serviceRunOutcome {
	if key == "" {
		return nil
	}
	if v, ok := a.pendingPostActions.Load(key); ok {
		return v.(*pendingPostActionsEntry).serviceDone
	}
	return nil
}

// snapshotNoter adds a note to the PBS comment of a finished snapshot.
type snapshotNoter interface {
	AddNote(note string) error
}

type noterBox struct{ n snapshotNoter }

// setRunNoter records the snapshot the run under key made (BackupOptions.OnSnapshot).
func (a *App) setRunNoter(key string, n snapshotNoter) {
	if key == "" || n == nil {
		return
	}
	if v, ok := a.pendingPostActions.Load(key); ok {
		v.(*pendingPostActionsEntry).noter.Store(&noterBox{n})
	}
}

// noter returns the entry's snapshot noter, or nil.
func (e *pendingPostActionsEntry) snapshotNoter() snapshotNoter {
	if e == nil {
		return nil
	}
	if b := e.noter.Load(); b != nil {
		return b.n
	}
	return nil
}

// runNoter returns the snapshot noter of the run under key without taking
// the entry.
func (a *App) runNoter(key string) snapshotNoter {
	if key == "" {
		return nil
	}
	if v, ok := a.pendingPostActions.Load(key); ok {
		return v.(*pendingPostActionsEntry).snapshotNoter()
	}
	return nil
}

// TrackRequestRun registers a run the GUI asked the service for, under its
// API entry jobID (api.RunTracker). Like RegisterRunOptions it carries the
// run's change detection and forensic copy; the backup's own completion removes it.
func (a *App) TrackRequestRun(jobID, backupType, changeDetectionMode string, readFreeSpace bool) string {
	key := a.RegisterRunOptions(backupType, changeDetectionMode, readFreeSpace)
	if v, ok := a.pendingPostActions.Load(key); ok {
		v.(*pendingPostActionsEntry).runID = jobID
	}
	return key
}

// finishRun records a service run's outcome on its API entry.
func (a *App) finishRun(runID string, success bool, message string) {
	if runID != "" && a.runReporter != nil {
		a.runReporter.FinishRun(runID, success, message)
	}
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
	a.pendingPostActions.Store(key, &pendingPostActionsEntry{job: &job, trigger: trigger, runDone: make(chan struct{})})
	return key
}

// localRunDone returns the channel closed when the run under key, done in
// this app, has ended (nil for a one-off run or an unknown key).
func (a *App) localRunDone(key string) chan struct{} {
	if key == "" {
		return nil
	}
	if v, ok := a.pendingPostActions.Load(key); ok {
		return v.(*pendingPostActionsEntry).runDone
	}
	return nil
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
	a := &App{
		config:        LoadConfig(),
		stopScheduler: make(chan struct{}),
		apiClient:     api.NewClient(getAPITokenPath()),
		callbacksMap:  make(map[string]*progressCallbacks),
	}
	a.useRunLimitFromConfig()
	return a
}

// NewAppForService creates an App instance for Windows Service (no Wails runtime)
func NewAppForService(ctx context.Context) *App {
	a := &App{
		ctx:              ctx,
		config:           LoadConfig(),
		stopScheduler:    make(chan struct{}),
		apiClient:        api.NewClient(getAPITokenPath()),
		mode:             api.ModeStandalone, // Service executes directly
		callbacksMap:     make(map[string]*progressCallbacks),
		isServiceProcess: true, // Prevent mode re-detection
	}
	a.useRunLimitFromConfig()
	return a
}

// useRunLimitFromConfig makes the queue (operation_queue.go) read how many
// backups may run at once from this app's settings, as they are at the time
// (the service reloads them before each run).
func (a *App) useRunLimitFromConfig() {
	maxConcurrentRuns = func() int {
		if c := a.config; c != nil {
			return clampRunLimit(c.MaxConcurrentRuns)
		}
		return DefaultMaxConcurrentRuns
	}
}
