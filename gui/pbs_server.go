package main

import (
	"fmt"
	"security"
)

// resolvePBS picks which configured PBS server to use — for a restore, or a
// backup too. When pbsID is empty the default PBS server is used. Falls back
// to legacy single-server fields when no multi-PBS entry is configured.
// Shared (no build tag) so both the GUI and the service build resolve a
// destination server identically — main.go's RestoreSnapshot/StartBackup and
// app_service_stubs.go's StartBackup both call this same implementation.
func (a *App) resolvePBS(pbsID string) (*Config, error) {
	if a.delegated() {
		return a.serviceAccess(pbsID, false)
	}
	if pbsID != "" {
		pbs, err := a.config.GetPBSServer(pbsID)
		if err != nil {
			return nil, err
		}
		return a.withAuth(pbs.ToConfig())
	}
	// A copy, never the app's own config: with a single legacy server
	// EffectivePBS returns the config itself, and each run unlocks its key
	// into the result (Crypt). Shared, a run whose key could not be unlocked
	// cleared it for another starting at the same moment, which then
	// uploaded unencrypted.
	cp := *a.config.EffectivePBS()
	cfg := &cp
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return a.withAuth(cfg)
}

// checkServiceKey applies checkKeyUsableByService to the server a service-routed
// backup will use. It only reads the stored settings, never contacts the server.
func (a *App) checkServiceKey(pbsID string) error {
	cfg := a.config.EffectivePBS()
	if pbsID != "" {
		pbs, err := a.config.GetPBSServer(pbsID)
		if err != nil {
			return nil
		}
		cfg = pbs.ToConfig()
	}
	return checkKeyUsableByService(cfg.EncryptionKeyFile, cfg.EncryptionKeyPassphrase)
}

// resolveRestorePBS is resolvePBS plus unlocking the configured encryption
// key. An encrypted snapshot's chunks are unreadable without it, so fail here
// with a clear message rather than deep inside the chunk fetcher.
func (a *App) resolveRestorePBS(pbsID string) (*Config, error) {
	var cfg *Config
	var err error
	if a.delegated() {
		cfg, err = a.serviceAccess(pbsID, true)
	} else {
		cfg, err = a.resolvePBS(pbsID)
	}
	if err != nil {
		return nil, err
	}
	if err := cfg.loadCryptConfig(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// serviceAccess is resolvePBS with the service holding the settings: the
// service hands what this one operation needs (a fresh ticket, or a token
// server's token, and with withKey the key file's contents and remembered
// passphrase), kept in this Config only, never written anywhere.
func (a *App) serviceAccess(pbsID string, withKey bool) (*Config, error) {
	acc, err := a.apiClient.PBSAccess(pbsID, withKey)
	if err != nil {
		return nil, err
	}
	return &Config{
		BaseURL:                 acc.BaseURL,
		CertFingerprint:         acc.CertFingerprint,
		Datastore:               acc.Datastore,
		Namespace:               acc.Namespace,
		AuthID:                  acc.AuthID,
		Secret:                  acc.Secret,
		Username:                acc.Username,
		Ticket:                  acc.Ticket,
		CSRFToken:               acc.CSRFToken,
		EncryptionKeyFile:       acc.EncryptionKeyFile,
		EncryptionKeyPassphrase: acc.KeyPassphrase,
		keyFileData:             acc.KeyFileData,
	}, nil
}

// PBSServer represents a single Proxmox Backup Server configuration
type PBSServer struct {
	ID              string `json:"id"`   // Unique identifier (e.g., "pbs1", "default")
	Name            string `json:"name"` // Human-readable name (e.g., "Big Data Storage")
	BaseURL         string `json:"baseurl"`
	CertFingerprint string `json:"certfingerprint"`
	AuthID          string `json:"authid"`
	Secret          string `json:"secret"`
	// Username/password authentication (alternative to the API token). Both are
	// persisted so the GUI can mint a FRESH, short-lived PBS ticket at the start
	// of every operation — PBS tickets expire, so we store the credentials and
	// never a ticket. A server uses EITHER (AuthID+Secret) OR (Username+Password).
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	// EncryptionKeyFile is the path to the PBS encryption key (JSON, as written
	// by `proxmox-backup-client key create`) to use for this server. It is kept
	// per-server because two PBS servers rarely share keys, and it is NOT a
	// secret: only the path is stored here, and the unlocked key lives in the
	// runtime-only Config.Crypt. Left empty, backups to this server are
	// unencrypted (and encrypted snapshots stored there cannot be restored).
	EncryptionKeyFile string `json:"encryption_key_file,omitempty"`
	// EncryptionKeyPassphrase / ...Set / Clear: see the same fields on Config.
	EncryptionKeyPassphrase      string `json:"encryption_key_passphrase,omitempty"`
	EncryptionKeyPassphraseSet   bool   `json:"encryption_key_passphrase_set,omitempty"`
	ClearEncryptionKeyPassphrase bool   `json:"clear_encryption_key_passphrase,omitempty"`
	Datastore                    string `json:"datastore"`
	Namespace                    string `json:"namespace"`
	Description                  string `json:"description,omitempty"` // Optional description
	IsOnline                     bool   `json:"is_online,omitempty"`   // Connection status (updated by GUI)
	SecretSet                    bool   `json:"secret_set,omitempty"`  // M-04: set on sanitized copies so the UI knows a token exists without receiving it
	PasswordSet                  bool   `json:"password_set,omitempty"`
	// CompressEncrypted is the "Compress encrypted backups" switch
	// (compress_encrypted.go): nil or true compresses encrypted chunks before
	// they are encrypted, false writes them as version 2.0 and older did.
	CompressEncrypted *bool `json:"compress_encrypted,omitempty"`
	// CompressEncryptedAsk marks a switch this version turned off itself when
	// it first read settings of an earlier version, until the user has
	// answered the question the app then asks once (turn it on, or keep it
	// off) or has changed the switch.
	CompressEncryptedAsk bool `json:"compress_encrypted_ask,omitempty"`
}

// sanitized returns a copy with the secret and password stripped (SecretSet /
// PasswordSet set), for handing PBS server records to the frontend without
// leaking credentials (M-04).
func (pbs *PBSServer) sanitized() *PBSServer {
	c := *pbs
	c.SecretSet = pbs.Secret != ""
	c.PasswordSet = pbs.Password != ""
	c.Secret = ""
	c.EncryptionKeyPassphraseSet = pbs.EncryptionKeyPassphrase != ""
	c.EncryptionKeyPassphrase = ""
	c.Password = "" // never hand the credentials to the frontend
	return &c
}

// Validate checks if the PBS server configuration is valid
func (pbs *PBSServer) Validate() error {
	// Validate ID
	if pbs.ID == "" {
		return fmt.Errorf("PBS server ID required")
	}

	// Validate Name
	if pbs.Name == "" {
		return fmt.Errorf("PBS server name required")
	}

	// Validate BaseURL
	if pbs.BaseURL == "" {
		return fmt.Errorf("PBS server URL required")
	}
	if err := security.ValidateURL(pbs.BaseURL); err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	// Auth: a server is configured with EITHER an API token (AuthID+Secret)
	// OR a username (the password is stored too). At least one must be present;
	// if a token is present its secret must be too. The password itself is
	// checked where the server is added/updated (it may be blank on update to
	// mean "keep the stored password").
	hasToken := pbs.AuthID != ""
	hasUser := pbs.Username != ""
	if hasToken {
		if err := security.ValidateAuthID(pbs.AuthID); err != nil {
			return fmt.Errorf("invalid authentication ID: %w", err)
		}
		if pbs.Secret == "" {
			return fmt.Errorf("secret required")
		}
	} else if !hasUser {
		return fmt.Errorf("API token (authid/secret) or username/password required")
	}

	// Validate Datastore
	if pbs.Datastore == "" {
		return fmt.Errorf("datastore required")
	}
	if err := security.ValidateDatastore(pbs.Datastore); err != nil {
		return fmt.Errorf("invalid datastore: %w", err)
	}

	// Validate Certificate Fingerprint if present
	if pbs.CertFingerprint != "" {
		if err := security.ValidateFingerprint(pbs.CertFingerprint); err != nil {
			return fmt.Errorf("invalid certificate fingerprint: %w", err)
		}
	}

	// Reject an unusable key file at add/update time, not at backup time. Uses
	// a throwaway Config so the per-server and legacy paths validate identically.
	if err := (&Config{EncryptionKeyFile: pbs.EncryptionKeyFile, EncryptionKeyPassphrase: pbs.EncryptionKeyPassphrase}).validateEncryptionKeyFile(); err != nil {
		return err
	}

	return nil
}

// ToConfig converts a PBSServer to the legacy Config format (for backward compatibility)
func (pbs *PBSServer) ToConfig() *Config {
	return &Config{
		BaseURL:                 pbs.BaseURL,
		CertFingerprint:         pbs.CertFingerprint,
		AuthID:                  pbs.AuthID,
		Secret:                  pbs.Secret,
		Username:                pbs.Username,
		Password:                pbs.Password,
		EncryptionKeyFile:       pbs.EncryptionKeyFile,
		EncryptionKeyPassphrase: pbs.EncryptionKeyPassphrase,
		CompressEncrypted:       pbs.CompressEncrypted,
		Datastore:               pbs.Datastore,
		Namespace:               pbs.Namespace,
	}
}
