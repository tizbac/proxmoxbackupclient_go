package main

// service_api.go implements the service-side half of the GUI↔service config
// round-trip: reading and writing the (sanitized) configuration document,
// testing PBS servers, minting short-lived PBS tickets, and exposing the job
// history. The file is compiled into BOTH builds (no build tag) so the GUI
// binary satisfies the same api.BackupHandler interface as the service; the
// GUI only ever *calls* these over HTTP when it runs in service mode, where
// the service is the single privileged owner/writer of config.json and the
// job files. A standalone GUI uses its local Config directly instead.

import (
	"encoding/json"
	"fmt"
	"os"
	"pbscommon"

	"github.com/tizbac/proxmoxbackupclient_go/gui/api"
)

// GetFullConfigForAPI returns the full configuration document as the GUI sees
// it in service mode: every persisted field, PBS servers included, but with
// all secrets replaced by *_set boolean markers (M-04). The hostname is added
// so the GUI can pre-fill the backup ID exactly like its local variant does.
func (a *App) GetFullConfigForAPI() map[string]interface{} {
	hostname, _ := os.Hostname()
	doc := map[string]interface{}{
		"hostname":          hostname,
		"secret_set":        a.config.Secret != "",
		"smtp_password_set": a.config.SMTPPassword != "",
	}

	// JSON round-trip of the sanitized config: the struct tags define the
	// wire format, sanitized() strips every secret and sets the per-server
	// SecretSet/PasswordSet markers.
	if b, err := json.Marshal(a.config.sanitized()); err == nil {
		var m map[string]interface{}
		if json.Unmarshal(b, &m) == nil {
			for k, v := range m {
				if k == "hostname" {
					continue
				}
				doc[k] = v
			}
		}
	}
	return doc
}

// SaveFullConfigFromAPI merges a whole configuration document (as produced by
// GetFullConfigForAPI plus GUI edits) into the stored config and persists it.
// The service is the single writer of config.json.
//
// Credential semantics, per entry:
//   - an empty secret / password means "keep the currently stored value":
//     the GUI never receives the stored credentials, so retention is the only
//     possible meaning (same convention as Config.UpdatePBSServerMem);
//   - a credential the GUI DOES know (typed in this session) is transmitted
//     and replaces the stored one;
//   - the auth method is taken from the entry itself (authid ⇒ token,
//     otherwise user/password), so switching method drops the other one's
//     stored credentials instead of leaving them behind.
//
// Validation runs on a COPY: a rejected document leaves the live config (and
// config.json) untouched.
func (a *App) SaveFullConfigFromAPI(doc map[string]interface{}) error {
	if doc == nil {
		return fmt.Errorf("document de configuration vide")
	}

	b, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("document de configuration invalide: %w", err)
	}

	// Unmarshal into a Config: unknown keys (the *_set markers, hostname) are
	// ignored, and json:"-" fields (Username/Password/Ticket/CSRFToken) are
	// never populated from the wire.
	var incoming Config
	if err := json.Unmarshal(b, &incoming); err != nil {
		return fmt.Errorf("document de configuration invalide: %w", err)
	}
	// Config.Username/Password are json:"-" (runtime-only), so a client that
	// does send legacy user/password credentials would otherwise have them
	// silently dropped — and the check below would then always claim that only
	// an authid is acceptable. Read them from the raw document instead.
	var creds struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	_ = json.Unmarshal(b, &creds)

	cfg := *a.config // shallow copy: never mutate the live config before validation

	// --- Legacy single-PBS fields (retention on empty secrets) -------------
	cfg.BaseURL = incoming.BaseURL
	cfg.CertFingerprint = incoming.CertFingerprint
	cfg.AuthID = incoming.AuthID
	if incoming.Secret != "" {
		cfg.Secret = incoming.Secret
	}
	if creds.Username != "" {
		cfg.Username = creds.Username
	}
	if creds.Password != "" {
		cfg.Password = creds.Password
	}
	if cfg.AuthID != "" {
		// One legacy method too: a token entry must never keep a stale
		// user/password beside it (and vice versa, see the check below).
		cfg.Username = ""
		cfg.Password = ""
	}
	cfg.Datastore = incoming.Datastore
	cfg.Namespace = incoming.Namespace

	// --- Backup settings ----------------------------------------------------
	cfg.BackupDir = incoming.BackupDir
	cfg.BackupID = incoming.BackupID
	cfg.UseVSS = incoming.UseVSS
	cfg.LastBackupDirs = incoming.LastBackupDirs
	cfg.DisableSplit = incoming.DisableSplit
	cfg.SplitSizeGB = incoming.SplitSizeGB
	// The key PATH is not a secret and the GUI knows it, so the document is the
	// source of truth: keeping it out of the merge would silently drop the
	// encryption key on the next push and stop encrypting backups.
	cfg.EncryptionKeyFile = incoming.EncryptionKeyFile
	if err := cfg.validateEncryptionKeyFile(); err != nil {
		return err
	}

	// --- Email notifications -------------------------------------------------
	cfg.SMTPHost = incoming.SMTPHost
	cfg.SMTPPort = incoming.SMTPPort
	cfg.SMTPUsername = incoming.SMTPUsername
	if incoming.SMTPPassword != "" {
		cfg.SMTPPassword = incoming.SMTPPassword
	}
	cfg.EmailFrom = incoming.EmailFrom
	cfg.EmailTo = incoming.EmailTo

	// --- Multi-PBS servers: whole-map replace, per-server credential
	// retention + auth-method resolution -------------------------------------
	// A server missing from the document was deleted by the GUI. Only replace
	// when the document actually carries the key: a client that omits
	// pbs_servers must not wipe the stored servers.
	if _, present := doc["pbs_servers"]; present {
		old := cfg.PBSServers
		next := make(map[string]*PBSServer, len(incoming.PBSServers))
		for id, srv := range incoming.PBSServers {
			if srv == nil {
				continue
			}
			srv.ID = id
			srv.IsOnline = false  // connection state is runtime-only, never persisted
			srv.SecretSet = false // *_set markers are derived when read, never stored
			srv.PasswordSet = false
			// Chooses token vs user/password, keeps the stored credential when
			// the incoming one is empty, and requires the selected method to
			// actually have a usable credential (the service holds them all).
			if err := normalizeServerAuth(old[id], srv, true); err != nil {
				return fmt.Errorf("serveur PBS %q: %w", id, err)
			}
			if err := srv.Validate(); err != nil {
				return fmt.Errorf("serveur PBS %q: %w", id, err)
			}
			next[id] = srv
		}
		cfg.PBSServers = next
	}

	// --- Default server -------------------------------------------------------
	cfg.DefaultPBSID = incoming.DefaultPBSID
	if cfg.DefaultPBSID != "" {
		if _, ok := cfg.PBSServers[cfg.DefaultPBSID]; !ok {
			cfg.DefaultPBSID = ""
		}
	}
	if cfg.DefaultPBSID == "" && len(cfg.PBSServers) == 1 {
		for id := range cfg.PBSServers {
			cfg.DefaultPBSID = id
			break
		}
	}

	// --- Field validation (per part; the legacy Validate() would wrongly
	// require a legacy BaseURL on pure multi-PBS configs) ----------------------
	if err := cfg.validatePBSFields(); err != nil {
		return err
	}

	if err := cfg.Save(); err != nil {
		return err
	}
	a.config = &cfg
	return nil
}

// applyPBSDraft merges non-empty string fields of a draft document onto a PBS
// server. It backs TestPBSServerForAPI, where the GUI can test unsaved form
// edits against the server before persisting them.
func applyPBSDraft(srv *PBSServer, draft map[string]interface{}) {
	str := func(k string) string {
		if v, ok := draft[k].(string); ok {
			return v
		}
		return ""
	}
	// A draft may SWITCH auth method (the frontend sends only the fields of
	// the active tab): username without authid means the user/password method,
	// so the stored token has to go first — otherwise normalizeServerAuth would
	// keep the token and silently discard the draft's username. The opposite
	// direction needs nothing here: normalizeServerAuth drops user/password as
	// soon as an authid is present.
	if str("username") != "" && str("authid") == "" {
		srv.AuthID = ""
		srv.Secret = ""
	}
	if v := str("name"); v != "" {
		srv.Name = v
	}
	if v := str("baseurl"); v != "" {
		srv.BaseURL = v
	}
	if v := str("certfingerprint"); v != "" {
		srv.CertFingerprint = v
	}
	if v := str("authid"); v != "" {
		srv.AuthID = v
	}
	if v := str("secret"); v != "" {
		srv.Secret = v
	}
	if v := str("username"); v != "" {
		srv.Username = v
	}
	if v := str("password"); v != "" {
		srv.Password = v
	}
	if v := str("datastore"); v != "" {
		srv.Datastore = v
	}
	if v := str("namespace"); v != "" {
		srv.Namespace = v
	}
	if v := str("description"); v != "" {
		srv.Description = v
	}
}

// TestPBSServerForAPI checks connectivity (DNS, TLS, auth, datastore access)
// to a stored PBS server, first merging any non-empty draft fields so the GUI
// can validate unsaved form edits. The test runs entirely inside the service,
// where the credentials live.
func (a *App) TestPBSServerForAPI(id string, draft map[string]interface{}) error {
	var base *PBSServer
	if id != "" {
		stored, err := a.config.GetPBSServer(id)
		if err != nil {
			return err
		}
		cp := *stored
		base = &cp
	} else {
		base = &PBSServer{}
	}

	applyPBSDraft(base, draft)

	// Same resolution as every save path: pick the method the draft asked for,
	// treat an empty credential as "keep the stored one" (only for that exact
	// token/user) and require a usable credential afterwards — the service
	// holds the stored secrets, so requireCreds applies.
	stored, _ := a.config.GetPBSServer(id)
	if err := normalizeServerAuth(stored, base, true); err != nil {
		return err
	}
	if err := base.Validate(); err != nil {
		return err
	}

	cfg, err := a.withAuth(base.ToConfig())
	if err != nil {
		return err
	}

	client := &pbscommon.PBSClient{
		BaseURL:          cfg.BaseURL,
		CertFingerPrint:  cfg.CertFingerprint,
		AuthID:           cfg.AuthID,
		Secret:           cfg.Secret,
		Username:         cfg.Username,
		Password:         cfg.Password,
		Ticket:           cfg.Ticket,
		CSRFToken:        cfg.CSRFToken,
		Datastore:        cfg.Datastore,
		Namespace:        cfg.Namespace,
		Insecure:         cfg.CertFingerprint != "",
		CompressionLevel: pbscommon.CompressionFastest,
		Manifest: pbscommon.BackupManifest{
			BackupID: cfg.BackupID,
		},
	}
	if err := client.TestConnection(); err != nil {
		return err
	}
	return nil
}

// MintPBSTicketForAPI mints a short-lived PBS session ticket for the given
// server (empty id = default server) and returns it with the non-sensitive
// connection parameters. The GUI uses the ticket for restore/listing
// operations and never sees the PBS credentials:
//   - user/password servers: classic username/password ticket;
//   - API-token servers: the token ID + secret are exchanged at
//     /access/ticket (the same endpoint the PBS web UI uses for token login),
//     yielding a cookie ticket the GUI can use without the secret.
func (a *App) MintPBSTicketForAPI(id string) (*api.PBSTicket, error) {
	srv, err := a.config.GetPBSServer(id)
	if err != nil {
		return nil, err
	}

	cfg := srv.ToConfig()

	if cfg.AuthID != "" {
		// Token server: mint the ticket from the token itself.
		if cfg.Secret == "" {
			return nil, fmt.Errorf("serveur %q: secret manquant", srv.ID)
		}
		client := &pbscommon.PBSClient{
			BaseURL:         cfg.BaseURL,
			CertFingerPrint: cfg.CertFingerprint,
			Username:        cfg.Username,
			Password:        cfg.Password,
			Insecure:        cfg.CertFingerprint != "",
		}
		if err := client.ObtainTicket(); err != nil {
			return nil, fmt.Errorf("émission du ticket impossible: %w", err)
		}
		cfg.Ticket = client.Ticket
		cfg.CSRFToken = client.CSRFToken
	} else {
		// User/password server (or none): let withAuth mint a fresh ticket.
		cfg, err = a.withAuth(cfg)
		if err != nil {
			return nil, err
		}
	}

	if cfg.Ticket == "" {
		return nil, fmt.Errorf("serveur %q: aucun ticket obtenu", srv.ID)
	}

	return &api.PBSTicket{
		Ticket:          cfg.Ticket,
		CSRFToken:       cfg.CSRFToken,
		BaseURL:         cfg.BaseURL,
		CertFingerprint: cfg.CertFingerprint,
		Datastore:       cfg.Datastore,
		Namespace:       cfg.Namespace,
	}, nil
}

// GetJobHistoryForAPI returns the scheduler's job history as generic maps so
// the GUI's history view works identically in service and standalone mode.
func (a *App) GetJobHistoryForAPI() ([]map[string]interface{}, error) {
	history, err := a.GetJobHistory()
	if err != nil {
		return nil, err
	}
	if len(history) == 0 {
		return []map[string]interface{}{}, nil
	}
	b, err := json.Marshal(history)
	if err != nil {
		return nil, err
	}
	var out []map[string]interface{}
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}
