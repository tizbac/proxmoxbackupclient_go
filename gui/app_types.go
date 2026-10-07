package main

import (
	"context"
	"sync"

	"github.com/tizbac/proxmoxbackupclient_go/gui/api"
)

// App struct contains the application state
type App struct {
	ctx           context.Context
	config        *Config
	stopScheduler chan struct{}
	apiClient     *api.Client
	mode          api.ExecutionMode
	// standaloneReason records WHY the GUI is in standalone mode:
	// "forced" (--standalone flag), "no_service" (service unreachable),
	// "auth_failed" (service running but token could not be acquired), ""
	// when running in service mode. Surfaced to the frontend notice.
	standaloneReason string
	callbacksMap     map[string]*progressCallbacks
	callbacksMutex   sync.RWMutex
	isServiceProcess bool // True if running as Windows Service (never re-detect mode)
	// scheduledBackupFn, when set, replaces the backup a scheduled job runs.
	// It exists for tests: the scheduler bookkeeping (history, lastRun/nextRun,
	// duplicate guard, run-now) must be testable without a real PBS server.
	// Production leaves it nil and routes through StartBackup.
	scheduledBackupFn func(job ScheduledJob) error

	// cancelFuncs tracks cancellation functions for running backup jobs.
	// Key is jobID, value is the cancel function to call for graceful stop.
	cancelFuncs     map[string]context.CancelFunc
	cancelFuncsMu   sync.Mutex

	// backupCtx is the context for the currently running backup (set by API handler
	// for service mode to enable cancellation). Protected by backupCtxMu.
	backupCtx     context.Context
	backupCancel  context.CancelFunc
	backupCtxMu   sync.Mutex
}

// isDelegatedToService reports whether this (non-service) GUI process is
// running in service mode with a live API client: config reads/writes, PBS
// tests and ticketing must all go through the service, which is the sole
// owner of the privileged config files.
func (a *App) isDelegatedToService() bool {
	return !a.isServiceProcess && a.mode == api.ModeService && a.apiClient != nil
}

// progressCallbacks stores the callback functions for a backup operation
type progressCallbacks struct {
	onProgress func(jobID string, percent float64, message string)
	onComplete func(jobID string, success bool, message string)
}

// PhysicalDiskInfo represents information about a physical disk. It lives in
// the shared app types (not main.go) because the platform disk-listing files
// (disklist_linux.go, disklist_windows.go) are compiled into the service build
// as well.
type PhysicalDiskInfo struct {
	DiskNumber   int64  `json:"disk_number"`
	Size         int64  `json:"size"`
	Model        string `json:"model"`
	IsBootDisk   bool   `json:"is_boot_disk"`
	IsSystemDisk bool   `json:"is_system_disk"`
	DeviceID     string `json:"device_id"`
	DevicePath   string `json:"device_path"`
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
