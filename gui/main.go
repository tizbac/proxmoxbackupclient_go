//go:build !service
// +build !service

package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"os"
	"os/exec"
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
	appName = "Proxmox Backup Client"
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
	// Elevated token-fetch child: this process was launched elevated ONLY to
	// read the service token file and hand it back (see token_elevated.go).
	// It must run before flag parsing, single-instance and everything else —
	// the child never starts the GUI.
	if handleElevatedTokenFetchChild(os.Args[1:]) {
		return
	}

	// Parse command line flags
	minimized := flag.Bool("minimized", false, "Start minimized to system tray")
	forceStandalone := flag.Bool("standalone", false, "Force standalone mode (do not connect to the local service)")
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

	// Resolve the execution mode BEFORE building the app: it decides where
	// the configuration lives. In service mode the service owns the
	// privileged state dir (config.json & co. are service-only readable);
	// in standalone mode the GUI keeps everything in the user's home.
	execMode, standaloneReason := resolveExecutionMode(*forceStandalone)
	// A service token that expires or is rotated mid-session must be renewed
	// with the same elevated fetch that got it at launch, instead of turning
	// every call into a 401. Installed here (and on the late switch to service
	// mode) so the launch probe above still sees the raw 401 and can fall back
	// to standalone when the prompt is declined.
	if execMode == api.ModeService {
		installTokenRefreshHook()
	}
	switch execMode {
	case api.ModeService:
		SetConfigDir(serviceStateDir())
	default:
		SetConfigDir(standaloneConfigDir())
		migrateStandaloneFromProgramData()
	}
	writeDebugLog(fmt.Sprintf("Execution mode: %s (standalone reason: %q)", execMode.String(), standaloneReason))

	// Create app instance
	app := NewApp()
	app.mode = execMode
	app.standaloneReason = standaloneReason
	writeDebugLog("App instance created")

	// Service mode: config.json lives in a root-only state directory (0700 via
	// systemd StateDirectoryMode=), so LoadConfig() inside NewApp() could not
	// read it — its result is empty. Fetch the sanitized document from the
	// service BEFORE the window opens: the frontend asks for the server list on
	// mount, and an async hydration would race it (saved servers "not showing
	// up" after a restart of the GUI).
	if execMode == api.ModeService {
		app.hydrateFromService()
	}

	// Create application options
	appOptions := &options.App{
		Title:     fmt.Sprintf("%s v%s", BrandFromExecutable().Title, appVersion),
		Width:     1200,
		Height:    840,
		MaxWidth:  1680, // Prevent window from being too large
		MaxHeight: 1008, // Prevent title bar from going off-screen
		MinWidth:  480,  // Allow very small windows for low-res screens
		MinHeight: 360,  // Allow very small windows for low-res screens
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
			WebviewUserDataPath:  filepath.Join(os.Getenv("APPDATA"), "ProxmoxBackupClient"),
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

// resolveExecutionMode probes the local service and decides how this GUI
// process runs:
//
//	HTTP 200    -> service mode (the token presented is accepted)
//	HTTP 401    -> service running, token missing: ONE elevated token fetch,
//	               then re-probe; any failure falls back to standalone
//	unreachable -> standalone (no service installed/started)
//
// The --standalone flag forces standalone unconditionally.
// PBSGO_TOKEN_FETCH_FAILED=1 (set by the package launcher after its own
// failed fetch attempt) suppresses the in-GUI elevation prompt so the user
// is never asked twice for the same token.
func resolveExecutionMode(forceStandalone bool) (api.ExecutionMode, string) {
	return resolveExecutionModeWith(forceStandalone,
		api.NewModeDetector(getAPITokenPath()).Probe,
		elevatedFetchTokenWithHandoff)
}

// resolveExecutionModeWith is resolveExecutionMode with the probe and the
// elevated fetch injected, so the decision table can be tested without a
// running service or a real pkexec/UAC prompt.
func resolveExecutionModeWith(forceStandalone bool, probe func() int, fetchToken func() (string, error)) (api.ExecutionMode, string) {
	if forceStandalone {
		writeDebugLog("Standalone mode forced by --standalone flag")
		return api.ModeStandalone, "forced"
	}

	switch probe() {
	case 200:
		writeDebugLog("Local service is running and accepted the token")
		return api.ModeService, ""
	case 401:
		if os.Getenv("PBSGO_TOKEN_FETCH_FAILED") != "" {
			writeDebugLog("Service is running but the token is missing and the launcher already attempted an elevated fetch (PBSGO_TOKEN_FETCH_FAILED): not prompting again")
			return api.ModeStandalone, "auth_failed"
		}
		writeDebugLog("Service is running but the token is missing: attempting one elevated token fetch")
		token, err := fetchToken()
		if err != nil {
			writeDebugLog(fmt.Sprintf("Elevated token fetch failed: %v — falling back to standalone", err))
			return api.ModeStandalone, "auth_failed"
		}
		// Keep the token in memory only; the root-owned file is never
		// copied to a user-readable location.
		api.SetTokenOverride(token)
		if probe() == 200 {
			writeDebugLog("Token acquired via elevated fetch; using service mode")
			return api.ModeService, ""
		}
		writeDebugLog("Token acquired but still rejected; falling back to standalone")
		// A token the service refuses must not linger: every later request
		// would keep presenting it instead of asking for a fresh one.
		api.SetTokenOverride("")
		return api.ModeStandalone, "auth_failed"
	default:
		writeDebugLog("Local service not reachable: standalone mode")
		return api.ModeStandalone, "no_service"
	}
}

func writeCrashReport(message string) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")

	crashContent := fmt.Sprintf(`=== PROXMOX BACKUP CLIENT CRASH REPORT ===
Time: %s
Version: %s

%s

=== SYSTEM INFO ===
Service Log: %s
Backup Log: %s

Please report this issue to the Proxmox Backup Client project:
- Register the issue at https://github.com/tizbac/proxmoxbackupclient_go/issues
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

// ListBackupJobs returns a list of all running/completed backup jobs from the service.
func (a *App) ListBackupJobs() ([]*api.BackupProgress, error) {
	if a.apiClient == nil {
		return nil, fmt.Errorf("no API client available")
	}
	return a.apiClient.ListBackupJobs()
}

// startup is called when the app starts
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	writeDebugLog("App.startup() called")

	// The execution mode was resolved in main() BEFORE the app was built —
	// it determines where the configuration lives, so it cannot change once
	// the config is loaded. If the service appears later, StartBackup()
	// re-detects and switches to service mode lazily.
	writeDebugLog(fmt.Sprintf("Execution mode: %s", a.mode.String()))

	// Standalone mode: one-shot backups/restores work against the user's
	// home config, but SCHEDULING requires the service, so the local
	// scheduler is NOT started. The frontend shows a persistent notice and
	// disables the scheduling UI. In service mode the scheduler runs in
	// the service.
	if a.mode == api.ModeStandalone {
		// Cleanup any abandoned "running" jobs from previous session
		a.CleanupAbandonedJobs()

		// Clear any orphaned VSS shadow copies and reset the VSS service
		// state from a previously crashed backup process. Without this, the
		// next backup can fail with "shadow copy creation is already in
		// progress". No-op on non-Windows platforms.
		if err := snapshot.VSSCleanup(); err != nil {
			writeDebugLog(fmt.Sprintf("VSS cleanup at startup reported error: %v", err))
		}

		writeDebugLog("Standalone mode: local scheduler NOT started (scheduling requires the service)")
	} else {
		writeDebugLog("Service mode - scheduler runs in service")
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

// beforeClose is called when the application is about to quit.
func (a *App) beforeClose(ctx context.Context) (prevent bool) {
	// Only Windows has a tray that keeps the app alive, so only there do we
	// swallow the close and hide the window. On other platforms the tray is a
	// no-op, so we must let the window close or the app can never be quit.
	if !a.preventCloseToTray() {
		writeDebugLog("App.beforeClose() called - allowing close (no tray on this platform)")
		return false
	}
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

// needsLocalElevation reports whether THIS process has to be root/admin to do
// privileged work (raw block devices and the kernel snapshot modules for
// machine backups, VSS snapshots) — and, on Windows, simply to LIST the disks.
// The rule is per platform, because what the GUI does with the disks differs:
//
//   - Windows: the GUI opens \\.\PhysicalDriveN itself to list and preview the
//     disks, and that needs an elevated token — even when the backup would be
//     executed by the service (the service can do everything else, it cannot
//     hand the GUI a disk list it has no rights to open).
//   - Linux: enumerating disks (sysfs) is unprivileged; only OPENING the
//     devices for the backup needs root, which the service provides in service
//     mode — so an unprivileged GUI is fine there.
//
// It is false when this process already is the privileged helper. The frontend
// must not ask such a GUI to relaunch with pkexec — that is what made
// "machine backup requires admin privileges" show up in service mode.
func (a *App) needsLocalElevation() bool {
	return needsElevationFor(stdruntime.GOOS, isAdmin(), a.isServiceProcess, a.mode)
}

// needsElevationFor is the pure decision rule behind needsLocalElevation,
// split out so every platform combination can be tested on any host.
func needsElevationFor(goos string, admin, serviceProcess bool, mode api.ExecutionMode) bool {
	if serviceProcess {
		return false // we ARE the privileged helper
	}
	if admin {
		return false // already elevated
	}
	if goos == "windows" {
		return true // disk listing alone demands it
	}
	return mode != api.ModeService
}

// switchToServiceMode flips the runtime mode after the late service
// re-detection (the service may start after the GUI) and tells the frontend,
// so everything derived from it — elevation warnings, the mode badge, the VSS
// notice — updates instead of keeping the value read at startup.
func (a *App) switchToServiceMode() {
	if a.mode == api.ModeService {
		return
	}
	a.mode = api.ModeService
	// The "why am I standalone" reason is obsolete the moment the service
	// takes over; leaving it set makes the frontend notice contradict itself.
	a.standaloneReason = ""
	// Now that this GUI talks to the service, its token can be rotated away
	// under it: refresh through an elevated fetch instead of failing.
	installTokenRefreshHook()
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, "mode:changed", a.GetSystemInfo())
}

// GetSystemInfo returns system information for UI (mode, admin status, etc.)
func (a *App) GetSystemInfo() map[string]interface{} {
	info := map[string]interface{}{
		"mode":     a.mode.String(),
		"is_admin": isAdmin(),
		// needs_local_elevation: see needsLocalElevation. The frontend uses it
		// (not is_admin) to decide whether to demand an elevated relaunch.
		"needs_local_elevation": a.needsLocalElevation(),
		"hostname":              a.GetHostname(),
		"service_available":     a.mode == api.ModeService,
		// standalone + standalone_reason drive the persistent top notice in
		// the frontend ("running standalone, scheduling not available") and
		// the disabling of the scheduling UI.
		"standalone":        a.mode == api.ModeStandalone,
		"standalone_reason": a.standaloneReason, // "forced" | "no_service" | "auth_failed" | ""
		// os = runtime.GOOS ("windows", "linux", "darwin") — used by the
		// restore UI to enable/disable the in-place mode when the snapshot
		// was taken on a different platform.
		"os": stdruntime.GOOS,
	}

	// On Linux, add snapshot module info for machine backups
	if stdruntime.GOOS == "linux" {
		if module := getSnapshotModule(); module != "" {
			info["snapshot_module"] = module
		}
	}

	return info
}

// getSnapshotModule detects which Linux block snapshot kernel module is available.
// Returns "elastio-snap", "dattobd", or empty string if neither is loaded.
// This function is only available on Linux due to build tags in snapshot package.
func getSnapshotModule() string {
	if stdruntime.GOOS != "linux" {
		return ""
	}
	if control, ok := snapshot.DetectControl(); ok {
		return control.Name
	}
	return ""
}

// RequestElevation re-launches the application with elevated privileges.
// On Linux this uses pkexec or sudo. On Windows it uses the Wails "RunAsAdmin" mechanism.
// On macOS it uses osascript with administrator privileges.
// Returns an error if elevation cannot be requested or was denied.
func (a *App) RequestElevation() error {
	// Nothing to elevate when this process already has everything the job
	// needs: in service mode on Linux the service runs the backup as root, so
	// relaunching this unprivileged GUI with pkexec/sudo would only spawn a
	// second copy of the same unprivileged process. On Windows the rule says
	// otherwise (the GUI must open the disks itself to list them), so the
	// relaunch happens there.
	if !a.needsLocalElevation() {
		writeDebugLog("RequestElevation: ignored (no local elevation needed)")
		return nil
	}
	switch stdruntime.GOOS {
	case "linux":
		return requestElevationLinux()
	case "windows":
		return requestElevationWindows()
	case "darwin":
		return requestElevationDarwin()
	default:
		return fmt.Errorf("elevation not supported on %s", stdruntime.GOOS)
	}
}

// CanModifyJobs returns true if the current user has permission to modify
// scheduled jobs. On Windows, this requires running as administrator (UAC elevated).
// On Linux, this requires being in the wheel/sudo group or having sudo privileges.
//
// In service mode neither applies: the privileged service owns the jobs file
// and the GUI saves through its API, so an unprivileged GUI may edit jobs
// without relaunching elevated.
func (a *App) CanModifyJobs() bool {
	if a.isDelegatedToService() {
		return true
	}
	return canModifyJobs()
}

// RequestJobModificationElevation requests elevation specifically for job modification.
// On Windows, this uses UAC. On Linux, this uses pkexec or sudo with a prompt for credentials.
func (a *App) RequestJobModificationElevation() error {
	return a.RequestElevation()
}

// requestElevationLinux re-launches the current executable with pkexec or sudo
func requestElevationLinux() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot determine executable path: %w", err)
	}

	// Try pkexec first (policykit), then sudo
	for _, cmd := range [][]string{
		{"pkexec", exe},
		{"sudo", exe},
	} {
		// Check if the command exists
		if _, err := exec.LookPath(cmd[0]); err == nil {
			// Launch detached so the current process can exit
			return exec.Command(cmd[0], cmd[1:]...).Start()
		}
	}
	return fmt.Errorf("neither pkexec nor sudo found; cannot elevate")
}

// requestElevationDarwin uses osascript to request admin privileges
func requestElevationDarwin() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot determine executable path: %w", err)
	}
	return exec.Command("osascript", "-e", fmt.Sprintf(`do shell script "%s" with administrator privileges`, exe)).Start()
}

func (a *App) GetVersion() string {
	writeDebugLog(fmt.Sprintf("GetVersion() returned: %s", appVersion))
	return appVersion
}

// ListPhysicalDisks returns a list of available physical disks
func (a *App) ListPhysicalDisks() ([]PhysicalDiskInfo, error) {
	writeDebugLog("ListPhysicalDisks() called from frontend")

	// Call platform-specific disk listing function
	disks, err := listPhysicalDisks()
	if err != nil {
		writeDebugLog(fmt.Sprintf("Error listing physical disks: %v", err))
		return nil, err
	}

	writeDebugLog(fmt.Sprintf("Found %d physical disks", len(disks)))
	return disks, nil
}

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
		"secret":     "",
		"secret_set": cfg.Secret != "",
		"datastore":  cfg.Datastore,
		"namespace":  cfg.Namespace,
		// Unlike Secret, the encryption key path is not a secret (only the path
		// is stored; the unlocked key never leaves the backend), so it is handed
		// to the frontend as-is. SaveConfig replaces the whole Config, so the
		// frontend MUST round-trip this field or every config save would silently
		// drop the key and restart encrypting with no way to read it back.
		"encryption_key_file": cfg.EncryptionKeyFile,
		"backupdir":           cfg.BackupDir,
		"backup-id":           cfg.BackupID,
		"usevss":              cfg.UseVSS,
		"hostname":            hostname,
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
		"config_path":      configPath,
		"baseurl_set":      cfg.BaseURL != "",
		"baseurl_value":    security.SanitizeURL(cfg.BaseURL),
		"authid_set":       cfg.AuthID != "",
		"datastore_set":    cfg.Datastore != "",
		"validation_ok":    validationError == "",
		"validation_error": validationError,
		"mode":             a.mode.String(),
	}
}

// hydrateFromService replaces the in-memory config with the sanitized document
// the service holds. In service mode the config files are root/SYSTEM-only, so
// the service API — not the local file — is the only source the GUI can read:
// startup, ReloadConfig and a service-mode save all go through here. Returns
// false when the service could not be reached, leaving the current config
// untouched.
func (a *App) hydrateFromService() bool {
	if a.apiClient == nil {
		return false
	}
	doc, err := a.apiClient.GetFullConfig()
	if err != nil {
		writeDebugLog(fmt.Sprintf("hydrateFromService: %v", err))
		return false
	}
	a.config = parseFullConfig(doc)
	writeDebugLog(fmt.Sprintf("Config hydrated from the service (%d PBS server(s))", len(a.config.PBSServers)))
	return true
}

// SaveConfig saves the configuration submitted by the legacy settings form.
//
// The form carries only the legacy top-level fields (the incoming value has a
// nil PBS server map), so it is MERGED over the stored config: assigning it
// directly would silently drop every saved server and the default server id.
// An empty secret/password keeps the stored one (M-04).
func (a *App) SaveConfig(config *Config) error {
	merged := a.config
	if a.config != nil && config != nil {
		next := *a.config
		next.BaseURL = config.BaseURL
		next.CertFingerprint = config.CertFingerprint
		next.AuthID = config.AuthID
		next.Datastore = config.Datastore
		next.Namespace = config.Namespace
		next.EncryptionKeyFile = config.EncryptionKeyFile
		next.BackupDir = config.BackupDir
		next.BackupID = config.BackupID
		next.UseVSS = config.UseVSS
		// M-04: the frontend never receives the stored secrets, so an empty
		// value means "keep the existing one", not "clear it".
		if config.Secret != "" {
			next.Secret = config.Secret
		}
		if config.SMTPPassword != "" {
			next.SMTPPassword = config.SMTPPassword
		}
		if next.AuthID != "" {
			// One legacy method only, like every other save path.
			next.Username = ""
			next.Password = ""
		}
		merged = &next
	}

	// Log sanitized config (no secrets)
	if merged != nil {
		writeDebugLog(fmt.Sprintf("SaveConfig() called: URL=%s, AuthID=%s, Datastore=%s, BackupID=%s",
			security.SanitizeURL(merged.BaseURL),
			merged.AuthID,
			merged.Datastore,
			merged.BackupID))
	}

	// In service mode the GUI delegates the write to the service, which owns
	// the privileged config file (root/SYSTEM-only) and re-validates the whole
	// document (including the per-server credentials) before storing it.
	if a.isDelegatedToService() {
		a.config = merged
		return a.pushConfigToService()
	}
	if merged == nil {
		return fmt.Errorf("configuration vide")
	}

	// Validate per part: a multi-PBS-only config has no legacy BaseURL and must
	// not be rejected for missing legacy credentials (they live per server).
	if err := merged.validatePBSFields(); err != nil {
		writeDebugLog(fmt.Sprintf("Config validation failed: %v", err))
		return err
	}

	// Unlock the key BEFORE persisting so the runtime-only Crypt matches what is
	// about to be written, and so an unusable key is reported without having
	// saved a config that cannot be used.
	if err := merged.loadCryptConfig(); err != nil {
		writeDebugLog(fmt.Sprintf("Encryption key load failed: %v", err))
		return err
	}

	// Save to disk
	if err := merged.Save(); err != nil {
		writeDebugLog(fmt.Sprintf("Config save to disk failed: %v", err))
		return err
	}

	// Update in-memory config
	a.config = merged
	writeDebugLog("Config saved successfully and loaded into app")
	return nil
}

// pushConfigToService sends the current config document to the service via
// /config POST. Credentials this GUI knows (typed in the current session) are
// transmitted so the service can store them; empty ones mean "keep existing"
// there — the stored secrets never leave the service.
func (a *App) pushConfigToService() error {
	if a.apiClient == nil {
		return fmt.Errorf("no API client for service mode")
	}
	doc := a.config.fullConfigDocument()
	if err := a.apiClient.SaveFullConfig(doc); err != nil {
		return err
	}
	// Re-fetch the sanitized document so a.config stays in sync with the service
	fetched, err := a.apiClient.GetFullConfig()
	if err != nil {
		return err
	}
	// Rehydrate from the fetched document (same code path as startup in service mode)
	a.config = parseFullConfig(fetched)
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

	// In service mode the GUI delegates the test to the service, which has
	// access to the stored credentials (the GUI never holds them).
	if a.isDelegatedToService() {
		// Convert the Config (or draft) to a map for the API.
		// For per-server tests, the caller passes a config with the server ID
		// in AuthID (legacy) or we need the server ID. We'll pass the draft
		// fields as-is; the API merges non-empty fields over the stored entry.
		draft := configToDraftMap(testConfig)
		return a.apiClient.TestPBSServer("", draft)
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

	// Mint a fresh PBS session ticket for this operation (username/password).
	testConfig, err := a.withAuth(testConfig)
	if err != nil {
		return err
	}

	// Create PBS client
	client := &pbscommon.PBSClient{
		BaseURL:          testConfig.BaseURL,
		CertFingerPrint:  testConfig.CertFingerprint,
		AuthID:           testConfig.AuthID,
		Secret:           testConfig.Secret,
		Ticket:           testConfig.Ticket,
		CSRFToken:        testConfig.CSRFToken,
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

// configToDraftMap extracts the PBS-relevant fields from a Config for the
// /pbs/test endpoint. In service mode the GUI sends a draft (partial) that
// the service merges over the stored server entry.
func configToDraftMap(cfg *Config) map[string]interface{} {
	if cfg == nil {
		return map[string]interface{}{}
	}
	draft := map[string]interface{}{}
	if cfg.BaseURL != "" {
		draft["baseurl"] = cfg.BaseURL
	}
	if cfg.CertFingerprint != "" {
		draft["certfingerprint"] = cfg.CertFingerprint
	}
	if cfg.AuthID != "" {
		draft["authid"] = cfg.AuthID
	}
	if cfg.Secret != "" {
		draft["secret"] = cfg.Secret
	}
	if cfg.Username != "" {
		draft["username"] = cfg.Username
	}
	if cfg.Password != "" {
		draft["password"] = cfg.Password
	}
	if cfg.Datastore != "" {
		draft["datastore"] = cfg.Datastore
	}
	if cfg.Namespace != "" {
		draft["namespace"] = cfg.Namespace
	}
	return draft
}

// GetLastBackupDirs returns the last used backup directories
func (a *App) GetLastBackupDirs() []string {
	writeDebugLog(fmt.Sprintf("GetLastBackupDirs() returned %d directories", len(a.config.LastBackupDirs)))
	return a.config.LastBackupDirs
}

// ReloadConfig reloads the configuration. In service mode the state directory
// is root-only, so reloading from disk would wipe the in-memory config (and
// with it every saved server): the service API is the source of truth there.
func (a *App) ReloadConfig() {
	if a.isDelegatedToService() {
		if a.hydrateFromService() {
			return
		}
		writeDebugLog("ReloadConfig: service unreachable, keeping the in-memory config")
		return
	}
	a.config = LoadConfig()
	writeDebugLog("Config reloaded from disk")
}

// ==================== MULTI-PBS MANAGEMENT ====================

// ListPBSServers returns all configured PBS servers
func (a *App) ListPBSServers() []*PBSServer {
	// An empty list in service mode means either "the user deleted everything"
	// or "startup hydration failed" (service restarted, token rotated…). Ask the
	// service once more instead of reporting an empty list, so saved servers
	// always show up after a GUI restart.
	if a.isDelegatedToService() && len(a.config.PBSServers) == 0 {
		a.hydrateFromService()
	}
	// In service mode the config is hydrated from the service; return it directly.
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

	// In service mode the GUI modifies its in-memory config and then pushes
	// the whole document to the service.
	if a.isDelegatedToService() {
		if err := a.config.AddPBSServerMem(pbs); err != nil {
			return err
		}
		return a.pushConfigToService()
	}
	return a.config.AddPBSServer(pbs)
}

// UpdatePBSServer updates an existing PBS server
func (a *App) UpdatePBSServer(pbs *PBSServer) error {
	writeDebugLog(fmt.Sprintf("UpdatePBSServer(%s) called", pbs.ID))

	// The "empty = keep the stored credential" resolution happens inside
	// UpdatePBSServerMem (normalizeServerAuth): it knows the auth method, so a
	// switch to user/password no longer inherits the old token secret and a
	// changed authid no longer silently reuses the old secret.
	if a.isDelegatedToService() {
		// This process never received the stored secret/password (M-04): an empty
		// value here means "keep", and the service — which holds them — performs
		// the final credential check while saving the pushed document.
		if err := a.config.UpdatePBSServerMem(pbs, false); err != nil {
			return err
		}
		return a.pushConfigToService()
	}
	return a.config.UpdatePBSServer(pbs)
}

// DeletePBSServer removes a PBS server
func (a *App) DeletePBSServer(id string) error {
	writeDebugLog(fmt.Sprintf("DeletePBSServer(%s) called", id))

	if a.isDelegatedToService() {
		if err := a.config.DeletePBSServerMem(id); err != nil {
			return err
		}
		return a.pushConfigToService()
	}
	return a.config.DeletePBSServer(id)
}

// SetDefaultPBSServer sets the default PBS server
func (a *App) SetDefaultPBSServer(id string) error {
	writeDebugLog(fmt.Sprintf("SetDefaultPBSServer(%s) called", id))

	if a.isDelegatedToService() {
		if err := a.config.SetDefaultPBSMem(id); err != nil {
			return err
		}
		return a.pushConfigToService()
	}
	return a.config.SetDefaultPBS(id)
}

// GetDefaultPBSID returns the default PBS server ID
func (a *App) GetDefaultPBSID() string {
	return a.config.DefaultPBSID
}

// TestPBSConnection tests connection to a specific PBS server
func (a *App) TestPBSConnection(pbsID string) error {
	writeDebugLog(fmt.Sprintf("TestPBSConnection(%s) called", pbsID))

	// In service mode the GUI delegates to the service which has the credentials.
	if a.isDelegatedToService() {
		return a.apiClient.TestPBSServer(pbsID, map[string]interface{}{})
	}

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
func (a *App) StartBackup(backupType string, backupDirs []string, driveLetters []string, excludeList []string, backupID string, useVSS bool, compression string, pbsID string) error {
	writeDebugLog(fmt.Sprintf("StartBackup() called - mode: %s, VSS: %v, compression: %s, pbsID: %s, isServiceProcess: %v", a.mode.String(), useVSS, compression, pbsID, a.isServiceProcess))

	// Default to "fastest" if compression is empty
	if compression == "" {
		compression = "fastest"
		writeDebugLog("[Compression] Using default: fastest")
	}

	// Re-detect mode if currently Standalone (service may have started after GUI)
	// IMPORTANT: Never re-detect if we ARE the service process (prevents
	// infinite loop), and never when --standalone said "don't connect": the
	// flag has to stay honoured for the whole session, otherwise the GUI would
	// quietly join a service the user asked it to ignore.
	if !a.isServiceProcess && a.mode == api.ModeStandalone && a.standaloneReason != "forced" {
		if a.apiClient.IsServiceAvailable() {
			writeDebugLog("[Mode Detection] Service now available, switching to Service mode")
			a.switchToServiceMode()
		}
	}

	// Route based on execution mode
	switch a.mode {
	case api.ModeService:
		// Use HTTP API to communicate with service (service has admin rights as LocalSystem)
		return a.startBackupViaService(backupType, backupDirs, driveLetters, excludeList, backupID, useVSS, compression, pbsID)
	case api.ModeStandalone:
		// Direct execution - check admin if VSS requested
		if useVSS && !isAdmin() {
			return fmt.Errorf("VSS (Shadow Copy) nécessite les privilèges administrateur - veuillez redémarrer l'application en tant qu'administrateur ou désactiver VSS")
		}
		return a.startBackupDirect(backupType, backupDirs, driveLetters, excludeList, backupID, useVSS, compression, pbsID)
	default:
		return fmt.Errorf("unknown execution mode: %v", a.mode)
	}
}

// StartMachineBackup starts a machine backup operation
func (a *App) StartMachineBackup(backupType string, backupDevices []string, backupID string, useVSS bool, compression string, pbsID string, backupKind string) error {
	writeDebugLog(fmt.Sprintf("StartMachineBackup() called - mode: %s, VSS: %v, compression: %s, pbsID: %s, backupKind: %s, isServiceProcess: %v", a.mode.String(), useVSS, compression, pbsID, backupKind, a.isServiceProcess))

	// Default to "fastest" if compression is empty
	if compression == "" {
		compression = "fastest"
		writeDebugLog("[Compression] Using default: fastest")
	}

	// Re-detect mode if currently Standalone (service may have started after GUI)
	// IMPORTANT: Never re-detect if we ARE the service process (prevents
	// infinite loop), and never when --standalone said "don't connect": the
	// flag has to stay honoured for the whole session, otherwise the GUI would
	// quietly join a service the user asked it to ignore.
	if !a.isServiceProcess && a.mode == api.ModeStandalone && a.standaloneReason != "forced" {
		if a.apiClient.IsServiceAvailable() {
			writeDebugLog("[Mode Detection] Service now available, switching to Service mode")
			a.switchToServiceMode()
		}
	}

	// Route based on execution mode
	switch a.mode {
	case api.ModeService:
		// Use HTTP API to communicate with service (service has admin rights as LocalSystem)
		return a.startMachineBackupViaService(backupType, backupDevices, backupID, useVSS, compression, pbsID, backupKind)
	case api.ModeStandalone:
		// Direct execution - check admin if VSS requested
		if useVSS && !isAdmin() {
			return fmt.Errorf("VSS (Shadow Copy) nécessite les privilèges administrateur - veuillez redémarrer l'application en tant qu'administrateur ou désactiver VSS")
		}
		return a.startMachineBackupDirect(backupType, backupDevices, backupID, useVSS, compression, pbsID, backupKind)
	default:
		return fmt.Errorf("unknown execution mode: %v", a.mode)
	}
}

// startBackupViaService sends backup request to the service via HTTP API
func (a *App) startBackupViaService(backupType string, backupDirs []string, driveLetters []string, excludeList []string, backupID string, useVSS bool, compression string, pbsID string) error {
	writeDebugLog("[Service Mode] Sending backup request to service")

	req := &api.BackupRequest{
		BackupType:   backupType,
		BackupID:     backupID,
		BackupDirs:   backupDirs,
		DriveLetters: driveLetters,
		ExcludeList:  excludeList,
		UseVSS:       useVSS,
		Compression:  compression,
		PBSID:        pbsID,
	}

	resp, err := a.apiClient.StartBackup(req)
	if err != nil {
		writeDebugLog(fmt.Sprintf("[Service Mode] Backup request failed: %v", err))
		return fmt.Errorf("échec de la communication avec le service: %w", err)
	}

	writeDebugLog(fmt.Sprintf("[Service Mode] Backup started: %s (JobID: %s)", resp.Message, resp.JobID))

	// Remember the job so the Stop button (which calls CancelBackup with no id)
	// can cancel this service-side run.
	a.setDelegatedJobID(resp.JobID)

	// Start polling for progress updates
	go a.pollBackupProgress(resp.JobID)

	return nil
}

// startMachineBackupViaService sends machine backup request to the service via HTTP API
func (a *App) startMachineBackupViaService(backupType string, backupDevices []string, backupID string, useVSS bool, compression string, pbsID string, backupKind string) error {
	writeDebugLog("[Service Mode] Sending machine backup request to service")

	req := &api.BackupRequest{
		BackupType:   backupType,
		BackupID:     backupID,
		DriveLetters: backupDevices,
		UseVSS:       useVSS,
		Compression:  compression,
		PBSID:        pbsID,
		BackupKind:   backupKind,
	}

	resp, err := a.apiClient.StartMachineBackup(req)
	if err != nil {
		writeDebugLog(fmt.Sprintf("[Service Mode] Machine backup request failed: %v", err))
		return fmt.Errorf("échec de la communication avec le service: %w", err)
	}

	writeDebugLog(fmt.Sprintf("[Service Mode] Machine backup started: %s (JobID: %s)", resp.Message, resp.JobID))

	// Remember the job so the Stop button (which calls CancelBackup with no id)
	// can cancel this service-side run.
	a.setDelegatedJobID(resp.JobID)

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
				a.clearDelegatedJobID()
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
			a.clearDelegatedJobID()
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
func (a *App) startBackupDirect(backupType string, backupDirs []string, driveLetters []string, excludeList []string, backupID string, useVSS bool, compression string, pbsID string) error {
	// Use hostname as fallback if backupID is empty
	if backupID == "" {
		backupID = a.GetHostname()
		writeDebugLog(fmt.Sprintf("[Backup ID] Empty backup-id, using hostname: %s", backupID))
	}

	// Sanitize backup ID for logging
	sanitizedID := security.SanitizeForLog(backupID)
	writeDebugLog(fmt.Sprintf("[Standalone Mode] StartBackup: type=%s, id=%s, vss=%v, compression=%s, pbsID=%s, dir_count=%d",
		backupType, sanitizedID, useVSS, compression, pbsID, len(backupDirs)))

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

	// Resolve PBS fields from the specified PBS server (or default), minting a fresh ticket (u/p).
	pbsCfg, err := a.resolveBackupPBS(pbsID)
	if err != nil {
		return err
	}

	// Validate PBS config
	if err := pbsCfg.Validate(); err != nil {
		return err
	}

	// Unlock the configured encryption key, if any, so every chunk this
	// backup uploads is AES-256-GCM and the manifest is signed.
	if err := pbsCfg.loadCryptConfig(); err != nil {
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
		Ticket:          pbsCfg.Ticket,
		CSRFToken:       pbsCfg.CSRFToken,
		Datastore:       pbsCfg.Datastore,
		Namespace:       pbsCfg.Namespace,
		CertFingerprint: pbsCfg.CertFingerprint,
		BackupObjects:   targetDirs,
		BackupID:        backupID,
		BackupType:      "host", // "host" for directory, would be "vm" for machine
		UseVSS:          useVSS,
		Compression:     compression,
		ExcludeList:     excludeList,
		DisableSplit:    a.config.DisableSplit,
		SplitSizeBytes:  a.config.SplitSizeBytes(),
		Crypt:           pbsCfg.Crypt,
		// Use the backup context that was set via SetBackupContext for this job
		Ctx: func() context.Context {
			a.backupCtxMu.RLock()
			defer a.backupCtxMu.RUnlock()
			return a.backupCtx
		}(),
		OnProgress: func(percent float64, message string) {
			writeDebugLog(fmt.Sprintf("Progress: %.1f%% - %s", percent*100, message))

			// Forward to the API server's registered callbacks (service mode).
			// The callback contract is a 0.0-1.0 fraction; the server scales it
			// to 0-100 for its progress map.
			hasCallbacks := a.dispatchProgress(percent, message)

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

			// Forward to the API server's registered callbacks (service mode)
			// and clean them up once the run is terminal.
			hasCallbacks := a.dispatchComplete(success, message)

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
		var err error
		if backupType == "machine" {
			// "host", not "vm": "vm" makes machinebackuplib generate a Proxmox VE
			// VM config, which needs a numeric VMID as the backup ID. The GUI
			// defaults to a hostname-style ID, so a "vm" backup transferred
			// every disk and then failed at that last step. "host" has no such
			// requirement and is the right type for a plain disk backup.
			opts.BackupType = "host"
			err = RunBackupInline(opts)
		} else {
			opts.BackupType = "host"
			err = RunBackupInline(opts)
		}
		if err != nil {
			writeDebugLog(fmt.Sprintf("Backup error: %v", err))
		}
	}()

	return nil
}

// startMachineBackupDirect performs machine backup directly (standalone mode)
func (a *App) startMachineBackupDirect(backupType string, backupDevices []string, backupID string, useVSS bool, compression string, pbsID string, backupKind string) error {
	// Use hostname as fallback if backupID is empty
	if backupID == "" {
		backupID = a.GetHostname()
		writeDebugLog(fmt.Sprintf("[Backup ID] Empty backup-id, using hostname: %s", backupID))
	}

	// Sanitize backup ID for logging
	sanitizedID := security.SanitizeForLog(backupID)
	writeDebugLog(fmt.Sprintf("[Standalone Mode] StartMachineBackup: type=%s, id=%s, vss=%v, compression=%s, pbsID=%s, backupKind=%s, device_count=%d, devs=%v",
		backupType, sanitizedID, useVSS, compression, pbsID, backupKind, len(backupDevices), backupDevices))

	// Validate BackupID (now guaranteed to be non-empty)
	if err := security.ValidateBackupID(backupID); err != nil {
		return fmt.Errorf("backup ID invalide: %w", err)
	}

	// Validate backup devices
	for _, device := range backupDevices {
		if device == "" {
			return fmt.Errorf("one or more devices are empty")
		}
	}

	// Note: Admin check for VSS is done in StartMachineBackup() routing layer
	// If we're here via service, we're already running as LocalSystem

	// Resolve PBS fields from the specified PBS server (or default), minting a fresh ticket (u/p).
	pbsCfg, err := a.resolveBackupPBS(pbsID)
	if err != nil {
		return err
	}

	// Validate PBS config
	if err := pbsCfg.Validate(); err != nil {
		return err
	}

	// Unlock the configured encryption key, if any, so every chunk this
	// backup uploads is AES-256-GCM and the manifest is signed.
	if err := pbsCfg.loadCryptConfig(); err != nil {
		return err
	}

	// Prepare backup options
	opts := BackupOptions{
		BaseURL:         pbsCfg.BaseURL,
		AuthID:          pbsCfg.AuthID,
		Secret:          pbsCfg.Secret,
		Ticket:          pbsCfg.Ticket,
		CSRFToken:       pbsCfg.CSRFToken,
		Datastore:       pbsCfg.Datastore,
		Namespace:       pbsCfg.Namespace,
		CertFingerprint: pbsCfg.CertFingerprint,
		BackupObjects:   backupDevices,
		BackupID:        backupID,
		Kind:            "machine",
		BackupType:      backupKind, // "host" or "vm" based on user selection
		UseVSS:          useVSS,
		Compression:     compression,
		ExcludeList:     []string{}, // No exclude list for machine backups
		DisableSplit:    a.config.DisableSplit,
		SplitSizeBytes:  a.config.SplitSizeBytes(),
		Crypt:           pbsCfg.Crypt,
		// Use the backup context that was set via SetBackupContext for this job
		Ctx: func() context.Context {
			a.backupCtxMu.RLock()
			defer a.backupCtxMu.RUnlock()
			return a.backupCtx
		}(),
		OnProgress: func(percent float64, message string) {
			writeDebugLog(fmt.Sprintf("Progress: %.1f%% - %s", percent*100, message))

			// Forward to the API server's registered callbacks (service mode).
			// The callback contract is a 0.0-1.0 fraction; the server scales it
			// to 0-100 for its progress map.
			hasCallbacks := a.dispatchProgress(percent, message)

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
			writeDebugLog(fmt.Sprintf("Machine backup complete: success=%v, %s", success, message))

			// Forward to the API server's registered callbacks (service mode)
			// and clean them up once the run is terminal.
			hasCallbacks := a.dispatchComplete(success, message)

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
				Name:       fmt.Sprintf("Backup machine - %s", backupID),
				Timestamp:  time.Now().Format(time.RFC3339),
				Status:     "success",
				Message:    message,
				BackupDirs: backupDevices,
				BackupID:   backupID,
				UseVSS:     useVSS,
			}
			if !success {
				historyEntry.Status = "failed"
			}
			if err := a.AddJobHistory(historyEntry); err != nil {
				writeDebugLog(fmt.Sprintf("Warning: Failed to add manual backup to history: %v", err))
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
		err := RunBackupInline(opts)
		if err != nil {
			writeDebugLog(fmt.Sprintf("Machine backup error: %v", err))
		}
	}()

	return nil
}

// ==================== RESTORE ====================

// resolveRestorePBS picks the PBS server to restore from. When pbsID is empty
// the default PBS server is used. Falls back to legacy single-server fields
// when no multi-PBS entry is configured.
func (a *App) resolveRestorePBS(pbsID string) (*Config, error) {
	var cfg *Config

	// In service mode the GUI doesn't hold PBS credentials — it asks the
	// service for a short-lived ticket and uses that for restore/listing.
	if a.isDelegatedToService() {
		ticket, err := a.apiClient.MintPBSTicket(pbsID)
		if err != nil {
			return nil, fmt.Errorf("service failed to mint PBS ticket: %w", err)
		}
		// Build a Config with the ticket + non-secret connection params.
		cfg := &Config{
			BaseURL:         ticket.BaseURL,
			CertFingerprint: ticket.CertFingerprint,
			Datastore:       ticket.Datastore,
			Namespace:       ticket.Namespace,
			Ticket:          ticket.Ticket,
			CSRFToken:       ticket.CSRFToken,
		}
		return cfg, nil
	}

	if pbsID != "" {
		pbs, err := a.config.GetPBSServer(pbsID)
		if err != nil {
			return nil, err
		}
		cfg = pbs.ToConfig()
	} else {
		effective := a.config.EffectivePBS()
		if err := effective.Validate(); err != nil {
			return nil, err
		}
		cfg = effective
	}
	cfg, err := a.withAuth(cfg)
	if err != nil {
		return nil, err
	}
	// An encrypted snapshot's chunks are unreadable without the key, so fail
	// here with a clear message rather than deep inside the chunk fetcher.
	if err := cfg.loadCryptConfig(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// resolveBackupPBS picks the PBS server to use for backup operations.
// When pbsID is empty the default PBS server is used.
// Falls back to legacy single-server fields when no multi-PBS entry is configured.
func (a *App) resolveBackupPBS(pbsID string) (*Config, error) {
	var cfg *Config

	// In service mode the GUI doesn't hold PBS credentials — it asks the
	// service for a short-lived ticket and uses that for backup.
	if a.isDelegatedToService() {
		ticket, err := a.apiClient.MintPBSTicket(pbsID)
		if err != nil {
			return nil, fmt.Errorf("service failed to mint PBS ticket: %w", err)
		}
		// Build a Config with the ticket + non-secret connection params.
		cfg := &Config{
			BaseURL:         ticket.BaseURL,
			CertFingerprint: ticket.CertFingerprint,
			Datastore:       ticket.Datastore,
			Namespace:       ticket.Namespace,
			Ticket:          ticket.Ticket,
			CSRFToken:       ticket.CSRFToken,
		}
		return cfg, nil
	}

	if pbsID != "" {
		pbs, err := a.config.GetPBSServer(pbsID)
		if err != nil {
			return nil, err
		}
		cfg = pbs.ToConfig()
	} else {
		effective := a.config.EffectivePBS()
		if err := effective.Validate(); err != nil {
			return nil, err
		}
		cfg = effective
	}
	cfg, err := a.withAuth(cfg)
	if err != nil {
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

	snaps, err := ListSnapshotsInline(cfg.BaseURL, cfg.AuthID, cfg.Secret, cfg.Ticket, cfg.CSRFToken,
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
		Ticket:          cfg.Ticket,
		CSRFToken:       cfg.CSRFToken,
		Datastore:       cfg.Datastore,
		Namespace:       cfg.Namespace,
		CertFingerprint: cfg.CertFingerprint,
		BackupID:        backupID,
		SnapshotTime:    time.Unix(snapshotUnix, 0),
		Crypt:           cfg.Crypt,
	}
	return ListSnapshotContentsInline(opts, "", forceRefresh)
}

// GetSnapshotMeta returns the `.proxmox_backup_client_meta.json` sidecar from a
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
		Ticket:          cfg.Ticket,
		CSRFToken:       cfg.CSRFToken,
		Datastore:       cfg.Datastore,
		Namespace:       cfg.Namespace,
		CertFingerprint: cfg.CertFingerprint,
		BackupID:        backupID,
		SnapshotTime:    time.Unix(snapshotUnix, 0),
		Crypt:           cfg.Crypt,
	}
	return ReadSnapshotMetaInline(opts, false)
}

// RestoreSnapshot extracts a snapshot (or selected files) according to mode.
//
//   - mode "original": restore in-place to the path captured in the snapshot's
//     .proxmox_backup_client_meta.json sidecar. destPath is ignored. Cross-host
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
		Ticket:            cfg.Ticket,
		CSRFToken:         cfg.CSRFToken,
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
		Crypt:             cfg.Crypt,
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
func (a *App) OpenDirectoryPicker() (dir string, err error) {
	if a.ctx == nil {
		return "", fmt.Errorf("runtime non disponible")
	}
	if a.isServiceProcess {
		writeDebugLog("OpenDirectoryPicker: native picker skipped in the headless service process")
		return "", fmt.Errorf("sélecteur de dossier indisponible dans le service")
	}
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("folder picker panic: %v", r)
			writeDebugLog(fmt.Sprintf("CRITICAL: OpenDirectoryPicker panic: %v\n%s", r, debug.Stack()))
		}
	}()

	defaultDir, herr := os.UserHomeDir()
	if herr != nil || defaultDir == "" {
		defaultDir = os.TempDir()
	}

	writeDebugLog(fmt.Sprintf("OpenDirectoryPicker: opening folder picker (default=%s)", defaultDir))
	dir, err = runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title:            "Choisir un dossier",
		DefaultDirectory: defaultDir,
	})
	writeDebugLog(fmt.Sprintf("OpenDirectoryPicker: returned dir=%q err=%v", dir, err))
	return dir, err
}

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
		Ticket:          cfg.Ticket,
		CSRFToken:       cfg.CSRFToken,
		Datastore:       cfg.Datastore,
		Namespace:       cfg.Namespace,
		CertFingerprint: cfg.CertFingerprint,
		HostPrefix:      hostPrefix,
		Query:           query,
		Mode:            SearchMatchMode(mode),
		From:            from,
		To:              to,
		AssembleMissing: assembleMissing,
		Crypt:           cfg.Crypt,
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
