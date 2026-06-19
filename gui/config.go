package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"security"
)

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
	Datastore       string `json:"datastore,omitempty"`
	Namespace       string `json:"namespace,omitempty"`

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

// getAPITokenPath is the shared local-API auth token file (H-01), placed next to
// config.json so the GUI and the privileged service resolve the same path.
func getAPITokenPath() string {
	dir, err := getConfigDir()
	if err != nil || dir == "" {
		return "nimbus-api-token"
	}
	return filepath.Join(dir, "api-token")
}

// getConfigDir returns the application's data directory, creating it if needed.
// On Windows it lives under ProgramData (shared GUI/Service); on Unix it's
// ~/.proxmox-backup-guardian. Used as the parent for config.json, the restore
// cache, and any other persistent state.
func getConfigDir() (string, error) {
	var configDir string

	if programData := os.Getenv("ProgramData"); programData != "" {
		// Windows: C:\ProgramData\NimbusBackup (accessible by both user and LocalSystem)
		configDir = filepath.Join(programData, "NimbusBackup")
	} else if systemDrive := os.Getenv("SystemDrive"); systemDrive != "" {
		// Windows fallback: if ProgramData not set, use C:\ProgramData hardcoded
		// This ensures service config is accessible even if env var is missing
		configDir = filepath.Join(systemDrive, "ProgramData", "NimbusBackup")
	} else {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		configDir = filepath.Join(homeDir, ".proxmox-backup-guardian")
	}

	// #nosec G703 -- ProgramData is a trusted Windows system environment variable, not user input
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
			ID:              "default",
			Name:            "Serveur PBS Principal",
			BaseURL:         config.BaseURL,
			CertFingerprint: config.CertFingerprint,
			AuthID:          config.AuthID,
			Secret:          config.Secret,
			Datastore:       config.Datastore,
			Namespace:       config.Namespace,
			Description:     "Serveur PBS par défaut (migré depuis ancienne config)",
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

	// Validate AuthID
	if c.AuthID == "" {
		return fmt.Errorf("authentication ID requis")
	}
	if err := security.ValidateAuthID(c.AuthID); err != nil {
		return fmt.Errorf("authentication ID invalide: %w", err)
	}

	// Validate Secret (non-empty check)
	if c.Secret == "" {
		return fmt.Errorf("secret requis")
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

// AddPBSServer adds a new PBS server to the configuration
func (c *Config) AddPBSServer(pbs *PBSServer) error {
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

	return c.Save()
}

// UpdatePBSServer updates an existing PBS server
func (c *Config) UpdatePBSServer(pbs *PBSServer) error {
	if err := pbs.Validate(); err != nil {
		return err
	}

	if _, exists := c.PBSServers[pbs.ID]; !exists {
		return fmt.Errorf("serveur PBS '%s' introuvable", pbs.ID)
	}

	c.PBSServers[pbs.ID] = pbs
	return c.Save()
}

// DeletePBSServer removes a PBS server from the configuration
func (c *Config) DeletePBSServer(id string) error {
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
		return fmt.Errorf("serveur PBS '%s' introuvable", id)
	}

	c.DefaultPBSID = id
	return c.Save()
}
