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
	"runtime/debug"
	"strings"
	"time"

	"pbscommon"
	"snapshot"

	"github.com/kardianos/service"
	"github.com/tizbac/proxmoxbackupclient_go/gui/api"
)

// BackupService wraps the application for Windows Service execution
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
		// A panic in the run loop kills the process and the SCM reports the
		// service as "terminated unexpectedly" (error 1053/1067) with nothing
		// in the log to explain it. Recover, dump the stack to the service
		// log, and keep the process alive so the failure is diagnosable.
		defer func() {
			if r := recover(); r != nil {
				writeDebugLog(fmt.Sprintf("CRITICAL: service run panic: %v\n%s", r, debug.Stack()))
				// Exit so the SCM reports a real failure (instead of a
				// "running" service that does nothing); the stack above is
				// already flushed to service-service.log.
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

	// The service owns the privileged state directory on Windows too.
	// Pin config dir before any config/scheduler operations.
	SetConfigDir(serviceStateDir())

	// Initialize app with background context (service has no Wails runtime)
	// IMPORTANT: Service App must be in Standalone mode to execute backups directly

	s.app = NewAppForService(context.Background())

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

	// Hand the API server to the App as the progress sink for backups that do
	// NOT start through an HTTP handler — scheduled runs and "Run now". The
	// GUI's Running Jobs tab lists GET /backup/jobs, and without this the
	// service executed scheduled machine backups that never appeared anywhere
	// in the GUI.
	s.app.SetJobReporter(s.apiServer)

	// Periodic token regeneration (default 24h, PBSGO_TOKEN_ROTATION/GRACE to
	// tune, rotation=0 to disable): the replaced token keeps authenticating
	// for the grace window, and a GUI that outlives it gets a 401 and repeats
	// the elevated (UAC) fetch instead of failing forever.
	interval, grace := api.TokenRotationFromEnv()
	s.tokenRotator = api.NewTokenRotator(s.apiServer, getAPITokenPath(), interval, grace)
	s.tokenRotator.Start()

	go func() {
		if err := s.apiServer.Start(); err != nil {
			writeDebugLog(fmt.Sprintf("API server error: %v", err))
		}
	}()

	// Start the scheduler LAST, once the reporter above is installed: a job
	// flagged "run at startup" fires synchronously from here, and startup is
	// exactly when the user opens the GUI to watch it — by then its run must
	// already be visible in the jobs list.
	s.app.StartScheduler()

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

	// Stop rotating the API token
	if s.tokenRotator != nil {
		s.tokenRotator.Stop()
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
		Name:        windowsServiceName(),
		DisplayName: windowsServiceDisplayName(),
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

// hardenWindowsACLs applies restrictive DACLs to the service state directory:
//   - api-token: SYSTEM (F) + BUILTIN\Administrators (R) — a user session only
//     reads it through an elevated (UAC) fetch, see api.EnsureToken;
//   - config.json, scheduled_jobs.json, job_history.json: SYSTEM-only (every
//     GUI access goes through the service API).
//
// Two details are load-bearing, both learned from a production lockout:
//
//   - Trustees are referenced by SID (*S-1-5-18 = NT AUTHORITY\SYSTEM,
//     *S-1-5-32-544 = BUILTIN\Administrators), never by the English account
//     names: on a localized (non-English) Windows those names do not resolve,
//     the /grant:r fails — and with /inheritance:r in the same command line
//     the file was left with an EMPTY DACL. No user could read or write it,
//     not even SYSTEM itself: the owner keeps WRITE_DAC (to repair the DACL)
//     but gets no file access at all, which locked the GUI out of the token
//     and every JSON config.
//   - The grant runs in its own icacls invocation BEFORE the inheritance
//     strip, so a failure can only leave the file MORE permissive than
//     intended (the inherited ACL), never unreachable.
//
// Best-effort: if icacls.exe is missing or a file is not there yet, we log
// and continue — service startup must never depend on it.
func hardenWindowsACLs(stateDir string) {
	icacls, err := exec.LookPath("icacls.exe")
	if err != nil {
		writeDebugLog("icacls.exe not found, skipping ACL hardening")
		return
	}

	run := func(target string, args ...string) {
		out, err := exec.Command(icacls, append([]string{target}, args...)...).CombinedOutput()
		if err != nil {
			// Keep icacls' own output: without it a failing grant is
			// indistinguishable from a missing file and the empty-DACL
			// lockout above would be undebuggable.
			writeDebugLog(fmt.Sprintf("icacls on %s failed (non-fatal): %v: %s",
				target, err, strings.TrimSpace(string(out))))
		}
	}

	targets := []struct {
		path string
		// extra trustees granted before inheritance is stripped; "" = SYSTEM only
		grant string
	}{
		{filepath.Join(stateDir, "api-token"), "*S-1-5-32-544:R"},
		{filepath.Join(stateDir, "config.json"), ""},
		{filepath.Join(stateDir, "scheduled_jobs.json"), ""},
		{filepath.Join(stateDir, "job_history.json"), ""},
	}

	for _, t := range targets {
		if _, err := os.Stat(t.path); err != nil {
			writeDebugLog(fmt.Sprintf("ACL hardening: %s not present, skipped", t.path))
			continue
		}
		// Grant first (SYSTEM must always keep access), strip inherited ACEs
		// only after every grant succeeded — see the comment above.
		run(t.path, "/grant:r", "*S-1-5-18:F")
		if t.grant != "" {
			run(t.path, "/grant:r", t.grant)
		}
		run(t.path, "/inheritance:r")
	}
}
