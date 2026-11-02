package main

import (
	"context"
	"fmt"
	"os"
	"runtime/debug"
	"time"

	"github.com/kardianos/service"
	"github.com/tizbac/proxmoxbackupclient_go/gui/api"
	"pbscommon"
	"snapshot"
)

// BackupService wraps the application for the background service: a Windows
// service running as Local System, or a systemd service running as root.
type BackupService struct {
	app          *App
	apiServer    *api.Server
	tokenRotator *api.TokenRotator
	stopChan     chan struct{}
}

// Start is called when the service starts
func (s *BackupService) Start(svc service.Service) error {
	writeDebugLog("Proxmox Backup Client service starting...")
	s.stopChan = make(chan struct{})
	go func() {
		// A panic in the run loop kills the process, and the SCM (or systemd)
		// only reports that the service stopped unexpectedly, with nothing in
		// the log to explain it. Log the panic and its stack first, then exit
		// so the failure is a real, diagnosable one rather than a "running"
		// service that does nothing (upstream's service does the same).
		defer func() {
			if r := recover(); r != nil {
				writeDebugLog(fmt.Sprintf("CRITICAL: service run panic: %v\n%s", r, debug.Stack()))
				os.Exit(1)
			}
		}()
		s.run()
	}()
	return nil
}

// run contains the main service loop
func (s *BackupService) run() {
	writeDebugLog("Proxmox Backup Client service running")

	// Initialize app with background context (service has no Wails runtime)
	// IMPORTANT: Service App must be in Standalone mode to execute backups directly
	s.app = &App{
		ctx:              context.Background(),
		config:           LoadConfig(),
		stopScheduler:    make(chan struct{}),
		apiClient:        api.NewClient(getAPITokenPath()),
		mode:             api.ModeStandalone, // Service executes directly, doesn't use API
		callbacksMap:     make(map[string]*progressCallbacks),
		isServiceProcess: true, // Prevent mode re-detection (would cause infinite loop)
	}

	// Load configuration (service will read config from file when needed)
	configMap := s.app.GetConfigWithHostname()
	if hostname, ok := configMap["hostname"].(string); ok {
		writeDebugLog(fmt.Sprintf("Service: Running for %s", hostname))
	} else {
		writeDebugLog("Service: Running in background")
	}

	// Config will be loaded from file by each scheduled job when needed

	// Clean up any abandoned jobs from previous crash
	s.app.CleanupAbandonedJobs()

	// Clear any orphaned VSS shadow copies and reset the VSS service state
	// from a previously crashed backup process. Without this, the next backup
	// can fail with "VSS_START - shadow copy creation is already in progress".
	if err := snapshot.VSSCleanup(); err != nil {
		writeDebugLog(fmt.Sprintf("VSS cleanup at startup reported error: %v", err))
	}

	// Lock the settings folder down before writing the API token into it.
	if dir, err := getConfigDir(); err == nil {
		if err := hardenConfigDir(dir); err != nil {
			writeDebugLog(fmt.Sprintf("[ConfigACL] %v", err))
		}
	}

	// Start HTTP API server for GUI communication (token-authenticated — H-01).
	// If token init fails the server keeps an empty token and rejects every
	// request (fail closed) rather than exposing the privileged API unauthenticated.
	// The token file is the service's alone (secureTokenFile): an app gets
	// the token through UAC or pkexec (upstream's strict model).
	api.SecureTokenFile = secureTokenFile
	// A new token at every start, never one found on disk: a token file
	// planted before the first install, or read under an earlier build's
	// wider permissions, stops working here.
	apiToken, tokErr := api.NewToken(getAPITokenPath())
	if tokErr != nil {
		writeDebugLog(fmt.Sprintf("API token init failed (API will reject all requests): %v", tokErr))
	}
	s.apiServer = api.NewServer("127.0.0.1:18765", s.app, apiToken, appVersion)
	// Every run reports to its entry on the API, scheduled ones included.
	s.app.runReporter = s.apiServer
	// It is also replaced every 24 hours (from upstream; PBSGO_TOKEN_ROTATION
	// and PBSGO_TOKEN_GRACE change it, 0 turns it off).
	if tokErr == nil {
		interval, grace := api.TokenRotationFromEnv()
		s.tokenRotator = api.NewTokenRotator(s.apiServer, getAPITokenPath(), interval, grace)
		s.tokenRotator.Start()
	}
	writeDebugLog("Starting HTTP API server on 127.0.0.1:18765")

	go func() {
		if err := s.apiServer.Start(); err != nil {
			writeDebugLog(fmt.Sprintf("API server error: %v", err))
		}
	}()

	// The scheduler starts last, so a run at startup already has the API to
	// report to.
	// Recalculate stale nextRun values (e.g. after service restart or missed window)
	s.app.RecalculateNextRuns()
	s.app.StartScheduler()
	// Sets with "Run at startup" run when the service starts: an app started
	// at sign-in is not connected to the service until a window asks, so it
	// can no longer start them itself.
	go s.app.HandleStartupRun()

	// Keep the service running (scheduler and API server run in background goroutines)
	writeDebugLog("Service main loop started, waiting for stop signal")
	<-s.stopChan // Block until stop signal received
	writeDebugLog("Stop signal received, service main loop exiting")
}

// Stop is called when the service stops
func (s *BackupService) Stop(svc service.Service) error {
	writeDebugLog("Proxmox Backup Client service stopping...")

	// Close any live PBS backup session before we return, so the server
	// releases the writer / snapshot lock instead of waiting for TCP
	// keepalive to reap the abandoned connection.
	pbscommon.CloseAllActive()

	// Signal the main loop to exit
	if s.stopChan != nil {
		close(s.stopChan)
	}

	// Stop the scheduler gracefully
	if s.app != nil {
		s.app.StopScheduler()
	}
	if s.tokenRotator != nil {
		s.tokenRotator.Stop()
	}

	// Give it a moment to finish current operations
	time.Sleep(2 * time.Second)

	writeDebugLog("Proxmox Backup Client service stopped")
	return nil
}
