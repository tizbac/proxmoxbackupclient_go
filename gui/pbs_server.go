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
	Datastore       string `json:"datastore"`
	Namespace       string `json:"namespace"`
	Description     string `json:"description,omitempty"` // Optional description
	IsOnline        bool   `json:"is_online,omitempty"`   // Connection status (updated by GUI)
	SecretSet       bool   `json:"secret_set,omitempty"`  // M-04: set on sanitized copies so the UI knows a token exists without receiving it
}

// sanitized returns a copy with the secret stripped and SecretSet set, for
// handing PBS server records to the frontend without leaking the token (M-04).
func (pbs *PBSServer) sanitized() *PBSServer {
	c := *pbs
	c.SecretSet = pbs.Secret != ""
	c.Secret = ""
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

	// Validate AuthID
	if pbs.AuthID == "" {
		return fmt.Errorf("authentication ID requis")
	}
	if err := security.ValidateAuthID(pbs.AuthID); err != nil {
		return fmt.Errorf("authentication ID invalide: %w", err)
	}

	// Validate Secret (non-empty check)
	if pbs.Secret == "" {
		return fmt.Errorf("secret requis")
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
		Datastore:       pbs.Datastore,
		Namespace:       pbs.Namespace,
	}
}
