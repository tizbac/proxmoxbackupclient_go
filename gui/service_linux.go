//go:build linux && service
// +build linux,service

package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tizbac/proxmoxbackupclient_go/gui/api"
	"pbscommon"
)

// BackupService wraps the application for Linux systemd service execution
type BackupService struct {
	app       *App
	apiServer *api.Server
	stopChan  chan struct{}
}

// Start is called when the service starts (systemd calls this via RunAsService)
func (s *BackupService) Start() error {
	writeDebugLog("Proxmox Backup Client service starting...")
	s.stopChan = make(chan struct{})
	go s.run()
	return nil
}

// run contains the main service loop
func (s *BackupService) run() {
	writeDebugLog("Proxmox Backup Client service running")

	// Initialize app with background context (service has no Wails runtime)
	// Service App must be in Standalone mode to execute backups directly
	s.app = &App{
		ctx:              context.Background(),
		config:           LoadConfig(),
		stopScheduler:    make(chan struct{}),
		apiClient:        api.NewClient(getAPITokenPath()),
		mode:             api.ModeStandalone, // Service executes directly, doesn't use API
		callbacksMap:     make(map[string]*progressCallbacks),
		isServiceProcess: true, // Prevent mode re-detection (would cause infinite loop)
	}

	// Load configuration
	configMap := s.app.GetConfigWithHostname()
	if hostname, ok := configMap["hostname"].(string); ok {
		writeDebugLog(fmt.Sprintf("Service: Running for %s", hostname))
	} else {
		writeDebugLog("Service: Running in background")
	}

	// Clean up any abandoned jobs from previous crash
	s.app.CleanupAbandonedJobs()

	// Recalculate stale nextRun values (e.g. after service restart or missed window)
	s.app.RecalculateNextRuns()

	// Start the scheduler
	s.app.StartScheduler()

	// Start HTTP API server for GUI communication (token-authenticated)
	apiToken, tokErr := api.EnsureToken(getAPITokenPath())
	if tokErr != nil {
		writeDebugLog(fmt.Sprintf("API token init failed (API will reject all requests): %v", tokErr))
	}
	s.apiServer = api.NewServer("127.0.0.1:18765", s.app, apiToken, appVersion)
	writeDebugLog("Starting HTTP API server on 127.0.0.1:18765")

	go func() {
		if err := s.apiServer.Start(); err != nil {
			writeDebugLog(fmt.Sprintf("API server error: %v", err))
		}
	}()

	// Wait for SIGTERM or SIGINT
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	// Keep the service running (scheduler and API server run in background goroutines)
	writeDebugLog("Service main loop started, waiting for stop signal")
	<-sigChan // Block until stop signal received
	writeDebugLog("Stop signal received, service main loop exiting")

	// Signal the main loop to exit
	close(s.stopChan)
}

// Stop is called when the service stops
func (s *BackupService) Stop() error {
	writeDebugLog("Proxmox Backup Client service stopping...")

	// Close any live PBS backup session
	pbscommon.CloseAllActive()

	// Stop the scheduler gracefully
	if s.app != nil {
		s.app.StopScheduler()
	}

	// Give it a moment to finish current operations
	time.Sleep(2 * time.Second)

	writeDebugLog("Proxmox Backup Client service stopped")
	return nil
}

// RunAsService starts the application as a Linux systemd service
func RunAsService() {
	writeDebugLog("Running as Linux systemd service")

	svc := &BackupService{}

	if err := svc.Start(); err != nil {
		log.Fatal(err)
	}

	// Block until stop signal is received (handled in svc.run())
	<-svc.stopChan

	if err := svc.Stop(); err != nil {
		log.Fatal(err)
	}
}