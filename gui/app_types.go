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
	// track. Set immediately before calling StartBackup/StartMachineBackup
	// and cleared via defer right after, in executeScheduledJob only.
	currentScheduledJobNameMu sync.Mutex
	currentScheduledJobName   string
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
