//go:build !service
// +build !service

package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	stdruntime "runtime"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wailswin "github.com/wailsapp/wails/v2/pkg/options/windows"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"pbscommon"
	"security"
	"snapshot"

	"github.com/tizbac/proxmoxbackupclient_go/gui/api"
)

//go:embed all:frontend/dist
var assets embed.FS

const (
	appName = "Nimbus Backup"
)


var (
	crashReportPath string
)

func init() {

	// Get executable directory for crash reports
	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)
	crashReportPath = filepath.Join(exeDir, "crash_report.txt")

	// Setup panic recovery
	defer func() {
		if r := recover(); r != nil {
			crashMsg := fmt.Sprintf("PANIC during init: %v\n%s", r, debug.Stack())
			writeDebugLog(crashMsg)
			writeCrashReport(crashMsg)
		}
	}()
}

func main() {
	// Parse command line flags
	minimized := flag.Bool("minimized", false, "Start minimized to system tray")
	flag.Parse()

	// Check for single instance (GUI only)
	// If another instance exists, activate it and exit
	if !CheckSingleInstance() {
		fmt.Println("Another instance is already running. Activating existing window...")
		os.Exit(0)
	}

	// Setup panic recovery for main
	defer func() {
		if r := recover(); r != nil {
			crashMsg := fmt.Sprintf("PANIC in main: %v\n%s", r, debug.Stack())
			writeDebugLog(crashMsg)
			writeCrashReport(crashMsg)
			fmt.Fprint(os.Stderr, "\n!!! APPLICATION CRASHED !!!\nSee crash_report.txt for details\n")
			os.Exit(1)
		}
	}()

	// Logging is now handled by RotatingLogger (initialized in logging_gui.go)
	writeDebugLog(fmt.Sprintf("=== %s v%s Starting ===", appName, appVersion))
	writeDebugLog(fmt.Sprintf("Time: %s", time.Now().Format(time.RFC3339)))
	writeDebugLog(fmt.Sprintf("Service log: %s", GetServiceLogPath()))
	writeDebugLog(fmt.Sprintf("Backup log: %s", GetBackupLogPath()))
	writeDebugLog(fmt.Sprintf("Crash report path: %s", crashReportPath))

	// Install SIGINT/SIGTERM handler so any live PBS backup session gets
	// closed before we exit. Without this, a forced kill (e.g. "update
	// and restart") leaves the HTTP/2 connection dangling — PBS keeps
	// the snapshot lock until TCP keepalive reaps it ~16 min later,
	// which blocks the next verify run on that group.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		writeDebugLog(fmt.Sprintf("Signal %v received — closing active PBS sessions", sig))
		pbscommon.CloseAllActive()
		os.Exit(1)
	}()

	// Clean up legacy auto-start from previous versions
	// (Task Scheduler or Registry entries before MSI service)
	CleanupLegacyAutoStart()

	// Create app instance
	app := NewApp()
	writeDebugLog("App instance created")

	// Create application options
	appOptions := &options.App{
		Title:     fmt.Sprintf("%s v%s", appName, appVersion),
		Width:     1000,
		Height:    700,
		MaxWidth:  1400, // Prevent window from being too large
		MaxHeight: 900,  // Prevent title bar from going off-screen
		MinWidth:  400,  // Allow very small windows for low-res screens
		MinHeight: 300,  // Allow very small windows for low-res screens
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		StartHidden:      *minimized, // Start hidden if --minimized flag is set
		OnStartup:        app.startup,
		OnDomReady:       app.domReady,
		OnBeforeClose:    app.beforeClose,
		OnShutdown:       app.shutdown,
		Bind: []interface{}{
			app,
		},
		Windows: &wailswin.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			DisableWindowIcon:    false,
			WebviewUserDataPath:  filepath.Join(os.Getenv("APPDATA"), "NimbusBackup"),
		},
	}

	if *minimized {
		writeDebugLog("Starting in minimized mode (hidden to tray)")
	}

	writeDebugLog("Application options configured")

	// Run application
	writeDebugLog("Starting Wails runtime...")
	err := wails.Run(appOptions)

	if err != nil {
		errMsg := fmt.Sprintf("ERROR: Wails.Run failed: %v\nStack trace:\n%s", err, debug.Stack())
		writeDebugLog(errMsg)
		writeCrashReport(errMsg)
		fmt.Fprint(os.Stderr, "\n!!! APPLICATION FAILED TO START !!!\n")
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		fmt.Fprintf(os.Stderr, "Check crash_report.txt and %s\n", GetServiceLogPath())
		os.Exit(1)
	}

	writeDebugLog("Application shutdown normally")
}


func writeCrashReport(message string) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")

	crashContent := fmt.Sprintf(`=== NIMBUS BACKUP CRASH REPORT ===
Time: %s
Version: %s

%s

=== SYSTEM INFO ===
Service Log: %s
Backup Log: %s

Please report this issue to RDEM Systems:
- Website: https://nimbus.rdem-systems.com
- Include this crash_report.txt file
`, timestamp, appVersion, message, GetServiceLogPath(), GetBackupLogPath())

	// Write to crash report file (overwrite each time)
	err := os.WriteFile(crashReportPath, []byte(crashContent), 0600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write crash report: %v\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "Crash report written to: %s\n", crashReportPath)
	}
}


// SetProgressCallbacks sets custom progress callbacks for API mode
func (a *App) SetProgressCallbacks(jobID string, onProgress func(string, float64, string), onComplete func(string, bool, string)) {
	writeDebugLog(fmt.Sprintf("[SetProgressCallbacks] Registered callbacks for jobID: %s", jobID))
	a.callbacksMutex.Lock()
	a.callbacksMap[jobID] = &progressCallbacks{
		onProgress: onProgress,
		onComplete: onComplete,
	}
	a.callbacksMutex.Unlock()
}

// startup is called when the app starts
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	writeDebugLog("App.startup() called")

	// Detect execution mode (Service vs Standalone)
	detector := api.NewModeDetector(getAPITokenPath())
	a.mode = detector.DetectMode()
	writeDebugLog(fmt.Sprintf("Execution mode: %s", a.mode.String()))

	// If running in standalone mode, start local scheduler
	// If in service mode, scheduler runs in the service
	if a.mode == api.ModeStandalone {
		// Cleanup any abandoned "running" jobs from previous session
		a.CleanupAbandonedJobs()

		// Clear any orphaned VSS shadow copies and reset the VSS service
		// state from a previously crashed Nimbus process. Without this, the
		// next backup can fail with "shadow copy creation is already in
		// progress". No-op on non-Windows platforms.
		if err := snapshot.VSSCleanup(); err != nil {
			writeDebugLog(fmt.Sprintf("VSS cleanup at startup reported error: %v", err))
		}

		// Recalculate stale nextRun values (e.g. after restart or missed window)
		a.RecalculateNextRuns()

		// Start background job scheduler
		a.StartScheduler()
		writeDebugLog("Background scheduler started (standalone mode)")
	} else {
		writeDebugLog("Service mode detected - scheduler runs in service")
	}

	// Execute startup jobs (jobs with runAtStartup=true)
	// Note: In service mode, these will be sent via API
	go a.HandleStartupRun()

	// Trim stale restore listing caches in the background (best-effort).
	go func() {
		trimSnapshotTreeCache(30 * 24 * time.Hour)
	}()

	// Setup system tray for background operation
	go a.SetupSystemTray()
}

// domReady is called after front-end resources have been loaded
func (a *App) domReady(ctx context.Context) {
	writeDebugLog("App.domReady() called - UI loaded successfully")
}

// beforeClose is called when the application is about to quit
func (a *App) beforeClose(ctx context.Context) (prevent bool) {
	writeDebugLog("App.beforeClose() called - minimizing to tray")
	// Instead of closing, minimize to tray
	a.MinimizeToTray()
	return true // Prevent actual close
}

// shutdown is called at application termination
func (a *App) shutdown(ctx context.Context) {
	writeDebugLog("App.shutdown() called — closing active PBS sessions")
	pbscommon.CloseAllActive()
}

// GetConfig returns the current configuration with secrets stripped (M-04). It is
// Wails-bound, so it must never expose tokens to the frontend; internal callers
// use a.config directly.
func (a *App) GetConfig() *Config {
	writeDebugLog("GetConfig() called from frontend")
	return a.config.sanitized()
}

// GetHostname returns the system hostname
func (a *App) GetHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		writeDebugLog(fmt.Sprintf("GetHostname() error: %v", err))
		return "unknown"
	}
	writeDebugLog(fmt.Sprintf("GetHostname() returned: %s", hostname))
	return hostname
}

// GetSystemInfo returns system information for UI (mode, admin status, etc.)
func (a *App) GetSystemInfo() map[string]interface{} {
	return map[string]interface{}{
		"mode":              a.mode.String(),
		"is_admin":          isAdmin(),
		"hostname":          a.GetHostname(),
		"service_available": a.mode == api.ModeService,
		// os = runtime.GOOS ("windows", "linux", "darwin") — used by the
		// restore UI to enable/disable the in-place mode when the snapshot
		// was taken on a different platform.
		"os": stdruntime.GOOS,
	}
}

func (a *App) GetVersion() string {
	writeDebugLog(fmt.Sprintf("GetVersion() returned: %s", appVersion))
	return appVersion
}

// ListPhysicalDisks returns a list of available physical disks (DISABLED - feature postponed)
/*
func (a *App) ListPhysicalDisks() ([]PhysicalDiskInfo, error) {
	writeDebugLog("ListPhysicalDisks() called from frontend")
	disks, err := ListPhysicalDisks()
	if err != nil {
		writeDebugLog(fmt.Sprintf("ListPhysicalDisks() error: %v", err))
		return nil, err
	}
	writeDebugLog(fmt.Sprintf("Found %d physical disks", len(disks)))
	return disks, nil
}
*/

// GetConfigWithHostname returns config with hostname pre-filled
func (a *App) GetConfigWithHostname() map[string]interface{} {
	hostname := a.GetHostname()
	cfg := a.config

	// Return config as map with hostname
	result := map[string]interface{}{
		"baseurl":         cfg.BaseURL,
		"certfingerprint": cfg.CertFingerprint,
		"authid":          cfg.AuthID,
		// M-04: never hand the PBS token to the webview/frontend. Expose only
		// whether one is stored; SaveConfig keeps the existing secret when the
		// frontend submits an empty value, and TestConnection falls back to it.
		"secret":          "",
		"secret_set":      cfg.Secret != "",
		"datastore":       cfg.Datastore,
		"namespace":       cfg.Namespace,
		"backupdir":       cfg.BackupDir,
		"backup-id":       cfg.BackupID,
		"usevss":          cfg.UseVSS,
		"hostname":        hostname,
	}

	// Pre-fill backup-id with hostname if empty
	if cfg.BackupID == "" {
		result["backup-id"] = hostname
	}

	return result
}

// DiagnoseConfig returns config validation status for debugging
func (a *App) DiagnoseConfig() map[string]interface{} {
	cfg := a.config

	var validationError string
	if err := cfg.Validate(); err != nil {
		validationError = err.Error()
	}

	configPath, _ := getConfigPath()

	return map[string]interface{}{
		"config_path":       configPath,
		"baseurl_set":       cfg.BaseURL != "",
		"baseurl_value":     security.SanitizeURL(cfg.BaseURL),
		"authid_set":        cfg.AuthID != "",
		"datastore_set":     cfg.Datastore != "",
		"validation_ok":     validationError == "",
		"validation_error":  validationError,
		"mode":              a.mode.String(),
	}
}

// SaveConfig saves the configuration
func (a *App) SaveConfig(config *Config) error {
	// M-04: the frontend never receives the stored secrets (GetConfigWithHostname
	// returns "" + a *_set marker), so an empty value here means "keep the existing
	// one", not "clear it". Only overwrite when the user supplied a new value.
	if a.config != nil {
		if config.Secret == "" {
			config.Secret = a.config.Secret
		}
		if config.SMTPPassword == "" {
			config.SMTPPassword = a.config.SMTPPassword
		}
	}

	// Log sanitized config (no secrets)
	writeDebugLog(fmt.Sprintf("SaveConfig() called: URL=%s, AuthID=%s, Datastore=%s, BackupID=%s",
		security.SanitizeURL(config.BaseURL),
		config.AuthID,
		config.Datastore,
		config.BackupID))

	// Validate before saving
	if err := config.Validate(); err != nil {
		writeDebugLog(fmt.Sprintf("Config validation failed: %v", err))
		return err
	}

	// Save to disk
	if err := config.Save(); err != nil {
		writeDebugLog(fmt.Sprintf("Config save to disk failed: %v", err))
		return err
	}

	// Update in-memory config
	a.config = config
	writeDebugLog("Config saved successfully and loaded into app")
	return nil
}

// TestConnection tests the PBS connection with the provided config (or current if nil)
func (a *App) TestConnection(config *Config) error {
	writeDebugLog("TestConnection() called")

	// Use provided config or fallback to current app config
	testConfig := config
	if testConfig == nil {
		testConfig = a.config
	}

	// M-04: the frontend no longer holds the secret, so an empty secret in the
	// submitted config means "use the stored one" (test the existing connection).
	if testConfig != nil && testConfig.Secret == "" && a.config != nil {
		testConfig.Secret = a.config.Secret
	}

	// Validate config first
	if err := testConfig.Validate(); err != nil {
		return err
	}

	// Create PBS client
	client := &pbscommon.PBSClient{
		BaseURL:          testConfig.BaseURL,
		CertFingerPrint:  testConfig.CertFingerprint,
		AuthID:           testConfig.AuthID,
		Secret:           testConfig.Secret,
		Datastore:        testConfig.Datastore,
		Namespace:        testConfig.Namespace,
		Insecure:         testConfig.CertFingerprint != "",
		CompressionLevel: pbscommon.CompressionFastest, // Default for test connections
		Manifest: pbscommon.BackupManifest{
			BackupID: testConfig.BackupID,
		},
	}

	// Debug log with sanitized credentials
	writeDebugLog(fmt.Sprintf("Testing connection: URL=%s, AuthID=%s, Secret=%s, Datastore=%s",
		security.SanitizeURL(testConfig.BaseURL),
		testConfig.AuthID,
		security.SanitizeSecret(testConfig.Secret),
		testConfig.Datastore))

	// Perform real HTTP test (checks DNS, connectivity, auth, datastore access)
	if err := client.TestConnection(); err != nil {
		writeDebugLog(fmt.Sprintf("Connection test failed: %v", err))
		return err
	}

	writeDebugLog("Connection test successful (authenticated + datastore accessible)")
	return nil
}

// GetLastBackupDirs returns the last used backup directories
func (a *App) GetLastBackupDirs() []string {
	writeDebugLog(fmt.Sprintf("GetLastBackupDirs() returned %d directories", len(a.config.LastBackupDirs)))
	return a.config.LastBackupDirs
}

// ReloadConfig reloads configuration from disk (for service when config changes)
func (a *App) ReloadConfig() {
	newConfig := LoadConfig()
	a.config = newConfig
	writeDebugLog("Config reloaded from disk")
}

// ==================== MULTI-PBS MANAGEMENT ====================

// ListPBSServers returns all configured PBS servers
func (a *App) ListPBSServers() []*PBSServer {
	servers := a.config.ListPBSServers()
	writeDebugLog(fmt.Sprintf("ListPBSServers() returned %d servers", len(servers)))
	// M-04: never hand PBS tokens to the frontend — return sanitized copies.
	out := make([]*PBSServer, 0, len(servers))
	for _, s := range servers {
		out = append(out, s.sanitized())
	}
	return out
}

// GetPBSServer returns a single PBS server by ID (secret stripped — M-04).
func (a *App) GetPBSServer(id string) (*PBSServer, error) {
	writeDebugLog(fmt.Sprintf("GetPBSServer(%s) called", id))
	s, err := a.config.GetPBSServer(id)
	if err != nil {
		return nil, err
	}
	return s.sanitized(), nil
}

// AddPBSServer adds a new PBS server to the configuration
func (a *App) AddPBSServer(pbs *PBSServer) error {
	writeDebugLog(fmt.Sprintf("AddPBSServer(%s) called", pbs.ID))
	return a.config.AddPBSServer(pbs)
}

// UpdatePBSServer updates an existing PBS server
func (a *App) UpdatePBSServer(pbs *PBSServer) error {
	writeDebugLog(fmt.Sprintf("UpdatePBSServer(%s) called", pbs.ID))
	// M-04: the frontend never receives the token (sanitized), so an empty secret
	// on update means "keep the stored one", not "clear it".
	if pbs.Secret == "" {
		if existing, err := a.config.GetPBSServer(pbs.ID); err == nil && existing != nil {
			pbs.Secret = existing.Secret
		}
	}
	return a.config.UpdatePBSServer(pbs)
}

// DeletePBSServer removes a PBS server
func (a *App) DeletePBSServer(id string) error {
	writeDebugLog(fmt.Sprintf("DeletePBSServer(%s) called", id))
	return a.config.DeletePBSServer(id)
}

// SetDefaultPBSServer sets the default PBS server
func (a *App) SetDefaultPBSServer(id string) error {
	writeDebugLog(fmt.Sprintf("SetDefaultPBSServer(%s) called", id))
	return a.config.SetDefaultPBS(id)
}

// GetDefaultPBSID returns the default PBS server ID
func (a *App) GetDefaultPBSID() string {
	return a.config.DefaultPBSID
}

// TestPBSConnection tests connection to a specific PBS server
func (a *App) TestPBSConnection(pbsID string) error {
	writeDebugLog(fmt.Sprintf("TestPBSConnection(%s) called", pbsID))

	pbs, err := a.config.GetPBSServer(pbsID)
	if err != nil {
		return err
	}

	// Convert to legacy Config format for existing TestConnection logic
	legacyConfig := pbs.ToConfig()
	return a.TestConnection(legacyConfig)
}

// GetServerFingerprint connects to baseURL and returns the server certificate's
// SHA-256 fingerprint (AA:BB:... uppercase) so the UI can offer trust-on-first-use
// pinning when a self-signed PBS rejects CA validation (audit H-02). Discovery
// only: no token is sent.
func (a *App) GetServerFingerprint(baseURL string) (string, error) {
	writeDebugLog(fmt.Sprintf("GetServerFingerprint(%s) called", security.SanitizeURL(baseURL)))
	fp, err := pbscommon.FetchServerFingerprint(baseURL)
	if err != nil {
		writeDebugLog(fmt.Sprintf("GetServerFingerprint failed: %v", err))
		return "", err
	}
	writeDebugLog(fmt.Sprintf("GetServerFingerprint discovered: %s", fp))
	return fp, nil
}

// PinPBSServerFingerprint stores fingerprint on the PBS server identified by id,
// resolving the secret server-side so the frontend (which never holds the token,
// M-04) can pin a discovered fingerprint without round-tripping credentials.
func (a *App) PinPBSServerFingerprint(id, fingerprint string) error {
	writeDebugLog(fmt.Sprintf("PinPBSServerFingerprint(%s) called", id))
	if err := security.ValidateFingerprint(fingerprint); err != nil {
		return fmt.Errorf("empreinte certificat invalide: %w", err)
	}
	// config.json lives under ProgramData and is owned by whichever process wrote it
	// first. When the privileged service is running it owns the file, so the
	// unprivileged GUI cannot overwrite it: its Save() rename fails and TOFU pinning
	// silently never persists (the connection test keeps reporting offline). Route the
	// write through the service in that case so a single privileged writer owns the
	// file; standalone GUIs (no service) write directly as before.
	if !a.isServiceProcess && a.mode == api.ModeService && a.apiClient != nil {
		writeDebugLog(fmt.Sprintf("PinPBSServerFingerprint(%s): delegating write to service", id))
		if err := a.apiClient.PinFingerprint(id, fingerprint); err != nil {
			writeDebugLog(fmt.Sprintf("PinPBSServerFingerprint: service-side pin failed: %v", err))
			return err
		}
		// Refresh our in-memory copy so a follow-up TestPBSConnection in this process
		// sees the fingerprint the service just wrote to disk.
		a.ReloadConfig()
		return nil
	}
	return a.pinFingerprintLocal(id, fingerprint)
}

// ==================== END MULTI-PBS MANAGEMENT ====================

// emitAnalysisProgress forwards split size-analysis progress to the GUI as an
// "analysis:progress" event (done/total folders sized + bytes so far). Used only
// on the explicit-split path so a multi-minute scan of a large volume shows
// movement instead of a frozen spinner.
func (a *App) emitAnalysisProgress(done, total int, scannedBytes uint64) {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, "analysis:progress", map[string]interface{}{
		"done":  done,
		"total": total,
		"bytes": scannedBytes,
	})
}

// StartBackup starts a backup operation (routes to service or direct based on mode)
func (a *App) StartBackup(backupType string, backupDirs []string, driveLetters []string, excludeList []string, backupID string, useVSS bool, compression string) error {
	writeDebugLog(fmt.Sprintf("StartBackup() called - mode: %s, VSS: %v, compression: %s, isServiceProcess: %v", a.mode.String(), useVSS, compression, a.isServiceProcess))

	// Default to "fastest" if compression is empty
	if compression == "" {
		compression = "fastest"
		writeDebugLog("[Compression] Using default: fastest")
	}

	// Re-detect mode if currently Standalone (service may have started after GUI)
	// IMPORTANT: Never re-detect if we ARE the service process (prevents infinite loop)
	if !a.isServiceProcess && a.mode == api.ModeStandalone {
		if a.apiClient.IsServiceAvailable() {
			writeDebugLog("[Mode Detection] Service now available, switching to Service mode")
			a.mode = api.ModeService
		}
	}

	// Route based on execution mode
	switch a.mode {
	case api.ModeService:
		// Use HTTP API to communicate with service (service has admin rights as LocalSystem)
		return a.startBackupViaService(backupType, backupDirs, driveLetters, excludeList, backupID, useVSS, compression)
	case api.ModeStandalone:
		// Direct execution - check admin if VSS requested
		if useVSS && !isAdmin() {
			return fmt.Errorf("VSS (Shadow Copy) nécessite les privilèges administrateur - veuillez redémarrer l'application en tant qu'administrateur ou désactiver VSS")
		}
		return a.startBackupDirect(backupType, backupDirs, driveLetters, excludeList, backupID, useVSS, compression)
	default:
		return fmt.Errorf("unknown execution mode: %v", a.mode)
	}
}

// startBackupViaService sends backup request to the service via HTTP API
func (a *App) startBackupViaService(backupType string, backupDirs []string, driveLetters []string, excludeList []string, backupID string, useVSS bool, compression string) error {
	writeDebugLog("[Service Mode] Sending backup request to service")

	req := &api.BackupRequest{
		BackupType:   backupType,
		BackupID:     backupID,
		BackupDirs:   backupDirs,
		DriveLetters: driveLetters,
		ExcludeList:  excludeList,
		UseVSS:       useVSS,
		Compression:  compression,
	}

	resp, err := a.apiClient.StartBackup(req)
	if err != nil {
		writeDebugLog(fmt.Sprintf("[Service Mode] Backup request failed: %v", err))
		return fmt.Errorf("échec de la communication avec le service: %w", err)
	}

	writeDebugLog(fmt.Sprintf("[Service Mode] Backup started: %s (JobID: %s)", resp.Message, resp.JobID))

	// Start polling for progress updates
	go a.pollBackupProgress(resp.JobID)

	return nil
}

// pollBackupProgress polls the service for backup progress and emits events to GUI
func (a *App) pollBackupProgress(jobID string) {
	writeDebugLog(fmt.Sprintf("[Service Mode] Starting progress polling for job: %s", jobID))
	ticker := time.NewTicker(3 * time.Second) // Poll every 3 seconds
	defer ticker.Stop()

	// Without a bound, a permanently-404ing job (evicted/collided entry, or a
	// service restart that dropped the progress map) would poll forever. Give up
	// after a run of consecutive failures so the goroutine can't leak.
	consecutiveErrors := 0
	const maxConsecutiveErrors = 20 // ~60s at 3s interval

	for range ticker.C {
		progress, err := a.apiClient.GetBackupStatus(jobID)
		if err != nil {
			consecutiveErrors++
			writeDebugLog(fmt.Sprintf("[Service Mode] Failed to get progress (%d/%d): %v", consecutiveErrors, maxConsecutiveErrors, err))
			if consecutiveErrors >= maxConsecutiveErrors {
				writeDebugLog("[Service Mode] Giving up polling after repeated failures")
				if a.ctx != nil {
					runtime.EventsEmit(a.ctx, "backup:complete", map[string]interface{}{
						"success": false,
						"message": "Lost contact with backup service (status unavailable)",
					})
				}
				return
			}
			continue
		}
		consecutiveErrors = 0

		// Emit progress event to GUI
		if a.ctx != nil && progress.Running {
			runtime.EventsEmit(a.ctx, "backup:progress", map[string]interface{}{
				"percent": progress.Progress,
				"message": progress.Message,
			})
		}

		// If backup completed, emit final event and stop polling
		if progress.Complete {
			writeDebugLog(fmt.Sprintf("[Service Mode] Backup completed: success=%v", progress.Success))
			if a.ctx != nil {
				runtime.EventsEmit(a.ctx, "backup:complete", map[string]interface{}{
					"success": progress.Success,
					"message": progress.Message,
				})
			}
			return
		}
	}
}

// startBackupDirect performs backup directly (standalone mode)
func (a *App) startBackupDirect(backupType string, backupDirs []string, driveLetters []string, excludeList []string, backupID string, useVSS bool, compression string) error {
	// Use hostname as fallback if backupID is empty
	if backupID == "" {
		backupID = a.GetHostname()
		writeDebugLog(fmt.Sprintf("[Backup ID] Empty backup-id, using hostname: %s", backupID))
	}

	// Sanitize backup ID for logging
	sanitizedID := security.SanitizeForLog(backupID)
	writeDebugLog(fmt.Sprintf("[Standalone Mode] StartBackup: type=%s, id=%s, vss=%v, compression=%s, dir_count=%d",
		backupType, sanitizedID, useVSS, compression, len(backupDirs)))

	// Validate BackupID (now guaranteed to be non-empty)
	if err := security.ValidateBackupID(backupID); err != nil {
		return fmt.Errorf("backup ID invalide: %w", err)
	}

	// Validate backup directories
	for _, dir := range backupDirs {
		if err := security.ValidatePath(dir); err != nil {
			return fmt.Errorf("chemin invalide '%s': %w", dir, err)
		}
	}

	// Note: Admin check for VSS is done in StartBackup() routing layer
	// If we're here via service, we're already running as LocalSystem

	// Resolve PBS fields from multi-PBS default when legacy fields are empty
	pbsCfg := a.config.EffectivePBS()

	// Validate PBS config
	if err := pbsCfg.Validate(); err != nil {
		return err
	}

	// Validate backup parameters and build target list
	var targetDirs []string
	if backupType == "directory" {
		if len(backupDirs) == 0 {
			return fmt.Errorf("au moins un répertoire de sauvegarde requis")
		}
		targetDirs = backupDirs
	}
	if backupType == "machine" {
		if len(driveLetters) == 0 {
			return fmt.Errorf("au moins un disque physique requis")
		}
		// Physical drive paths are used directly (e.g., \\.\PhysicalDrive0)
		targetDirs = driveLetters
	}

	// Prepare backup options
	opts := BackupOptions{
		BaseURL:         pbsCfg.BaseURL,
		AuthID:          pbsCfg.AuthID,
		Secret:          pbsCfg.Secret,
		Datastore:       pbsCfg.Datastore,
		Namespace:       pbsCfg.Namespace,
		CertFingerprint: pbsCfg.CertFingerprint,
		BackupDirs:      targetDirs,
		BackupID:        backupID,
		BackupType:      "host", // "host" for directory, would be "vm" for machine
		UseVSS:          useVSS,
		Compression:     compression,
		ExcludeList:     excludeList,
		DisableSplit:    a.config.DisableSplit,
		SplitSizeBytes:  a.config.SplitSizeBytes(),
		OnProgress: func(percent float64, message string) {
			writeDebugLog(fmt.Sprintf("Progress: %.1f%% - %s", percent*100, message))

			// Check if there's a registered callback for any job (service mode)
			a.callbacksMutex.RLock()
			hasCallbacks := len(a.callbacksMap) > 0
			if hasCallbacks {
				// Call all registered callbacks (typically just one per backup)
				for jobID, callbacks := range a.callbacksMap {
					if callbacks.onProgress != nil {
						writeDebugLog(fmt.Sprintf("[OnProgress] Calling custom callback for jobID: %s", jobID))
						callbacks.onProgress(jobID, percent*100, message)
					}
				}
			}
			a.callbacksMutex.RUnlock()

			// If no custom callbacks and we have Wails context, emit events (GUI standalone mode)
			// NEVER emit events if we're the service process (no Wails runtime)
			if !hasCallbacks && !a.isServiceProcess && a.ctx != nil {
				writeDebugLog("[OnProgress] Emitting Wails event (GUI mode)")
				runtime.EventsEmit(a.ctx, "backup:progress", map[string]interface{}{
					"percent": percent * 100,
					"message": message,
				})
			} else if !hasCallbacks && (a.isServiceProcess || a.ctx == nil) {
				writeDebugLog("[OnProgress] No callbacks/context (service or headless mode)")
			}
		},
		OnComplete: func(success bool, message string) {
			writeDebugLog(fmt.Sprintf("Backup complete: success=%v, %s", success, message))

			// Check if there's a registered callback for any job (service mode)
			a.callbacksMutex.RLock()
			hasCallbacks := len(a.callbacksMap) > 0
			var jobIDsToCleanup []string
			if hasCallbacks {
				// Call all registered callbacks and collect jobIDs for cleanup
				for jobID, callbacks := range a.callbacksMap {
					if callbacks.onComplete != nil {
						writeDebugLog(fmt.Sprintf("[OnComplete] Calling custom callback for jobID: %s", jobID))
						callbacks.onComplete(jobID, success, message)
					}
					jobIDsToCleanup = append(jobIDsToCleanup, jobID)
				}
			}
			a.callbacksMutex.RUnlock()

			// Clean up completed callbacks
			if len(jobIDsToCleanup) > 0 {
				a.callbacksMutex.Lock()
				for _, jobID := range jobIDsToCleanup {
					delete(a.callbacksMap, jobID)
					writeDebugLog(fmt.Sprintf("[OnComplete] Cleaned up callbacks for jobID: %s", jobID))
				}
				a.callbacksMutex.Unlock()
			}

			// If no custom callbacks and we have Wails context, emit events (GUI standalone mode)
			// NEVER emit events if we're the service process (no Wails runtime)
			if !hasCallbacks && !a.isServiceProcess && a.ctx != nil {
				writeDebugLog("[OnComplete] Emitting Wails event (GUI mode)")
				runtime.EventsEmit(a.ctx, "backup:complete", map[string]interface{}{
					"success": success,
					"message": message,
				})
			} else if !hasCallbacks && (a.isServiceProcess || a.ctx == nil) {
				writeDebugLog("[OnComplete] No callbacks/context (service or headless mode)")
			}

			// Add manual backup to history
			historyEntry := JobHistory{
				ID:         fmt.Sprintf("%d", time.Now().Unix()),
				Name:       fmt.Sprintf("Backup manuel - %s", backupID),
				Timestamp:  time.Now().Format(time.RFC3339),
				Status:     "success",
				Message:    message,
				BackupDirs: targetDirs,
				BackupID:   backupID,
				UseVSS:     useVSS,
			}
			if !success {
				historyEntry.Status = "failed"
			}
			if err := a.AddJobHistory(historyEntry); err != nil {
				writeDebugLog(fmt.Sprintf("Warning: Failed to add manual backup to history: %v", err))
			}

			// Save last used backup directories on success
			if success && backupType == "directory" {
				a.config.LastBackupDirs = backupDirs
				if err := a.config.Save(); err != nil {
					writeDebugLog(fmt.Sprintf("Failed to save last backup dirs: %v", err))
				} else {
					writeDebugLog(fmt.Sprintf("Saved %d backup directories to config", len(backupDirs)))
				}
			}
		},
	}

	// Structured live stats + final structured result for the GUI (standalone mode).
	// In the service process there is no Wails runtime, and the service-mode stats
	// bridge is a separate backlog item (service-mode progress), so we only emit here.
	opts.OnStats = func(stats *BackupProgressStats) {
		if a.isServiceProcess || a.ctx == nil {
			return
		}
		runtime.EventsEmit(a.ctx, "backup:stats", map[string]interface{}{
			"percent":      stats.Percent * 100,
			"bytesDone":    stats.BytesDone,
			"bytesTotal":   stats.BytesTotal,
			"newChunks":    stats.NewChunks,
			"reusedChunks": stats.ReusedChunks,
			"failedChunks": stats.FailedChunks,
			"currentDir":   stats.CurrentDir,
			"message":      stats.Message,
		})
	}
	opts.OnResult = func(status *BackupStatus) {
		if a.isServiceProcess || a.ctx == nil {
			return
		}
		runtime.EventsEmit(a.ctx, "backup:result", map[string]interface{}{
			"outcome":      string(status.Outcome),
			"newChunks":    status.NewChunks,
			"reusedChunks": status.ReusedChunks,
			"failedChunks": status.FailedChunks,
			"totalBytes":   status.TotalBytes,
			"durationSec":  status.DurationSec,
			"skippedCount": len(status.SkippedReadError),
		})
	}

	// Run backup inline (in background goroutine to not block UI)
	go func() {
		// Machine backup disabled for now - Windows Defender flags it
		// if backupType == "machine" {
		// 	err = RunMachineBackup(opts)
		// } else {
		err := RunBackupInline(opts)
		// }
		if err != nil {
			writeDebugLog(fmt.Sprintf("Backup error: %v", err))
		}
	}()

	return nil
}

// ==================== RESTORE ====================

// resolveRestorePBS picks the PBS server to restore from. When pbsID is empty
// the default PBS server is used. Falls back to legacy single-server fields
// when no multi-PBS entry is configured.
func (a *App) resolveRestorePBS(pbsID string) (*Config, error) {
	if pbsID != "" {
		pbs, err := a.config.GetPBSServer(pbsID)
		if err != nil {
			return nil, err
		}
		return pbs.ToConfig(), nil
	}
	cfg := a.config.EffectivePBS()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// ListSnapshots lists available snapshots on a PBS server, optionally filtered
// by backup ID (partial match supports split backups).
//
// pbsID selects the PBS server. Empty means "use the default server" — kept
// for backward compatibility with the legacy single-PBS UI.
func (a *App) ListSnapshots(pbsID, backupID string) ([]map[string]interface{}, error) {
	writeDebugLog(fmt.Sprintf("ListSnapshots(pbs=%s, backupID=%s)", pbsID, backupID))

	cfg, err := a.resolveRestorePBS(pbsID)
	if err != nil {
		return nil, err
	}

	snaps, err := ListSnapshotsInline(cfg.BaseURL, cfg.AuthID, cfg.Secret,
		cfg.Datastore, cfg.Namespace, cfg.CertFingerprint, backupID)
	if err != nil {
		writeDebugLog(fmt.Sprintf("ListSnapshotsInline failed: %v", err))
		return nil, fmt.Errorf("échec de la liste des snapshots: %v", err)
	}

	result := make([]map[string]interface{}, 0, len(snaps))
	for _, s := range snaps {
		result = append(result, map[string]interface{}{
			"id":          s.BackupTime.UTC().Format("2006-01-02T15:04:05Z"),
			"backup_id":   s.BackupID,
			"backup_type": s.BackupType,
			"time":        s.BackupTime.Format("2006-01-02 15:04:05"),
			"unix":        s.BackupTime.Unix(),
			"files":       s.Files,
		})
	}
	writeDebugLog(fmt.Sprintf("Returning %d snapshots", len(result)))
	return result, nil
}

// ListSnapshotContents downloads a snapshot's PXAR archive and returns its
// flat tree of entries. The frontend turns this into a navigable view so the
// user can pick individual files or directories before restoring.
//
// snapshotUnix is the snapshot's backup-time as Unix seconds (the `unix` field
// returned by ListSnapshots). Set forceRefresh to bypass the local listing
// cache — useful for a manual "Reload" action.
func (a *App) ListSnapshotContents(pbsID, backupID string, snapshotUnix int64, forceRefresh bool) ([]SnapshotEntry, error) {
	writeDebugLog(fmt.Sprintf("ListSnapshotContents(pbs=%s, backupID=%s, unix=%d, force=%v)",
		pbsID, backupID, snapshotUnix, forceRefresh))

	cfg, err := a.resolveRestorePBS(pbsID)
	if err != nil {
		return nil, err
	}
	if backupID == "" {
		return nil, fmt.Errorf("backup ID requis")
	}

	opts := RestoreOptions{
		BaseURL:         cfg.BaseURL,
		AuthID:          cfg.AuthID,
		Secret:          cfg.Secret,
		Datastore:       cfg.Datastore,
		Namespace:       cfg.Namespace,
		CertFingerprint: cfg.CertFingerprint,
		BackupID:        backupID,
		SnapshotTime:    time.Unix(snapshotUnix, 0),
	}
	return ListSnapshotContentsInline(opts, "", forceRefresh)
}

// GetSnapshotMeta returns the `.nimbus_backup_meta.json` sidecar from a
// snapshot. Returns nil (not an error) when the snapshot predates the sidecar
// — the frontend should fall back to a generic banner in that case.
//
// Cheap when the snapshot has already been listed: the meta is bundled in the
// same restore-cache envelope as the entries.
func (a *App) GetSnapshotMeta(pbsID, backupID string, snapshotUnix int64) (*BackupMeta, error) {
	writeDebugLog(fmt.Sprintf("GetSnapshotMeta(pbs=%s, backupID=%s, unix=%d)",
		pbsID, backupID, snapshotUnix))

	cfg, err := a.resolveRestorePBS(pbsID)
	if err != nil {
		return nil, err
	}
	if backupID == "" {
		return nil, fmt.Errorf("backup ID requis")
	}

	opts := RestoreOptions{
		BaseURL:         cfg.BaseURL,
		AuthID:          cfg.AuthID,
		Secret:          cfg.Secret,
		Datastore:       cfg.Datastore,
		Namespace:       cfg.Namespace,
		CertFingerprint: cfg.CertFingerprint,
		BackupID:        backupID,
		SnapshotTime:    time.Unix(snapshotUnix, 0),
	}
	return ReadSnapshotMetaInline(opts, false)
}

// RestoreSnapshot extracts a snapshot (or selected files) according to mode.
//
//   - mode "original": restore in-place to the path captured in the snapshot's
//     .nimbus_backup_meta.json sidecar. destPath is ignored. Cross-host
//     attempts are refused unless allowCrossHost is true.
//   - mode "alternate_abs" (or empty): write to destPath, preserving the full
//     archive directory layout below it.
//   - mode "alternate_flat": write to destPath stripping the longest common
//     prefix of the selection — useful for restoring a single file as
//     destPath/<basename>.
//
// includePaths uses archive-style paths (forward slash). When empty the entire
// snapshot is restored. The ACL/ADS/timestamps flags are accepted today but
// only timestamps is effective — the per-file NTFS sidecar required for the
// other two is still on the roadmap.
//
// Progress is streamed to the frontend via the "restore:progress" event;
// completion via "restore:complete".
func (a *App) RestoreSnapshot(pbsID, backupID, snapshotID, destPath, mode string,
	includePaths []string, allowCrossHost, restoreACLs, restoreADS, restoreTimestamps, overwrite bool) error {
	writeDebugLog(fmt.Sprintf("RestoreSnapshot(pbs=%s, backupID=%s, snap=%s, mode=%s, dest=%s, includes=%d, crossHost=%v, acl=%v, ads=%v, ts=%v, overwrite=%v)",
		pbsID, backupID, snapshotID, mode, destPath, len(includePaths), allowCrossHost, restoreACLs, restoreADS, restoreTimestamps, overwrite))

	cfg, err := a.resolveRestorePBS(pbsID)
	if err != nil {
		return err
	}
	if backupID == "" {
		return fmt.Errorf("backup ID requis")
	}
	if snapshotID == "" {
		return fmt.Errorf("ID du snapshot requis")
	}

	restoreMode := RestoreMode(mode)
	if restoreMode == "" {
		restoreMode = RestoreModeAlternateAbs
	}

	// Destination is only required + validated for alternate modes. In-place
	// derives the target from the backup metadata sidecar.
	if restoreMode != RestoreModeOriginal {
		if destPath == "" {
			return fmt.Errorf("chemin de destination requis")
		}
		if err := security.ValidatePath(destPath); err != nil {
			return fmt.Errorf("chemin de destination invalide: %w", err)
		}
	}

	timestamp, err := time.Parse("2006-01-02T15:04:05Z", snapshotID)
	if err != nil {
		return fmt.Errorf("ID de snapshot invalide: %v", err)
	}

	emit := func(percent float64, message string) {
		if a.ctx == nil {
			return
		}
		runtime.EventsEmit(a.ctx, "restore:progress", map[string]interface{}{
			"percent": percent,
			"message": message,
		})
	}

	opts := RestoreOptions{
		BaseURL:           cfg.BaseURL,
		AuthID:            cfg.AuthID,
		Secret:            cfg.Secret,
		Datastore:         cfg.Datastore,
		Namespace:         cfg.Namespace,
		CertFingerprint:   cfg.CertFingerprint,
		BackupID:          backupID,
		SnapshotTime:      timestamp,
		DestPath:          destPath,
		Mode:              restoreMode,
		AllowCrossHost:    allowCrossHost,
		IncludePaths:      includePaths,
		Overwrite:         overwrite,
		RestoreACLs:       restoreACLs,
		RestoreADS:        restoreADS,
		RestoreTimestamps: restoreTimestamps,
		OnProgress:        emit,
	}

	go func() {
		// A restore can fail in surprising ways (corrupt archive, disk full).
		// Recover so a panic surfaces as an error in the UI instead of taking
		// the whole GUI process down.
		var err error
		func() {
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("restore panic: %v", r)
					writeDebugLog(fmt.Sprintf("CRITICAL: restore panic: %v\n%s", r, debug.Stack()))
				}
			}()
			err = RestoreSnapshotInline(opts)
		}()
		success := err == nil
		msg := "Restauration terminée"
		if err != nil {
			msg = err.Error()
			writeDebugLog(fmt.Sprintf("Restore failed: %v", err))
		}
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "restore:complete", map[string]interface{}{
				"success": success,
				"message": msg,
			})
		}
	}()
	return nil
}

// OpenRestoreDestDialog opens a native folder picker so the user can choose
// where to restore files. Returns "" if the dialog was cancelled.
//
// Hardened against a reported crash on the client: the native Windows folder
// picker (IFileDialog) can fault when handed an empty/invalid initial folder,
// so we seed DefaultDirectory with a path we know exists. A recover() turns any
// Go-level panic into an error instead of taking the process down, and the
// surrounding logging makes the next failure diagnosable from the debug log.
func (a *App) OpenRestoreDestDialog() (dir string, err error) {
	if a.ctx == nil {
		return "", fmt.Errorf("runtime non disponible")
	}
	// Only the headless service process (session 0, LocalSystem, no interactive
	// desktop) crashes on the native folder picker — a native COM fault that
	// recover() cannot catch. The interactive GUI process opens it safely and
	// hands the chosen path to the service, so gate on isServiceProcess, NOT on
	// a.mode (which is also ModeService in the GUI whenever a service exists —
	// the previous guard disabled the picker for every GUI user with a service).
	if a.isServiceProcess {
		writeDebugLog("OpenRestoreDestDialog: native picker skipped in the headless service process — use manual path entry")
		return "", fmt.Errorf("sélecteur de dossier indisponible dans le service — saisissez le chemin de destination manuellement")
	}
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("folder picker panic: %v", r)
			writeDebugLog(fmt.Sprintf("CRITICAL: OpenRestoreDestDialog panic: %v\n%s", r, debug.Stack()))
		}
	}()

	// Seed the dialog with a folder that is guaranteed to exist. An empty or
	// stale DefaultDirectory is a known trigger for native dialog crashes.
	defaultDir, herr := os.UserHomeDir()
	if herr != nil || defaultDir == "" {
		defaultDir = os.TempDir()
	}

	writeDebugLog(fmt.Sprintf("OpenRestoreDestDialog: opening folder picker (default=%s)", defaultDir))
	dir, err = runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title:            "Choisir le dossier de destination",
		DefaultDirectory: defaultDir,
	})
	writeDebugLog(fmt.Sprintf("OpenRestoreDestDialog: returned dir=%q err=%v", dir, err))
	return dir, err
}

// SearchFiles scans every backup-id matching hostPrefix over the given period
// for entries matching query, and returns the matches. mode is one of "name"
// (substring on file name), "regex", or "path" (substring on full path).
//
// fromUnix/toUnix bound the snapshot period in Unix seconds; pass 0 for an
// open end. When assembleMissing is true, snapshots not already in the local
// listing cache are downloaded + assembled (slow, needs temp space); otherwise
// only cached snapshots are searched. Progress is streamed via "search:progress".
func (a *App) SearchFiles(pbsID, hostPrefix, query, mode string, fromUnix, toUnix int64, assembleMissing bool) (*SearchResult, error) {
	writeDebugLog(fmt.Sprintf("SearchFiles(pbs=%s, prefix=%s, query=%q, mode=%s, from=%d, to=%d, assemble=%v)",
		pbsID, hostPrefix, query, mode, fromUnix, toUnix, assembleMissing))

	cfg, err := a.resolveRestorePBS(pbsID)
	if err != nil {
		return nil, err
	}

	var from, to time.Time
	if fromUnix > 0 {
		from = time.Unix(fromUnix, 0)
	}
	if toUnix > 0 {
		to = time.Unix(toUnix, 0)
	}

	emit := func(percent float64, message string) {
		if a.ctx == nil {
			return
		}
		runtime.EventsEmit(a.ctx, "search:progress", map[string]interface{}{
			"percent": percent,
			"message": message,
		})
	}

	opts := SearchOptions{
		BaseURL:         cfg.BaseURL,
		AuthID:          cfg.AuthID,
		Secret:          cfg.Secret,
		Datastore:       cfg.Datastore,
		Namespace:       cfg.Namespace,
		CertFingerprint: cfg.CertFingerprint,
		HostPrefix:      hostPrefix,
		Query:           query,
		Mode:            SearchMatchMode(mode),
		From:            from,
		To:              to,
		AssembleMissing: assembleMissing,
		OnProgress:      emit,
	}
	return SearchFilesInline(opts)
}

// CancelSearch asks an in-flight SearchFiles to stop at the next snapshot
// boundary. The call returning does not mean the search has stopped yet — the
// search returns its partial result with Cancelled=true.
func (a *App) CancelSearch() {
	writeDebugLog("CancelSearch requested")
	CancelFileSearch()
}
