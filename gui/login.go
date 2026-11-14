package main

import (
	"fmt"

	"pbscommon"
)

// withAuth returns a Config ready for a PBS operation.
//
// PBS session tickets are short-lived (server-side TTL) and revocable, so we
// deliberately NEVER persist one nor hold it between operations. For
// username/password servers we mint a FRESH ticket right here, per operation,
// and overlay it on a shallow copy of cfg so a PBSClient built from the result
// authenticates via the PBSAuthCookie. The receiver's stored config is never
// mutated. API-token servers need no ticket and are returned unchanged, as is
// a config with no credentials at all.
func (a *App) withAuth(cfg *Config) (*Config, error) {
	if cfg == nil || cfg.BaseURL == "" {
		return cfg, nil
	}
	// API-token server: authenticate with the token, nothing to mint.
	if cfg.AuthID != "" {
		return cfg, nil
	}
	// Username/password server: obtain a fresh ticket now.
	if cfg.Username != "" && cfg.Password != "" {
		client := &pbscommon.PBSClient{
			BaseURL:         cfg.BaseURL,
			CertFingerPrint: cfg.CertFingerprint,
			Username:        cfg.Username,
			Password:        cfg.Password,
			Datastore:       cfg.Datastore,
			Namespace:       cfg.Namespace,
			Insecure:        cfg.CertFingerprint != "",
		}
		if err := client.ObtainTicket(); err != nil {
			return cfg, fmt.Errorf("username/password authentication failed: %w", err)
		}
		cp := *cfg
		cp.Ticket = client.Ticket
		cp.CSRFToken = client.CSRFToken
		return &cp, nil
	}
	return cfg, nil
}

// ticketRenewer returns how a backup gets a fresh ticket for cfg's
// username/password login when PBS refuses the one it started with (a ticket
// lasts 2 hours; the snapshot's comment is set after the backup), or nil for
// an API token or a config without a login. It mints the ticket as withAuth
// does, only when asked.
func (a *App) ticketRenewer(cfg *Config) func() (ticket, csrf string, err error) {
	if cfg == nil || cfg.AuthID != "" || cfg.Username == "" || cfg.Password == "" {
		return nil
	}
	login := *cfg
	login.Ticket, login.CSRFToken = "", ""
	return func() (string, string, error) {
		fresh, err := a.withAuth(&login)
		if err != nil {
			return "", "", err
		}
		return fresh.Ticket, fresh.CSRFToken, nil
	}
}
