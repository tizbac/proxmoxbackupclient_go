package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	stdruntime "runtime"

	"pbscommon"
	"security"
)

// configDirOverride pins the config directory (and everything derived from it:
// config.json, scheduled jobs, job history, restore cache) for this process.
// It is set once at startup: the service — and the GUI when it talks to the
// service — use serviceStateDir(); a standalone GUI uses
// standaloneConfigDir(). When unset, the historical default applies (home dir
// on Linux, ProgramData on Windows), which existing tests rely on.
var configDirOverride string

// SetConfigDir pins the config directory for this process. It must be called
// before any config/scheduler/cache path is resolved.
func SetConfigDir(dir string) {
	configDirOverride = dir
}

// serviceStateDir is the privileged shared state directory owned by the local
// service: /var/lib/pbsgo on Linux (created 0700 by systemd via
// StateDirectory=) and C:\ProgramData\ProxmoxBackupClient on Windows. The
// service writes config.json, scheduled_jobs.json, job_history.json and
// api-token there; the GUI never writes there directly — every change goes
// through the service API.
func serviceStateDir() string {
	if programData := os.Getenv("ProgramData"); programData != "" {
		// #nosec G108 -- ProgramData is a trusted Windows system environment variable, not user input
		return filepath.Join(programData, "ProxmoxBackupClient")
	}
	return "/var/lib/pbsgo"
}

// serviceTokenPath is where the service keeps its local-API token (0600,
// root-owned on Linux, SYSTEM+Administrators DACL on Windows). The GUI always
// probes the service through THIS path — never a home-dir copy.
func serviceTokenPath() string {
	return filepath.Join(serviceStateDir(), "api-token")
}

// standaloneConfigDir is where a standalone GUI (local service not running)
// keeps its config on BOTH platforms: the user's own home directory, so no
// elevation is needed to read or write it.
func standaloneConfigDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, ".proxmox-backup-guardian")
}

// migrateStandaloneFromProgramData copies the legacy shared-state files
// (config.json, scheduled_jobs.json, job_history.json) from the service state
// dir into the home-based standalone dir, once, when the home copies don't
// exist yet. Windows-only concern: after the Windows config moved to the home
// dir, users upgrading from a version that stored everything in ProgramData
// should keep their settings visible. Best-effort and silent: the files are
// SYSTEM-only readable once hardened, so the copy may simply not happen.
func migrateStandaloneFromProgramData() {
	if stdruntime.GOOS != "windows" {
		return
	}
	src := serviceStateDir()
	dst := standaloneConfigDir()
	if src == "" || dst == "" || src == dst {
		return
	}
	for _, name := range []string{"config.json", "scheduled_jobs.json", "job_history.json"} {
		srcFile := filepath.Join(src, name)
		dstFile := filepath.Join(dst, name)
		if _, err := os.Stat(dstFile); err == nil {
			continue // home copy already exists: never clobber it
		}
		data, err := os.ReadFile(srcFile)
		if err != nil {
			continue // not readable (e.g. SYSTEM-only ACL): skip
		}
		_ = os.MkdirAll(dst, 0755)
		_ = atomicWriteFile(dstFile, data, 0600)
	}
}

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
	// Crypt is EncryptionKeyFile after it has been read and unlocked, held
	// runtime-only so the unlocked key never lands in config.json.
	Crypt *pbscommon.CryptConfig `json:"-"`

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

	// ==================== EMAIL NOTIFICATIONS ====================
	SMTPHost     string `json:"smtp_host,omitempty"`
	SMTPPort     string `json:"smtp_port,omitempty"`
	SMTPUsername string `json:"smtp_username,omitempty"`
	SMTPPassword string `json:"smtp_password,omitempty"`
	EmailFrom    string `json:"email_from,omitempty"`
	EmailTo      string `json:"email_to,omitempty"`
}

// sanitized returns a copy of the config with all secrets stripped (legacy PBS
// token, SMTP password, and every PBSServer token), for any path that hands the
// config to the frontend (M-04). Internal callers must use the real *Config.
func (c *Config) sanitized() *Config {
	cp := *c
	cp.Secret = ""
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

// getAPITokenPath is the shared local-API auth token file (H-01). It always
// points at the service's token file: the GUI authenticates to the local
// service (if one is running) with that token, and a standalone GUI has no
// service to talk to, so no other token path is ever used.
func getAPITokenPath() string {
	return serviceTokenPath()
}

// getConfigDir returns the application's data directory, creating it if needed.
// When SetConfigDir() pinned one, that directory is used as-is (service state
// dir for the service / service-mode GUI, home dir for a standalone GUI).
// Otherwise the historical default applies: ProgramData on Windows (shared
// GUI/Service) and ~/.proxmox-backup-guardian on Unix. Used as the parent for
// config.json, the restore cache, and any other persistent state.
func getConfigDir() (string, error) {
	configDir := configDirOverride

	if configDir == "" {
		if programData := os.Getenv("ProgramData"); programData != "" {
			// Windows: C:\ProgramData\ProxmoxBackupClient (accessible by both user and LocalSystem)
			// #nosec G108 -- ProgramData is a trusted Windows system environment variable, not user input
			configDir = filepath.Join(programData, "ProxmoxBackupClient")
		} else if systemDrive := os.Getenv("SystemDrive"); systemDrive != "" {
			// Windows fallback: if ProgramData not set, use C:\ProgramData hardcoded
			// This ensures service config is accessible even if env var is missing
			configDir = filepath.Join(systemDrive, "ProgramData", "ProxmoxBackupClient")
		} else {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			configDir = filepath.Join(homeDir, ".proxmox-backup-guardian")
		}
	}

	if configDir == "" {
		return "", fmt.Errorf("no config directory available")
	}

	// #nosec G302 -- configDir is a system location or an explicit override set at startup
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", err
	}

	return configDir, nil
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
		// Config doesn't exist yet, return empty config
		return config
	}

	if err := json.Unmarshal(data, config); err != nil {
		return config
	}

	// ==================== AUTO-MIGRATION ====================
	// If legacy single PBS config exists (BaseURL not empty) and PBSServers is empty,
	// migrate to multi-PBS format automatically
	if config.BaseURL != "" && len(config.PBSServers) == 0 {
		// Create default PBS server from legacy config
		defaultPBS := &PBSServer{
			ID:                "default",
			Name:              "Serveur PBS Principal",
			BaseURL:           config.BaseURL,
			CertFingerprint:   config.CertFingerprint,
			AuthID:            config.AuthID,
			Secret:            config.Secret,
			EncryptionKeyFile: config.EncryptionKeyFile,
			Datastore:         config.Datastore,
			Namespace:         config.Namespace,
			Description:       "Serveur PBS par défaut (migré depuis ancienne config)",
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
	configPath, err := getConfigPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	return atomicWriteFile(configPath, data, 0600)
}

func (c *Config) Validate() error {
	// Validate BaseURL
	if c.BaseURL == "" {
		return fmt.Errorf("URL du serveur PBS requis")
	}
	if err := security.ValidateURL(c.BaseURL); err != nil {
		return fmt.Errorf("URL invalide: %w", err)
	}

	// Auth: an API token (AuthID+Secret), a stored username/password (a fresh
	// ticket is minted per operation by App.withAuth), or an in-flight ticket.
	if c.AuthID != "" {
		if err := security.ValidateAuthID(c.AuthID); err != nil {
			return fmt.Errorf("authentication ID invalide: %w", err)
		}
		if c.Secret == "" {
			return fmt.Errorf("secret requis")
		}
	} else if c.Ticket == "" && (c.Username == "" || c.Password == "") {
		return fmt.Errorf("authentification requise (API token ou utilisateur/mot de passe)")
	}

	// Validate Datastore
	if c.Datastore == "" {
		return fmt.Errorf("datastore requis")
	}
	if err := security.ValidateDatastore(c.Datastore); err != nil {
		return fmt.Errorf("datastore invalide: %w", err)
	}

	// Validate BackupID if present
	if c.BackupID != "" {
		if err := security.ValidateBackupID(c.BackupID); err != nil {
			return fmt.Errorf("backup ID invalide: %w", err)
		}
	}

	// Validate Certificate Fingerprint if present
	if c.CertFingerprint != "" {
		if err := security.ValidateFingerprint(c.CertFingerprint); err != nil {
			return fmt.Errorf("empreinte certificat invalide: %w", err)
		}
	}

	// Validate BackupDir if present
	if c.BackupDir != "" {
		if err := security.ValidatePath(c.BackupDir); err != nil {
			return fmt.Errorf("chemin de backup invalide: %w", err)
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

// validatePBSFields validates the legacy top-level PBS block PART BY PART and
// only where a value is set: it is what a whole-document save (service
// /config POST, legacy settings form) runs, because Config.Validate() demands
// a legacy BaseURL and credentials outright — which a multi-PBS-only
// configuration (empty baseurl/authid, credentials carried per server) can
// never satisfy. Requiring them there rejected every save with "Either
// username or authid is required!" even though the document was perfectly
// valid.
func (c *Config) validatePBSFields() error {
	// Credentials first: they are what the legacy block is really about.
	if c.AuthID != "" {
		if err := security.ValidateAuthID(c.AuthID); err != nil {
			return fmt.Errorf("authentication ID invalide: %w", err)
		}
		if c.Secret == "" {
			return fmt.Errorf("secret requis")
		}
	} else if c.Username != "" {
		if err := security.ValidateUsername(c.Username); err != nil {
			return fmt.Errorf("invalid username: %w", err)
		}
		if c.Password == "" {
			return fmt.Errorf("password required")
		}
	} else if c.BaseURL != "" {
		// A legacy entry with a URL but no credentials at all can never
		// authenticate; an empty legacy block (multi-PBS-only) is fine.
		return fmt.Errorf("Either username or authid is required!") //nolint:staticcheck // message predates the linter and is what the GUI/service reports
	}

	if c.BaseURL != "" {
		if err := security.ValidateURL(c.BaseURL); err != nil {
			return fmt.Errorf("URL du serveur PBS invalide: %w", err)
		}
	}
	if c.CertFingerprint != "" {
		if err := security.ValidateFingerprint(c.CertFingerprint); err != nil {
			return fmt.Errorf("empreinte certificat invalide: %w", err)
		}
	}
	if c.Datastore != "" {
		if err := security.ValidateDatastore(c.Datastore); err != nil {
			return fmt.Errorf("datastore invalide: %w", err)
		}
	}
	if c.BackupID != "" {
		if err := security.ValidateBackupID(c.BackupID); err != nil {
			return fmt.Errorf("backup ID invalide: %w", err)
		}
	}
	if c.BackupDir != "" {
		if err := security.ValidatePath(c.BackupDir); err != nil {
			return fmt.Errorf("chemin de backup invalide: %w", err)
		}
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
		return fmt.Errorf("chemin de cle de chiffrement invalide: %w", err)
	}
	if _, err := os.Stat(c.EncryptionKeyFile); err != nil {
		return fmt.Errorf("fichier de cle de chiffrement illisible (%s): %w", c.EncryptionKeyFile, err)
	}
	keyCfg, err := pbscommon.LoadKeyConfig(c.EncryptionKeyFile)
	if err != nil {
		return fmt.Errorf("lecture du fichier de cle de chiffrement %s: %w", c.EncryptionKeyFile, err)
	}
	if keyCfg.KDF != nil {
		return fmt.Errorf("le fichier de cle de chiffrement %s est protege par phrase de passe, ce que l'interface graphique ne peut pas deverrouiller: utilisez une cle creee avec `proxmox-backup-client key create --kdf none`, ou restaurez depuis la ligne de commande", c.EncryptionKeyFile)
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
		return nil, fmt.Errorf("aucun serveur PBS spécifié et pas de serveur par défaut")
	}

	// Get server by ID
	pbs, exists := c.PBSServers[id]
	if !exists {
		return nil, fmt.Errorf("serveur PBS '%s' introuvable", id)
	}

	return pbs, nil
}

// AddPBSServerMem adds a new PBS server to the in-memory configuration
// WITHOUT persisting. The GUI in service mode uses it (the service owns the
// files) and pushes the result through the service API afterwards.
//
// A new server always carries freshly typed credentials, so requireCreds is
// unconditional here: an empty secret/password is a missing credential, not
// "keep the stored one" (there is no stored entry yet).
func (c *Config) AddPBSServerMem(pbs *PBSServer) error {
	if err := normalizeServerAuth(nil, pbs, true); err != nil {
		return err
	}
	if err := pbs.Validate(); err != nil {
		return err
	}

	if c.PBSServers == nil {
		c.PBSServers = make(map[string]*PBSServer)
	}

	// Check if ID already exists
	if _, exists := c.PBSServers[pbs.ID]; exists {
		return fmt.Errorf("serveur PBS avec ID '%s' existe déjà", pbs.ID)
	}

	c.PBSServers[pbs.ID] = pbs

	// If this is the first server, set it as default
	if len(c.PBSServers) == 1 {
		c.DefaultPBSID = pbs.ID
	}

	return nil
}

// AddPBSServer adds a new PBS server to the configuration
func (c *Config) AddPBSServer(pbs *PBSServer) error {
	if err := c.AddPBSServerMem(pbs); err != nil {
		return err
	}
	return c.Save()
}

// UpdatePBSServerMem updates an existing PBS server in memory WITHOUT
// persisting (see AddPBSServerMem).
//
// requireCreds: false when the caller never received the stored credentials —
// the service-mode GUI — because an empty secret/password then means "keep the
// stored one" and the presence check belongs to the service, which owns them.
// A standalone GUI and the service itself pass true.
func (c *Config) UpdatePBSServerMem(pbs *PBSServer, requireCreds bool) error {
	existing, exists := c.PBSServers[pbs.ID]
	if !exists {
		return fmt.Errorf("serveur PBS '%s' introuvable", pbs.ID)
	}

	// Selects the auth method, clears the other one, and applies the
	// "empty = keep the stored credential" convention (never a silent reuse of
	// a token secret for a different authid, or of a password for another user).
	if err := normalizeServerAuth(existing, pbs, requireCreds); err != nil {
		return err
	}
	if err := pbs.validate(requireCreds); err != nil {
		return err
	}

	c.PBSServers[pbs.ID] = pbs
	return nil
}

// UpdatePBSServer updates an existing PBS server (standalone path: this
// process holds the stored credentials, so they are required).
func (c *Config) UpdatePBSServer(pbs *PBSServer) error {
	if err := c.UpdatePBSServerMem(pbs, true); err != nil {
		return err
	}
	return c.Save()
}

// DeletePBSServerMem removes a PBS server from the in-memory configuration
// WITHOUT persisting (see AddPBSServerMem).
func (c *Config) DeletePBSServerMem(id string) error {
	if _, exists := c.PBSServers[id]; !exists {
		return fmt.Errorf("serveur PBS '%s' introuvable", id)
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

	return nil
}

// DeletePBSServer removes a PBS server from the configuration
func (c *Config) DeletePBSServer(id string) error {
	if err := c.DeletePBSServerMem(id); err != nil {
		return err
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

// SetDefaultPBSMem sets the default PBS server ID in memory WITHOUT persisting
// (see AddPBSServerMem).
func (c *Config) SetDefaultPBSMem(id string) error {
	if _, exists := c.PBSServers[id]; !exists {
		return fmt.Errorf("serveur PBS '%s' introuvable", id)
	}

	c.DefaultPBSID = id
	return nil
}

// SetDefaultPBS sets the default PBS server ID
func (c *Config) SetDefaultPBS(id string) error {
	if err := c.SetDefaultPBSMem(id); err != nil {
		return err
	}
	return c.Save()
}

// loadCryptConfig reads and unlocks EncryptionKeyFile into the runtime-only
// Crypt field, which every restore-side pbscommon.PBSClient is built with.
//
// A passphrase-protected key file cannot be unlocked from a GUI that has no
// console, so only `--kdf none` key files are accepted here; anything else
// gets an explicit error naming the flag to set instead of a confusing
// "unable to decrypt blob - missing CryptConfig" much later. An empty
// EncryptionKeyFile leaves Crypt nil, which is correct for plain snapshots —
// encrypted ones will then fail per chunk with a message saying the key is
// missing.
func (c *Config) loadCryptConfig() error {
	if c.EncryptionKeyFile == "" {
		c.Crypt = nil
		return nil
	}

	keyCfg, err := pbscommon.LoadKeyConfig(c.EncryptionKeyFile)
	if err != nil {
		return fmt.Errorf("reading encryption key file %s: %w", c.EncryptionKeyFile, err)
	}
	if keyCfg.KDF != nil {
		return fmt.Errorf("encryption key file %s is passphrase-protected, which the GUI cannot unlock: use a key created with `proxmox-backup-client key create --kdf none`, or restore from the command line with proxmoxbackupclient/nbd", c.EncryptionKeyFile)
	}

	c.Crypt, err = keyCfg.CryptConfig(nil)
	if err != nil {
		return fmt.Errorf("unlocking encryption key file %s: %w", c.EncryptionKeyFile, err)
	}
	return nil
}

// fullConfigDocument returns a JSON-compatible map of the entire configuration
// (legacy fields + PBS servers) for the GUI in service mode to push to the
// service via the API.
//
// It is a BACKEND→service document (authenticated local API, never handed to
// the frontend), so it carries the credentials this process actually knows:
// a value typed in the current session is transmitted so the service can store
// it, while an unknown/untouched credential is "" and means "keep the stored
// one" on the service side (the GUI never receives those — M-04). The *_set
// markers are kept alongside for the frontend's placeholders.
func (c *Config) fullConfigDocument() map[string]interface{} {
	hostname, _ := os.Hostname()
	doc := map[string]interface{}{
		"baseurl":             c.BaseURL,
		"certfingerprint":     c.CertFingerprint,
		"authid":              c.AuthID,
		"secret":              c.Secret,
		"secret_set":          c.Secret != "",
		"datastore":           c.Datastore,
		"namespace":           c.Namespace,
		"encryption_key_file": c.EncryptionKeyFile,
		"backupdir":           c.BackupDir,
		"backup-id":           c.BackupID,
		"usevss":              c.UseVSS,
		"last_backup_dirs":    c.LastBackupDirs,
		"disable_split":       c.DisableSplit,
		"split_size_gb":       c.SplitSizeGB,
		"smtp_host":           c.SMTPHost,
		"smtp_port":           c.SMTPPort,
		"smtp_username":       c.SMTPUsername,
		"smtp_password":       c.SMTPPassword,
		"smtp_password_set":   c.SMTPPassword != "",
		"email_from":          c.EmailFrom,
		"email_to":            c.EmailTo,
		"default_pbs_id":      c.DefaultPBSID,
		"hostname":            hostname,
	}
	servers := map[string]interface{}{}
	for id, pbs := range c.PBSServers {
		if pbs == nil {
			continue
		}
		s := pbs.sanitized()
		servers[id] = map[string]interface{}{
			"id":                  s.ID,
			"name":                s.Name,
			"baseurl":             s.BaseURL,
			"certfingerprint":     s.CertFingerprint,
			"authid":              s.AuthID,
			"secret":              pbs.Secret,  // "" = keep the stored one
			"secret_set":          s.SecretSet, // unknown to this process ⇒ false
			"username":            s.Username,
			"password":            pbs.Password, // "" = keep the stored one
			"password_set":        s.PasswordSet,
			"datastore":           s.Datastore,
			"namespace":           s.Namespace,
			"encryption_key_file": s.EncryptionKeyFile,
			"description":         s.Description,
		}
	}
	// Always present (even when empty): the service treats a document without
	// the key as "this client does not manage servers", which would make a
	// delete of the last server impossible to persist.
	doc["pbs_servers"] = servers
	return doc
}

// parseFullConfig rebuilds a Config from the sanitized document returned by
// the service's /config GET. It is the inverse of fullConfigDocument() and
// preserves the "keep existing secret" semantics: the service fills
// secret_set/password_set markers but never the actual secrets, so the
// rehydrated Config has empty secrets (as expected — the GUI never holds them).
func parseFullConfig(doc map[string]interface{}) *Config {
	c := &Config{
		PBSServers: make(map[string]*PBSServer),
	}
	str := func(k string) string {
		if v, ok := doc[k].(string); ok {
			return v
		}
		return ""
	}
	num := func(k string) int {
		if v, ok := doc[k].(float64); ok {
			return int(v)
		}
		return 0
	}
	bol := func(k string) bool {
		if v, ok := doc[k].(bool); ok {
			return v
		}
		return false
	}

	c.BaseURL = str("baseurl")
	c.CertFingerprint = str("certfingerprint")
	c.AuthID = str("authid")
	// Secret is intentionally empty — the GUI never holds it
	c.Datastore = str("datastore")
	c.Namespace = str("namespace")
	// The key PATH is not a secret (only the path is stored; the unlocked key
	// never leaves the backend). Dropping it here would make the next push to
	// the service hand back an empty path and silently stop encrypting.
	c.EncryptionKeyFile = str("encryption_key_file")
	c.BackupDir = str("backupdir")
	c.BackupID = str("backup-id")
	c.UseVSS = bol("usevss")
	c.LastBackupDirs = stringSlice(doc["last_backup_dirs"])
	c.DisableSplit = bol("disable_split")
	c.SplitSizeGB = num("split_size_gb")
	c.SMTPHost = str("smtp_host")
	c.SMTPPort = str("smtp_port")
	c.SMTPUsername = str("smtp_username")
	// SMTPPassword intentionally empty
	c.EmailFrom = str("email_from")
	c.EmailTo = str("email_to")
	c.DefaultPBSID = str("default_pbs_id")

	if servers, ok := doc["pbs_servers"].(map[string]interface{}); ok {
		for id, srv := range servers {
			if m, ok := srv.(map[string]interface{}); ok {
				pbs := &PBSServer{
					ID:                id,
					Name:              strFrom(m, "name"),
					BaseURL:           strFrom(m, "baseurl"),
					CertFingerprint:   strFrom(m, "certfingerprint"),
					AuthID:            strFrom(m, "authid"),
					SecretSet:         boolFrom(m, "secret_set"),
					Username:          strFrom(m, "username"),
					PasswordSet:       boolFrom(m, "password_set"),
					Datastore:         strFrom(m, "datastore"),
					Namespace:         strFrom(m, "namespace"),
					EncryptionKeyFile: strFrom(m, "encryption_key_file"),
					Description:       strFrom(m, "description"),
				}
				c.PBSServers[id] = pbs
			}
		}
	}
	return c
}

func strFrom(m map[string]interface{}, k string) string {
	if v, ok := m[k].(string); ok {
		return v
	}
	return ""
}

func boolFrom(m map[string]interface{}, k string) bool {
	if v, ok := m[k].(bool); ok {
		return v
	}
	return false
}

func stringSlice(v interface{}) []string {
	if v == nil {
		return nil
	}
	var out []string
	if arr, ok := v.([]interface{}); ok {
		for _, x := range arr {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
	}
	return out
}
