package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"pbscommon"
	"security"
)

// configSaveMu keeps two saves of config.json in this process from
// interleaving.
var configSaveMu sync.Mutex

type Config struct {
	// ==================== MULTI-PBS SUPPORT ====================
	// New: Map of PBS servers (key = server ID, value = server config)
	PBSServers map[string]*PBSServer `json:"pbs_servers,omitempty"`
	// Default PBS server ID to use when none is specified
	DefaultPBSID string `json:"default_pbs_id,omitempty"`

	// ==================== LEGACY SINGLE PBS (Deprecated) ====================
	// These fields are kept for backward compatibility with existing config.json
	// When loaded, they are automatically migrated to PBSServers["default"]
	BaseURL         string `json:"baseurl,omitempty"`
	CertFingerprint string `json:"certfingerprint,omitempty"`
	AuthID          string `json:"authid,omitempty"`
	Secret          string `json:"secret,omitempty"`
	// Username/password authentication. Runtime-only here: the source of truth
	// is the PBSServer entry (which persists both), promoted onto this Config by
	// EffectivePBS()/ToConfig(). A FRESH PBS ticket is minted from these at the
	// start of every operation (PBS tickets expire, so we never persist one).
	Username string `json:"-"`
	Password string `json:"-"`
	// PBS session ticket + CSRF for the in-flight operation. Never persisted;
	// set by App.withAuth (fresh per operation) just before a PBSClient is built,
	// so it authenticates via the PBSAuthCookie.
	Ticket    string `json:"-"`
	CSRFToken string `json:"-"`
	Datastore string `json:"datastore,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	// EncryptionKeyFile is the path to a Proxmox Backup Server encryption key
	// (JSON, as written by `proxmox-backup-client key create`). Snapshots
	// taken with encryption enabled cannot be listed-restored, extracted or
	// verified without it: every chunk is AES-256-GCM and the chunk digests in
	// the indexes are sha256(plaintext || id_key).
	EncryptionKeyFile string `json:"encryption_key_file,omitempty"`
	// EncryptionKeyPassphrase is the stored passphrase of a passphrase-protected
	// key file. Empty means "ask when needed". Stored like the PBS token, and
	// needed for scheduled jobs, which have nobody to ask. Never sent to the
	// frontend (EncryptionKeyPassphraseSet says whether one is stored).
	EncryptionKeyPassphrase    string `json:"encryption_key_passphrase,omitempty"`
	EncryptionKeyPassphraseSet bool   `json:"encryption_key_passphrase_set,omitempty"`
	// ClearEncryptionKeyPassphrase is a frontend instruction on save: delete
	// the stored passphrase. It is never persisted.
	ClearEncryptionKeyPassphrase bool `json:"clear_encryption_key_passphrase,omitempty"`
	// Crypt is EncryptionKeyFile after it has been read and unlocked, held
	// runtime-only so the unlocked key never lands in config.json.
	Crypt *pbscommon.CryptConfig `json:"-"`
	// keyFileData is the key file's contents as the service handed them to
	// an app that may not read the file (serviceAccess); unlockCrypt uses
	// them instead of reading EncryptionKeyFile.
	keyFileData []byte

	// ==================== BACKUP SETTINGS ====================
	BackupDir      string   `json:"backupdir,omitempty"`
	BackupID       string   `json:"backup-id,omitempty"`
	UseVSS         bool     `json:"usevss"`
	LastBackupDirs []string `json:"last_backup_dirs,omitempty"` // Remember last used directories
	// Auto-split settings. DisableSplit defaults to false (zero value) so existing
	// configs keep auto-splitting. SplitSizeGB is both the split threshold and the
	// per-bin target size; 0 means the default (DefaultSplitSizeGB).
	DisableSplit bool `json:"disable_split,omitempty"`
	SplitSizeGB  int  `json:"split_size_gb,omitempty"`
	// UpstreamPinnedDisks is the disk table the project's app kept before the
	// fork was merged (1.2.0-rc): each disk's identity and the device path it
	// was last seen at, e.g. "serial:S4XNX0A123": "\\.\PhysicalDrive0". It is
	// only read: migrateUpstreamDiskPins gives a machine set saved by that app
	// the identities of its disks from it each time the sets are loaded. It is
	// kept in the file (SaveConfig carries it over), so a set not yet saved
	// with identities of its own still finds them.
	UpstreamPinnedDisks map[string]string `json:"pinned_disks,omitempty"`

	// ==================== RESTORE SETTINGS ====================
	// ParallelRestore opts into extracting files through a worker pool
	// instead of one at a time — added 2026-09-23 after a real restore's
	// timing breakdown showed per-file filesystem overhead (create/close/
	// rename/chtimes), not network, as the dominant cost once chunk-fetch
	// prefetch was in place. Defaults to false (the original, proven
	// sequential path — pbscommon.PXARReader.ExtractWithRewriter) since this
	// is newer, less-proven code touching real file writes during restore;
	// true uses ExtractWithRewriterParallel instead. Both paths are kept
	// fully independent in pbscommon so this can never change the default's
	// behavior.
	ParallelRestore bool `json:"parallel_restore,omitempty"`

	// ==================== BACKUP READ PERFORMANCE ====================
	// ParallelBackupRead opts into prefetching each directory's files into
	// memory with a worker pool, ahead of the archiver's own single-threaded
	// scan/chunk/hash pass — added 2026-09-30 after live CPU sampling on a
	// large (447GB), mostly-unchanged directory backup showed the process
	// fluctuating at 20-70% of one core rather than pinned, consistent with
	// disk-read wait rather than CPU-bound hashing. Deliberately does NOT
	// change chunk boundaries or dedup behavior at all: the archiver
	// (pbscommon.PXARArchive.WriteFile) still scans/chunks bytes in the exact
	// same order either way — prefetch only changes where those bytes come
	// from (an already-filled buffer instead of a blocking disk read), so
	// there is no one-time re-dedup cost and no format change, unlike a
	// genuine parallel-hashing redesign would carry. Defaults to false (the
	// original, proven sequential read path) for the same reason
	// ParallelRestore above does. BackupReadWorkers is only meaningful when
	// ParallelBackupRead is true; 0 or unset means the default (see
	// pbscommon.DefaultPrefetchWorkers).
	ParallelBackupRead bool `json:"parallel_backup_read,omitempty"`
	BackupReadWorkers  int  `json:"backup_read_workers,omitempty"`

	// MaxConcurrentRuns is how many backups and restores may run at once
	// (operation_queue.go): 0 means the default (DefaultMaxConcurrentRuns),
	// 1 runs them one at a time as before. Whatever it is set to, only one
	// machine backup and one restore run at a time, and a set never runs
	// twice at once.
	MaxConcurrentRuns int `json:"max_concurrent_runs,omitempty"`

	// Speed limits in MB/s (decimal megabytes), 0 for none (speed_limits.go).
	// Each is one budget for the whole process, shared by every backup and
	// restore running at the same time.
	UploadLimitMBps   int `json:"upload_limit_mbps,omitempty"`
	DownloadLimitMBps int `json:"download_limit_mbps,omitempty"`
	DiskReadLimitMBps int `json:"disk_read_limit_mbps,omitempty"`

	// UploadThreads is how many chunks a backup sends to PBS at a time, 1 to
	// pbscommon.MaxUploadThreads, or 0 for auto (pbscommon/upload_threads.go).
	UploadThreads int `json:"upload_threads,omitempty"`

	// ChunkTimeoutSeconds is how long a chunk upload may make no progress
	// before it is given up and sent again, 30 to 1800 seconds, or 0 for the
	// default of 90 (pbscommon/upload_stall.go).
	ChunkTimeoutSeconds int `json:"chunk_timeout_seconds,omitempty"`

	// ShareLogins are the logins for network shares, by server in lower
	// case (share_logins.go): what the service connects to a share with.
	ShareLogins map[string]*ShareLogin `json:"share_logins,omitempty"`

	// CompressionLevel is how hard chunks are compressed before they go to
	// PBS, for every backup this computer makes (compression.go): fastest,
	// default (zstd level 3), better or best. Empty means default. Backup
	// Sets' own "compression" field is no longer read.
	CompressionLevel string `json:"compression_level,omitempty"`

	// ==================== EMAIL NOTIFICATIONS ====================
	// Global SMTP account, used by every Backup Set's own on-completion/
	// on-failure email toggle (see ScheduledJob) rather than each job storing
	// its own SMTP credentials. EmailTo is unused by that per-job flow (each
	// job has its own recipient) but kept for backward compatibility with
	// anything that already persisted it.
	SMTPHost     string `json:"smtp_host,omitempty"`
	SMTPPort     string `json:"smtp_port,omitempty"`
	SMTPUsername string `json:"smtp_username,omitempty"`
	SMTPPassword string `json:"smtp_password,omitempty"`
	SMTPInsecure bool   `json:"smtp_insecure,omitempty"`
	EmailFrom    string `json:"email_from,omitempty"`
	EmailTo      string `json:"email_to,omitempty"`

	// ==================== UI PREFERENCES ====================
	// Theme is where earlier versions kept the Preferences > Theme choice,
	// one for the whole computer. Each user's choice now has its own file
	// (theme.go); this is only read for a user who has not chosen yet, so
	// nobody's colours change on upgrade.
	Theme *ThemeSettings `json:"theme,omitempty"`
}

// ThemeSettings is the app's accent color plus the hero banner's 3-stop
// gradient (dark end -> Accent, at 60% -> light end). Accent/AccentHover/
// HeroStart/HeroEnd are only meaningful when Preset == "custom" — for a
// built-in preset the frontend already knows its colors and only needs the
// name, but they're saved either way so a custom theme's exact values
// survive even if the built-in preset list changes later.
type ThemeSettings struct {
	Preset      string `json:"preset"` // "amber" | "blue" | "green" | "red" | "dark" | "custom"
	Accent      string `json:"accent,omitempty"`
	AccentHover string `json:"accentHover,omitempty"`
	HeroStart   string `json:"heroStart,omitempty"`
	HeroEnd     string `json:"heroEnd,omitempty"`
}

// sanitized returns a copy of the config with all secrets stripped (legacy PBS
// token, SMTP password, and every PBSServer token), for any path that hands the
// config to the frontend (M-04). Internal callers must use the real *Config.
func (c *Config) sanitized() *Config {
	cp := *c
	cp.Secret = ""
	cp.EncryptionKeyPassphraseSet = c.EncryptionKeyPassphrase != ""
	cp.EncryptionKeyPassphrase = ""
	cp.SMTPPassword = ""
	cp.Username = "" // runtime credentials, never to the frontend
	cp.Password = ""
	cp.Ticket = "" // runtime credential, never to the frontend
	cp.CSRFToken = ""
	if c.PBSServers != nil {
		cp.PBSServers = make(map[string]*PBSServer, len(c.PBSServers))
		for k, v := range c.PBSServers {
			cp.PBSServers[k] = v.sanitized()
		}
	}
	cp.ShareLogins = sanitizedShareLogins(c.ShareLogins)
	return &cp
}

// SplitSizeBytes returns the configured auto-split threshold / per-bin target in
// bytes, falling back to DefaultSplitSizeGB when SplitSizeGB is unset (<= 0).
func (c *Config) SplitSizeBytes() uint64 {
	gb := c.SplitSizeGB
	if gb <= 0 {
		gb = DefaultSplitSizeGB
	}
	return uint64(gb) * 1024 * 1024 * 1024
}

// DefaultBackupReadWorkers is used when ParallelBackupRead is on but
// BackupReadWorkers hasn't been set — matches chunkUploadWorkers
// (backup_inline.go) so the read-ahead pool and the upload pool default to
// the same size.
const DefaultBackupReadWorkers = 8

// EffectivePrefetchWorkers returns the worker count WriteDir's read-ahead
// should use: 0 (disabled) unless ParallelBackupRead is on, in which case
// it's BackupReadWorkers or DefaultBackupReadWorkers if that's unset.
func (c *Config) EffectivePrefetchWorkers() int {
	if !c.ParallelBackupRead {
		return 0
	}
	if c.BackupReadWorkers <= 0 {
		return DefaultBackupReadWorkers
	}
	return c.BackupReadWorkers
}

// atomicWriteFile writes data to path crash-safely: it writes a temp file in the
// same directory, fsyncs+closes it, then atomically renames it over path. A crash
// mid-write leaves the original intact instead of a truncated/half-written JSON
// (audit M-03). NOTE: this does NOT serialize concurrent GUI+service writers — a
// cross-process lock / single-writer is a separate fix for lost updates.
func atomicWriteFile(path string, data []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-"+filepath.Base(path)+"-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }() // no-op once the rename succeeds
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// getAPITokenPath is the service's local-API token file (H-01), in the
// service's settings folder whichever program asks: the service writes it,
// and only the service, root and elevated administrators may read it.
func getAPITokenPath() string {
	dir, err := serviceStateDir()
	if err != nil || dir == "" {
		return serviceExeBase() + "-api-token"
	}
	return filepath.Join(dir, "api-token")
}

// useServiceStateDir makes this app use the service's settings folder: set
// by the Linux command line (-list-sets, -undelete, -rollback), which runs
// as root on the service's Backup Sets.
var useServiceStateDir atomic.Bool

// getConfigDir returns the settings folder this program keeps its files in,
// creating it if needed.
//
// The background service keeps the computer's settings in its own folder
// (serviceStateDir): C:\ProgramData\Salvanza, or /var/lib/salvanza on Linux
// (each brand its own, identity.go). Only the service, SYSTEM or root, and
// elevated administrators may open it (upstream's strict model); the app
// reaches those settings through the service's API.
//
// The app keeps its own files in the user's ~/.salvanza, as upstream does
// with its own folder: all its settings when it runs without a service, and
// otherwise only what is its own (restore caches, its message log copy).
func getConfigDir() (string, error) {
	var (
		dir string
		err error
	)
	if serviceBuild || useServiceStateDir.Load() {
		dir, err = serviceStateDir()
	} else {
		dir, err = userConfigDir()
	}
	if err != nil {
		return "", err
	}
	// #nosec G703 -- the folder comes from ProgramData or the home folder, not user input
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return dir, nil
}

// serviceStateDir is the background service's settings folder (not created):
// C:\ProgramData\<brand folder> on Windows. The service, the first time it
// runs, takes over an earlier install's settings (adoptWindowsState).
func serviceStateDir() (string, error) {
	programData := os.Getenv("ProgramData")
	if programData == "" {
		if systemDrive := os.Getenv("SystemDrive"); systemDrive != "" {
			// ProgramData unset (a service started with a bare environment).
			programData = filepath.Join(systemDrive, "ProgramData")
		}
	}
	if programData == "" {
		return unixServiceStateDir()
	}
	dir := filepath.Join(programData, folderName())
	if serviceBuild {
		adoptServiceState(dir)
	}
	return dir, nil
}

var adoptWindowsStateOnce sync.Once

// adoptServiceState runs adoptWindowsState for the service's settings folder
// dir (C:\ProgramData\<brand folder>), once per process: in the service, and
// in the app as administrator before Run as Service gives the service the
// user's own settings (copyUserSettingsToService), so an earlier install's
// service settings are taken over first and win, as in the service itself.
func adoptServiceState(dir string) {
	adoptWindowsStateOnce.Do(func() { adoptWindowsState(filepath.Dir(dir), dir) })
}

// adoptWindowsState gives the service's settings folder an earlier install's
// settings when it has none: the folder every brand shared before the rename
// (C:\ProgramData\ProxmoxBackupClient), or another brand's, which an install
// of this brand replaces for now. Logged; a failure leaves the folder empty
// and the service starts without settings, as on a new computer.
func adoptWindowsState(programData, dir string) {
	candidates := []string{filepath.Join(programData, legacyFolder)}
	for _, f := range otherBrandFolders() {
		candidates = append(candidates, filepath.Join(programData, f))
	}
	from, err := adoptFolder(dir, candidates)
	switch {
	case err != nil:
		fmt.Fprintf(os.Stderr, "Settings of an earlier install not taken over: %v\n", err)
	case from != "":
		fmt.Fprintf(os.Stderr, "Took over the settings of an earlier install: %s is now %s\n", from, dir)
	}
}

func getConfigPath() (string, error) {
	dir, err := getConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func LoadConfig() *Config {
	config := &Config{
		UseVSS:     true, // Default to VSS enabled on Windows
		PBSServers: make(map[string]*PBSServer),
	}

	configPath, err := getConfigPath()
	if err != nil {
		return config
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		// Config doesn't exist yet, return empty config. Anything else (no
		// permission, say) is worth a line: the settings are there, unread.
		if !os.IsNotExist(err) {
			writeDebugLog(fmt.Sprintf("config: could not read %s: %v", configPath, err))
		}
		return config
	}

	if err := json.Unmarshal(stripUTF8BOM(data), config); err != nil {
		return config
	}
	if config.openSecrets() {
		sealOnLoadOnce.Do(func() {
			if err := config.Save(); err != nil {
				writeDebugLog(fmt.Sprintf("config: could not write the secrets back sealed: %v", err))
			} else {
				writeDebugLog("config: plain-text secrets in config.json sealed")
			}
		})
	}
	// An encryption key that went along when an earlier install's settings
	// folder became this one is named where it is now. Saved, so the settings
	// agree with the disk; a save that fails is tried again on the next load.
	if moved := config.repointMovedKeyPaths(filepath.Dir(configPath)); len(moved) > 0 {
		if err := config.Save(); err != nil {
			writeDebugLog(fmt.Sprintf("config: could not save the encryption key paths put right: %v", err))
		}
		for _, m := range moved {
			writeDebugLog("config: encryption key moved with the settings folder: " + m)
		}
	}

	// ==================== AUTO-MIGRATION ====================
	// If legacy single PBS config exists (BaseURL not empty) and PBSServers is empty,
	// migrate to multi-PBS format automatically
	if config.BaseURL != "" && len(config.PBSServers) == 0 {
		// Create default PBS server from legacy config
		defaultPBS := &PBSServer{
			ID:                      "default",
			Name:                    "Main PBS Server",
			BaseURL:                 config.BaseURL,
			CertFingerprint:         config.CertFingerprint,
			AuthID:                  config.AuthID,
			Secret:                  config.Secret,
			EncryptionKeyFile:       config.EncryptionKeyFile,
			EncryptionKeyPassphrase: config.EncryptionKeyPassphrase,
			Datastore:               config.Datastore,
			Namespace:               config.Namespace,
			Description:             "Default PBS server (migrated from old config)",
		}

		// Initialize PBSServers map if nil
		if config.PBSServers == nil {
			config.PBSServers = make(map[string]*PBSServer)
		}

		config.PBSServers["default"] = defaultPBS
		config.DefaultPBSID = "default"

		// Save migrated config immediately
		_ = config.Save()
	}

	// Ensure PBSServers map is initialized
	if config.PBSServers == nil {
		config.PBSServers = make(map[string]*PBSServer)
	}

	return config
}

func (c *Config) Save() error {
	// Runs that finish together save from their own goroutines.
	configSaveMu.Lock()
	defer configSaveMu.Unlock()
	configPath, err := getConfigPath()
	if err != nil {
		return err
	}

	sealed, err := c.sealedCopy()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(sealed, "", "  ")
	if err != nil {
		return err
	}

	return atomicWriteFile(configPath, data, 0600)
}

func (c *Config) Validate() error {
	// Validate BaseURL
	if c.BaseURL == "" {
		return fmt.Errorf("PBS server URL required")
	}
	if err := security.ValidateURL(c.BaseURL); err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	// Auth: an API token (AuthID+Secret), a stored username/password (a fresh
	// ticket is minted per operation by App.withAuth), or an in-flight ticket.
	if c.AuthID != "" {
		if err := security.ValidateAuthID(c.AuthID); err != nil {
			return fmt.Errorf("invalid authentication ID: %w", err)
		}
		if c.Secret == "" {
			return fmt.Errorf("secret required")
		}
	} else if c.Ticket == "" && (c.Username == "" || c.Password == "") {
		return fmt.Errorf("authentication required (API token or username/password)")
	}

	// Validate Datastore
	if c.Datastore == "" {
		return fmt.Errorf("datastore required")
	}
	if err := security.ValidateDatastore(c.Datastore); err != nil {
		return fmt.Errorf("invalid datastore: %w", err)
	}

	// Validate BackupID if present
	if c.BackupID != "" {
		if err := security.ValidateBackupID(c.BackupID); err != nil {
			return fmt.Errorf("invalid backup ID: %w", err)
		}
	}

	// Validate Certificate Fingerprint if present
	if c.CertFingerprint != "" {
		if err := security.ValidateFingerprint(c.CertFingerprint); err != nil {
			return fmt.Errorf("invalid certificate fingerprint: %w", err)
		}
	}

	// Validate BackupDir if present
	if c.BackupDir != "" {
		if err := security.ValidatePath(c.BackupDir); err != nil {
			return fmt.Errorf("invalid backup path: %w", err)
		}
	}

	// Validate the encryption key file up front. A dangling or unsupported key
	// path would otherwise only surface much later — deep inside a backup or,
	// worse, as an "unable to decrypt blob" halfway through a restore — so fail
	// here where the user just typed the path.
	if err := c.validateEncryptionKeyFile(); err != nil {
		return err
	}

	return nil
}

// validateEncryptionKeyFile checks that EncryptionKeyFile is usable by the GUI
// without a console to prompt on. It deliberately does NOT touch Crypt: callers
// that are about to encrypt or decrypt call loadCryptConfig to unlock it.
//
// An empty path is valid (unencrypted snapshots). Anything else must exist and
// must not be passphrase-protected.
func (c *Config) validateEncryptionKeyFile() error {
	if c.EncryptionKeyFile == "" {
		return nil
	}
	if err := security.ValidatePath(c.EncryptionKeyFile); err != nil {
		return fmt.Errorf("invalid encryption key path: %w", err)
	}
	if _, err := os.Stat(c.EncryptionKeyFile); err != nil {
		return fmt.Errorf("cannot read the encryption key file (%s): %w", c.EncryptionKeyFile, err)
	}
	keyCfg, err := pbscommon.LoadKeyConfig(c.EncryptionKeyFile)
	if err != nil {
		return fmt.Errorf("reading the encryption key file %s: %w", c.EncryptionKeyFile, err)
	}
	if c.EncryptionKeyPassphrase != "" && keyCfg.KDF != nil {
		if err := verifyKeyPassphrase(c.EncryptionKeyFile, c.EncryptionKeyPassphrase); err != nil {
			return fmt.Errorf("the saved passphrase does not unlock the key %s: %w", c.EncryptionKeyFile, err)
		}
	}
	return nil
}

// ==================== MULTI-PBS HELPER METHODS ====================

// EffectivePBS returns a Config whose legacy PBS fields (BaseURL, AuthID, etc.)
// are guaranteed to reflect the active PBS server. If the legacy fields are
// already set, the receiver is returned as-is. Otherwise, the default entry
// from PBSServers is promoted into a shallow copy so callers that still read
// legacy fields work seamlessly with multi-PBS configurations.
func (c *Config) EffectivePBS() *Config {
	if c.BaseURL != "" || len(c.PBSServers) == 0 {
		return c
	}
	pbs, err := c.GetPBSServer("")
	if err != nil {
		return c
	}
	cp := *c
	cp.BaseURL = pbs.BaseURL
	cp.CertFingerprint = pbs.CertFingerprint
	cp.AuthID = pbs.AuthID
	cp.Secret = pbs.Secret
	cp.Username = pbs.Username
	cp.Password = pbs.Password
	// The key is per-server: without this the whole multi-PBS path silently ran
	// unencrypted (and could not decrypt encrypted snapshots), because the
	// legacy top-level EncryptionKeyFile stays empty in that mode.
	cp.EncryptionKeyFile = pbs.EncryptionKeyFile
	cp.EncryptionKeyPassphrase = pbs.EncryptionKeyPassphrase
	cp.Datastore = pbs.Datastore
	cp.Namespace = pbs.Namespace
	return &cp
}

// GetPBSServer returns a PBS server by ID, or the default if ID is empty
func (c *Config) GetPBSServer(id string) (*PBSServer, error) {
	// If no ID specified, use default
	if id == "" {
		id = c.DefaultPBSID
	}

	// If still empty and only one server exists, use it
	if id == "" && len(c.PBSServers) == 1 {
		for _, pbs := range c.PBSServers {
			return pbs, nil
		}
	}

	// If still empty, return error
	if id == "" {
		return nil, fmt.Errorf("no PBS server specified and no default server")
	}

	// Get server by ID
	pbs, exists := c.PBSServers[id]
	if !exists {
		return nil, fmt.Errorf("PBS server '%s' not found", id)
	}

	return pbs, nil
}

// AddPBSServer adds a new PBS server to the configuration
func (c *Config) AddPBSServer(pbs *PBSServer) error {
	if err := pbs.Validate(); err != nil {
		return err
	}

	// A server authenticates with EITHER a token OR user/password, never both.
	if pbs.AuthID != "" {
		pbs.Username = ""
		pbs.Password = ""
	} else if pbs.Username != "" {
		if pbs.Password == "" {
			return fmt.Errorf("password required for username/password login")
		}
		pbs.Secret = ""
	}

	if c.PBSServers == nil {
		c.PBSServers = make(map[string]*PBSServer)
	}

	// Check if ID already exists
	if _, exists := c.PBSServers[pbs.ID]; exists {
		return fmt.Errorf("PBS server with ID '%s' already exists", pbs.ID)
	}

	c.PBSServers[pbs.ID] = pbs

	// If this is the first server, set it as default
	if len(c.PBSServers) == 1 {
		c.DefaultPBSID = pbs.ID
	}

	return c.Save()
}

// UpdatePBSServer updates an existing PBS server
func (c *Config) UpdatePBSServer(pbs *PBSServer) error {
	if err := pbs.Validate(); err != nil {
		return err
	}

	existing, exists := c.PBSServers[pbs.ID]
	if !exists {
		return fmt.Errorf("PBS server '%s' not found", pbs.ID)
	}

	// A server authenticates with EITHER a token OR user/password, never both.
	if pbs.AuthID != "" {
		pbs.Username = ""
		pbs.Password = ""
	} else if pbs.Username != "" {
		// Empty password on a user/pass edit means "keep the stored one": the
		// frontend never receives the password, so it cannot echo it back.
		// Only for the same user: the stored password is the old user's.
		if pbs.Password == "" {
			if existing.Password == "" || existing.Username != pbs.Username {
				return fmt.Errorf("password required for username/password login")
			}
			pbs.Password = existing.Password
		}
		pbs.Secret = ""
	}

	c.PBSServers[pbs.ID] = pbs
	return c.Save()
}

// DeletePBSServer removes a PBS server from the configuration
func (c *Config) DeletePBSServer(id string) error {
	if _, exists := c.PBSServers[id]; !exists {
		return fmt.Errorf("PBS server '%s' not found", id)
	}

	delete(c.PBSServers, id)

	// If we deleted the default server, pick a new default
	if c.DefaultPBSID == id {
		if len(c.PBSServers) > 0 {
			// Pick the first available server as new default
			for newDefaultID := range c.PBSServers {
				c.DefaultPBSID = newDefaultID
				break
			}
		} else {
			c.DefaultPBSID = ""
		}
	}

	return c.Save()
}

// ListPBSServers returns all configured PBS servers
func (c *Config) ListPBSServers() []*PBSServer {
	servers := make([]*PBSServer, 0, len(c.PBSServers))
	for _, pbs := range c.PBSServers {
		servers = append(servers, pbs)
	}
	return servers
}

// SetDefaultPBS sets the default PBS server ID
func (c *Config) SetDefaultPBS(id string) error {
	if _, exists := c.PBSServers[id]; !exists {
		return fmt.Errorf("PBS server '%s' not found", id)
	}

	c.DefaultPBSID = id
	return c.Save()
}

// loadCryptConfig reads and unlocks EncryptionKeyFile into the runtime-only
// Crypt field, which every restore-side pbscommon.PBSClient is built with.
//
// A passphrase-protected key is unlocked with the stored passphrase, else the
// one entered earlier in this session, else by asking the user through the GUI
// (see crypt_passphrase.go). In the service build nobody can be asked, so a
// protected key without a stored passphrase fails with ENCRYPTION_PASSPHRASE_REQUIRED.
// An empty EncryptionKeyFile leaves Crypt nil, which is correct for plain
// snapshots; encrypted ones then fail per chunk saying the key is missing.
func (c *Config) loadCryptConfig() error {
	return c.unlockCrypt(true)
}

// parseKeyConfig reads a key file's contents (pbscommon.LoadKeyConfig from
// memory).
func parseKeyConfig(raw []byte) (*pbscommon.KeyConfig, error) {
	cfg := &pbscommon.KeyConfig{}
	if err := json.Unmarshal(raw, cfg); err != nil {
		return nil, fmt.Errorf("unable to parse the key file: %w", err)
	}
	if cfg.Data == "" {
		return nil, fmt.Errorf("the key file contains no key data")
	}
	return cfg, nil
}

// unlockCrypt is loadCryptConfig with control over whether the user may be
// asked for a passphrase. Saving settings must not pop a dialog, so it passes
// false and treats errPassphraseRequired as fine.
func (c *Config) unlockCrypt(interactive bool) error {
	if c.EncryptionKeyFile == "" {
		c.Crypt = nil
		return nil
	}

	var keyCfg *pbscommon.KeyConfig
	var err error
	if c.keyFileData != nil {
		keyCfg, err = parseKeyConfig(c.keyFileData)
	} else {
		keyCfg, err = pbscommon.LoadKeyConfig(c.EncryptionKeyFile)
	}
	if err != nil {
		return fmt.Errorf("reading encryption key file %s: %w", c.EncryptionKeyFile, err)
	}

	if keyCfg.KDF != nil {
		c.Crypt, err = unlockProtectedKey(c.EncryptionKeyFile, c.EncryptionKeyPassphrase, keyCfg, interactive)
	} else {
		c.Crypt, err = keyCfg.CryptConfig(nil)
	}
	if err != nil {
		if errors.Is(err, errPassphraseRequired) && !interactive {
			c.Crypt = nil
			return err
		}
		return fmt.Errorf("unlocking encryption key file %s: %w", c.EncryptionKeyFile, err)
	}
	return nil
}
