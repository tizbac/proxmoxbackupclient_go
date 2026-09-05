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
	// Active PBS session tickets (username/password login), keyed by server
	// BaseURL. Session-only: dropped on app exit and never written to disk.
	tickets   map[string]*pbsticket
	ticketsMu sync.RWMutex
}

// progressCallbacks stores the callback functions for a backup operation
type progressCallbacks struct {
	onProgress func(jobID string, percent float64, message string)
	onComplete func(jobID string, success bool, message string)
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{
		config:        LoadConfig(),
		stopScheduler: make(chan struct{}),
		apiClient:     api.NewClient(getAPITokenPath()),
		callbacksMap:  make(map[string]*progressCallbacks),
		tickets:       make(map[string]*pbsticket),
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
		tickets:          make(map[string]*pbsticket),
	}
}
