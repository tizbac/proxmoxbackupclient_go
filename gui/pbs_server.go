package main

import (
	"fmt"
	"security"
)

// PBSServer represents a single Proxmox Backup Server configuration
type PBSServer struct {
	ID              string `json:"id"`                // Unique identifier (e.g., "pbs1", "default")
	Name            string `json:"name"`              // Human-readable name (e.g., "Big Data Storage")
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
	Datastore       string `json:"datastore"`
	Namespace       string `json:"namespace"`
	Description     string `json:"description,omitempty"` // Optional description
	IsOnline        bool   `json:"is_online,omitempty"`   // Connection status (updated by GUI)
	SecretSet       bool   `json:"secret_set,omitempty"`  // M-04: set on sanitized copies so the UI knows a token exists without receiving it
	PasswordSet     bool   `json:"password_set,omitempty"`
}

// sanitized returns a copy with the secret and password stripped (SecretSet /
// PasswordSet set), for handing PBS server records to the frontend without
// leaking credentials (M-04).
func (pbs *PBSServer) sanitized() *PBSServer {
	c := *pbs
	c.SecretSet = pbs.Secret != ""
	c.PasswordSet = pbs.Password != ""
	c.Secret = ""
	c.Password = "" // never hand the credentials to the frontend
	return &c
}

// Validate checks if the PBS server configuration is valid
func (pbs *PBSServer) Validate() error {
	// Validate ID
	if pbs.ID == "" {
		return fmt.Errorf("PBS server ID requis")
	}

	// Validate Name
	if pbs.Name == "" {
		return fmt.Errorf("PBS server name requis")
	}

	// Validate BaseURL
	if pbs.BaseURL == "" {
		return fmt.Errorf("URL du serveur PBS requis")
	}
	if err := security.ValidateURL(pbs.BaseURL); err != nil {
		return fmt.Errorf("URL invalide: %w", err)
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
			return fmt.Errorf("authentication ID invalide: %w", err)
		}
		if pbs.Secret == "" {
			return fmt.Errorf("secret requis")
		}
	} else if !hasUser {
		return fmt.Errorf("API token (authid/secret) ou identifiant/mot de passe requis")
	}

	// Validate Datastore
	if pbs.Datastore == "" {
		return fmt.Errorf("datastore requis")
	}
	if err := security.ValidateDatastore(pbs.Datastore); err != nil {
		return fmt.Errorf("datastore invalide: %w", err)
	}

	// Validate Certificate Fingerprint if present
	if pbs.CertFingerprint != "" {
		if err := security.ValidateFingerprint(pbs.CertFingerprint); err != nil {
			return fmt.Errorf("empreinte certificat invalide: %w", err)
		}
	}

	return nil
}

// ToConfig converts a PBSServer to the legacy Config format (for backward compatibility)
func (pbs *PBSServer) ToConfig() *Config {
	return &Config{
		BaseURL:         pbs.BaseURL,
		CertFingerprint: pbs.CertFingerprint,
		AuthID:          pbs.AuthID,
		Secret:          pbs.Secret,
		Username:        pbs.Username,
		Password:        pbs.Password,
		Datastore:       pbs.Datastore,
		Namespace:       pbs.Namespace,
	}
}
