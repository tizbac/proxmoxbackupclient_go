//go:build windows
// +build windows

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/kardianos/service"
	"github.com/tizbac/proxmoxbackupclient_go/gui/api"
	"pbscommon"
	"snapshot"
)

// BackupService wraps the application for Windows Service execution
type BackupService struct {
	app       *App
	apiServer *api.Server
	stopChan  chan struct{}
}

// Start is called when the service starts
func (s *BackupService) Start(svc service.Service) error {
	writeDebugLog("Proxmox Backup Client service starting...")
	s.stopChan = make(chan struct{})
	go s.run()
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

	// Recalculate stale nextRun values (e.g. after service restart or missed window)
	s.app.RecalculateNextRuns()

	// Start the scheduler
	s.app.StartScheduler()

	// Start HTTP API server for GUI communication (token-authenticated — H-01).
	// If token init fails the server keeps an empty token and rejects every
	// request (fail closed) rather than exposing the privileged API unauthenticated.
	apiToken, tokErr := api.EnsureToken(getAPITokenPath())
	if tokErr != nil {
		writeDebugLog(fmt.Sprintf("API token init failed (API will reject all requests): %v", tokErr))
	}

	// Harden file ACLs: token readable by SYSTEM + Administrators; config files
	// SYSTEM-only. Best-effort — never fail service startup if icacls is missing.
	go hardenWindowsACLs(serviceStateDir())

	s.apiServer = api.NewServer("127.0.0.1:18765", s.app, apiToken, appVersion)
	writeDebugLog("Starting HTTP API server on 127.0.0.1:18765")

	go func() {
		if err := s.apiServer.Start(); err != nil {
			writeDebugLog(fmt.Sprintf("API server error: %v", err))
		}
	}()

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

	// Give it a moment to finish current operations
	time.Sleep(2 * time.Second)

	writeDebugLog("Proxmox Backup Client service stopped")
	return nil
}

// RunAsService starts the application as a Windows Service
func RunAsService() {
	writeDebugLog("Attempting to run as Windows Service")

	svcConfig := &service.Config{
		Name:        "ProxmoxBackupClient",
		DisplayName: "Proxmox Backup Client Service",
		Description: "Executes scheduled backups to Proxmox Backup Server with VSS support",
	}

	backupSvc := &BackupService{}
	s, err := service.New(backupSvc, svcConfig)
	if err != nil {
		log.Fatal(err)
	}

	logger, err := s.Logger(nil)
	if err != nil {
		log.Fatal(err)
	}

	err = s.Run()
	if err != nil {
		logger.Error(err)
	}
}

// IsServiceMode checks if running in service mode
func IsServiceMode() bool {
	for _, arg := range os.Args {
		if arg == "--service" {
			return true
		}
	}
	return false
}

// hardenWindowsACLs applies restrictive ACLs to the service state directory:
// - api-token: readable by SYSTEM and BUILTIN\Administrators (for elevated GUI fetch)
// - config.json, scheduled_jobs.json, job_history.json: SYSTEM-only
// Best-effort: if icacls.exe is missing or fails, we log and continue.
func hardenWindowsACLs(stateDir string) {
	icacls, err := exec.LookPath("icacls.exe")
	if err != nil {
		writeDebugLog("icacls.exe not found, skipping ACL hardening")
		return
	}

	tokenPath := filepath.Join(stateDir, "api-token")
	configFiles := []string{
		filepath.Join(stateDir, "config.json"),
		filepath.Join(stateDir, "scheduled_jobs.json"),
		filepath.Join(stateDir, "job_history.json"),
	}

	// api-token: SYSTEM:(OI)(CI)F + BUILTIN\Administrators:(OI)(CI)R
	// (OI)(CI) = Object Inherit + Container Inherit (for future files)
	if _, err := exec.Command(icacls, tokenPath,
		"/inheritance:r",          // remove inherited ACEs
		"/grant:r", "SYSTEM:(OI)(CI)F",
		"/grant:r", "BUILTIN\\Administrators:(OI)(CI)R",
	).CombinedOutput(); err != nil {
		writeDebugLog(fmt.Sprintf("icacls on api-token failed (non-fatal): %v", err))
	}

	// Config files: SYSTEM-only (F = full control)
	for _, f := range configFiles {
		if _, err := exec.Command(icacls, f,
			"/inheritance:r",
			"/grant:r", "SYSTEM:(OI)(CI)F",
		).CombinedOutput(); err != nil {
			writeDebugLog(fmt.Sprintf("icacls on %s failed (non-fatal): %v", f, err))
		}
	}
}
