package main

import (
	"context"
	"sync"

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

	// currentScheduledJobPostActions carries a running scheduled job's own
	// post-backup-action settings (email/run-app/exit/shutdown, see
	// ScheduledJob) across to startBackupDirect/startMachineBackupDirect's
	// OnComplete closure (main.go), which is where the REAL outcome is known
	// in standalone/GUI mode — executeScheduledJob's own call to
	// StartBackup/StartMachineBackup is fire-and-forget there (see
	// currentScheduledJobName's doc comment for the same reasoning, including
	// why the clear happens at each consumption point rather than via a
	// defer right after the setter; safe as a plain field for the identical
	// reason: operation_queue.go already serializes to one backup at a time
	// process-wide). nil means "not triggered by a scheduled job" (the
	// normal one-off case) — OnComplete must check for nil before using it.
	currentScheduledJobPostActionsMu sync.Mutex
	currentScheduledJobPostActions   *ScheduledJob

	// currentScheduledJobTrigger carries how the in-flight run was started
	// ("scheduled"/"startup"/"manual") across to OnComplete (main.go), for
	// the same reason and with the same lifecycle as currentScheduledJobName
	// (set in executeScheduledJob, cleared at whichever of its three
	// genuinely-final points applies — see that field's doc comment). Empty
	// means "not triggered by a scheduled job" (a genuine one-off backup),
	// which OnComplete records as "oneoff" rather than leaving blank.
	currentScheduledJobTriggerMu sync.Mutex
	currentScheduledJobTrigger   string
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

// setScheduledJobPostActions/currentPostActionsJob are the accessors for
// currentScheduledJobPostActions — see its doc comment on the App struct.
func (a *App) setScheduledJobPostActions(job *ScheduledJob) {
	a.currentScheduledJobPostActionsMu.Lock()
	a.currentScheduledJobPostActions = job
	a.currentScheduledJobPostActionsMu.Unlock()
}

func (a *App) currentPostActionsJob() *ScheduledJob {
	a.currentScheduledJobPostActionsMu.Lock()
	defer a.currentScheduledJobPostActionsMu.Unlock()
	return a.currentScheduledJobPostActions
}

// setScheduledJobTrigger/triggerOr are the accessors for
// currentScheduledJobTrigger — see its doc comment on the App struct.
func (a *App) setScheduledJobTrigger(trigger string) {
	a.currentScheduledJobTriggerMu.Lock()
	a.currentScheduledJobTrigger = trigger
	a.currentScheduledJobTriggerMu.Unlock()
}

// triggerOr returns the current run's trigger, or fallback if this
// backup/restore wasn't triggered by a scheduled job (the normal one-off
// case).
func (a *App) triggerOr(fallback string) string {
	a.currentScheduledJobTriggerMu.Lock()
	defer a.currentScheduledJobTriggerMu.Unlock()
	if a.currentScheduledJobTrigger != "" {
		return a.currentScheduledJobTrigger
	}
	return fallback
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
