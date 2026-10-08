//go:build service && !windows
// +build service,!windows

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"pbscommon"

	"github.com/kardianos/service"
	"github.com/tizbac/proxmoxbackupclient_go/gui/api"
)

// BackupService runs the scheduler and the local API server under systemd on
// Linux. It mirrors the Windows BackupService (service.go) minus VSS, which
// is a Windows-only concern. The service runs as root and owns the privileged
// state directory: config.json, scheduled_jobs.json, job_history.json and
// api-token all live there with 0600/0700 permissions, so only the service
// (root) can read or write them.
type BackupService struct {
	app          *App
	apiServer    *api.Server
	tokenRotator *api.TokenRotator
	stopChan     chan struct{}
	stopOnce     sync.Once
}

// Start is called when the service starts
func (s *BackupService) Start(svc service.Service) error {
	writeDebugLog("pbsgo service starting...")
	s.stopChan = make(chan struct{})
	go s.run()
	return nil
}

// run contains the main service loop
func (s *BackupService) run() {
	writeDebugLog("pbsgo service running")

	// The service owns the privileged state directory. systemd creates
	// /var/lib/pbsgo 0700 (StateDirectory=pbsgo + StateDirectoryMode=0700);
	// we enforce the same mode for a manual run and pin every
	// config/scheduler/cache path inside it BEFORE loading the config.
	stateDir := serviceStateDir()
	SetConfigDir(stateDir)
	if err := os.MkdirAll(stateDir, 0700); err != nil {
		writeDebugLog(fmt.Sprintf("create state dir %s failed: %v", stateDir, err))
	}
	// #nosec G302 -- stateDir is a fixed system path, not user input
	_ = os.Chmod(stateDir, 0700)

	s.app = NewAppForService(context.Background())

	configMap := s.app.GetConfigWithHostname()
	if hostname, ok := configMap["hostname"].(string); ok {
		writeDebugLog(fmt.Sprintf("Service: Running for %s", hostname))
	} else {
		writeDebugLog("Service: Running in background")
	}

	// Clean up any abandoned jobs from previous crash
	s.app.CleanupAbandonedJobs()

	// Recalculate stale nextRun values (e.g. after restart or missed window)
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
	s.apiServer = api.NewServer("127.0.0.1:18765", s.app, apiToken, appVersion)
	writeDebugLog("Starting HTTP API server on 127.0.0.1:18765")

	// Regenerate the token on a schedule (default 24h, PBSGO_TOKEN_ROTATION to
	// change, =0 to disable): a secret that lives forever in user sessions
	// would outlive every legitimate reason to hold it. The server keeps
	// accepting the replaced token for a grace window (default 10m,
	// PBSGO_TOKEN_GRACE) so in-flight requests are not cut off; a GUI that is
	// idle past that window gets a 401 and re-runs the elevated fetch.
	interval, grace := api.TokenRotationFromEnv()
	s.tokenRotator = api.NewTokenRotator(s.apiServer, getAPITokenPath(), interval, grace)
	s.tokenRotator.Start()

	go func() {
		if err := s.apiServer.Start(); err != nil {
			writeDebugLog(fmt.Sprintf("API server error: %v", err))
		}
	}()

	// Keep the service running (scheduler and API server run in background
	// goroutines). systemd (Type=simple) signals SIGTERM on stop; kardianos
	// does not translate it, so we catch it here and run the same graceful
	// shutdown as Stop().
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sigChan
		writeDebugLog("Stop signal received, shutting down gracefully")
		s.shutdown()
	}()
	writeDebugLog("Service main loop started, waiting for stop signal")
	<-s.stopChan // Block until stop signal received
	writeDebugLog("Service main loop exiting")
}

// shutdown performs the graceful teardown exactly once: it is called both by
// the signal goroutine (SIGTERM/SIGINT) and by Stop() (service control).
func (s *BackupService) shutdown() {
	s.stopOnce.Do(func() {
		writeDebugLog("pbsgo service stopping...")

		// Close any live PBS backup session before we return, so the server
		// releases the writer / snapshot lock instead of waiting for TCP
		// keepalive to reap the abandoned connection.
		pbscommon.CloseAllActive()

		// Stop the scheduler gracefully
		if s.app != nil {
			s.app.StopScheduler()
		}

		// Stop rotating the API token (in-flight requests already got the
		// current one from the server, which is going away anyway).
		if s.tokenRotator != nil {
			s.tokenRotator.Stop()
		}

		if s.stopChan != nil {
			close(s.stopChan)
			s.stopChan = nil
		}

		// Give it a moment to finish current operations
		time.Sleep(2 * time.Second)

		writeDebugLog("pbsgo service stopped")
	})
}

// Stop is called when the service stops
func (s *BackupService) Stop(svc service.Service) error {
	s.shutdown()
	return nil
}
