package main

import (
	"fmt"
	"security"
)

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
	Datastore         string `json:"datastore"`
	Namespace         string `json:"namespace"`
	Description       string `json:"description,omitempty"` // Optional description
	IsOnline          bool   `json:"is_online,omitempty"`   // Connection status (updated by GUI)
	SecretSet         bool   `json:"secret_set,omitempty"`  // M-04: set on sanitized copies so the UI knows a token exists without receiving it
	PasswordSet       bool   `json:"password_set,omitempty"`
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

// normalizeServerAuth picks the authentication method of a PBS server entry
// and applies the "empty credential = keep the stored one" convention used by
// the whole GUI↔service config round-trip: the frontend never receives the
// stored secret/password, so it can only ever send "" (keep) or a newly typed
// value (replace).
//
// Exactly one method survives, so switching method (token ↔ user/password)
// also drops the credentials of the other one instead of leaving a stale token
// secret behind on a user/password entry (and vice versa):
//
//   - authid present → API-token method: username/password are dropped, and an
//     empty secret keeps the stored one ONLY when the authid is unchanged — a
//     secret belongs to exactly one token, so a changed authid without a new
//     secret is an error, never a silent reuse of the old secret;
//   - no authid      → user/password method: authid/secret are dropped, and an
//     empty password keeps the stored one ONLY when the username is unchanged.
//
// requireCreds distinguishes the callers: the service and a standalone GUI
// hold the stored credentials and can tell "missing" from "keep" (true). The
// service-mode GUI never receives them (M-04), so it sends "" for an unknown
// credential and the final presence check happens in the service, where the
// real secrets live (false).
func normalizeServerAuth(existing, srv *PBSServer, requireCreds bool) error {
	if srv == nil {
		return fmt.Errorf("serveur PBS vide")
	}

	if srv.AuthID != "" {
		// API-token method wins: user/password must not survive on a token entry.
		srv.Username = ""
		srv.Password = ""
		if srv.Secret == "" {
			switch {
			case existing != nil && existing.AuthID == srv.AuthID:
				// Same token as stored: keep the stored secret (may be "" when the
				// caller never received it — the service fills it in).
				srv.Secret = existing.Secret
			case requireCreds:
				return fmt.Errorf("secret requis pour l'authentification ID %q (un secret n'est pas reutilisable avec un autre authid)", srv.AuthID)
			}
		}
		if srv.Secret == "" && requireCreds {
			return fmt.Errorf("secret requis pour l'authentification ID %q", srv.AuthID)
		}
		return nil
	}

	// User/password method wins: the token must not survive on a user/pass entry.
	srv.AuthID = ""
	srv.Secret = ""
	if srv.Username == "" {
		return fmt.Errorf("API token (authid/secret) ou identifiant/mot de passe requis")
	}
	if srv.Password == "" && existing != nil && existing.Username == srv.Username {
		// Same user as stored: keep the stored password ("" when unknown to this
		// caller — the service fills it in).
		srv.Password = existing.Password
	}
	if srv.Password == "" && requireCreds {
		return fmt.Errorf("mot de passe requis pour l'utilisateur %q", srv.Username)
	}
	return nil
}

// validateFields checks every field of a PBS server EXCEPT credential presence
// (see normalizeServerAuth for why presence is a separate, caller-dependent
// check): ID, name, URL, authid format, that exactly one auth method is
// selected, datastore, fingerprint and encryption key file.
func (pbs *PBSServer) validateFields() error {
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

	// Auth: a server authenticates with EITHER an API token (AuthID+Secret) OR
	// a username/password — never both, and never neither.
	if pbs.AuthID != "" {
		if err := security.ValidateAuthID(pbs.AuthID); err != nil {
			return fmt.Errorf("authentication ID invalide: %w", err)
		}
	} else if pbs.Username == "" {
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

	// Reject an unusable key file at add/update time, not at backup time. Uses
	// a throwaway Config so the per-server and legacy paths validate identically.
	if err := (&Config{EncryptionKeyFile: pbs.EncryptionKeyFile}).validateEncryptionKeyFile(); err != nil {
		return err
	}

	return nil
}

// Validate checks if the PBS server configuration is valid, including that the
// selected auth method carries its credential. It is the gate for callers that
// hold the stored credentials (standalone GUI, service); the service-mode GUI
// goes through normalizeServerAuth(..., false) + validate(false), because it
// never receives the stored secret/password.
func (pbs *PBSServer) Validate() error {
	return pbs.validate(true)
}

// validate applies validateFields plus, when requireCreds is set, the presence
// of the selected method's credential. A user/password entry whose password is
// absent is reported by normalizeServerAuth (which is retention-aware), so this
// only re-checks the token secret for callers that skipped that helper.
func (pbs *PBSServer) validate(requireCreds bool) error {
	if err := pbs.validateFields(); err != nil {
		return err
	}
	if requireCreds && pbs.AuthID != "" && pbs.Secret == "" {
		return fmt.Errorf("secret requis")
	}
	return nil
}

// ToConfig converts a PBSServer to the legacy Config format (for backward compatibility)
func (pbs *PBSServer) ToConfig() *Config {
	return &Config{
		BaseURL:           pbs.BaseURL,
		CertFingerprint:   pbs.CertFingerprint,
		AuthID:            pbs.AuthID,
		Secret:            pbs.Secret,
		Username:          pbs.Username,
		Password:          pbs.Password,
		EncryptionKeyFile: pbs.EncryptionKeyFile,
		Datastore:         pbs.Datastore,
		Namespace:         pbs.Namespace,
	}
}
