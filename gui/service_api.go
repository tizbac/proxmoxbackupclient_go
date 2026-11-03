package main

// The service's side of the API an app without elevation works through
// (upstream's strict model; gui/api/server_state.go). Compiled into both
// programs; only the service is asked, and it runs everything here against
// its own settings, the computer's.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/tizbac/proxmoxbackupclient_go/gui/api"
)

// remarshal converts in to out through JSON (a map to the app's own type).
func remarshal(in, out interface{}) error {
	b, err := json.Marshal(in)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, out)
}

// GetFullConfigForAPI is the settings document the app shows: every setting,
// no secret (sanitized), with *_set markers where one is stored.
func (a *App) GetFullConfigForAPI() map[string]interface{} {
	doc := map[string]interface{}{}
	if err := remarshal(a.config.sanitized(), &doc); err != nil {
		writeDebugLog(fmt.Sprintf("[API] settings document: %v", err))
	}
	doc["secret_set"] = a.config.Secret != ""
	doc["smtp_password_set"] = a.config.SMTPPassword != ""
	return doc
}

// PBSAccessForAPI gives the app what one PBS operation on server req.ID ("":
// the default) needs: a fresh ticket for a username server (never its
// password), the token for a token server (PBS issues no ticket for one),
// and, when asked, the encryption key's file and remembered passphrase.
func (a *App) PBSAccessForAPI(req api.PBSAccessRequest) (*api.PBSAccess, error) {
	cfg, err := a.resolvePBS(req.ID)
	if err != nil {
		return nil, err
	}
	if cfg == nil || cfg.BaseURL == "" {
		return nil, fmt.Errorf("no PBS server is set up")
	}
	acc := &api.PBSAccess{
		BaseURL:         cfg.BaseURL,
		CertFingerprint: cfg.CertFingerprint,
		Datastore:       cfg.Datastore,
		Namespace:       cfg.Namespace,
	}
	switch {
	case cfg.AuthID != "":
		acc.AuthID, acc.Secret = cfg.AuthID, cfg.Secret
	case cfg.Ticket != "":
		acc.Username, acc.Ticket, acc.CSRFToken = cfg.Username, cfg.Ticket, cfg.CSRFToken
	default:
		return nil, fmt.Errorf("the PBS server has no token or password stored")
	}
	if req.WithKey && cfg.EncryptionKeyFile != "" {
		data, err := os.ReadFile(cfg.EncryptionKeyFile) // #nosec G304 -- the key path the settings name
		if err != nil {
			return nil, fmt.Errorf("reading the encryption key %s: %w", cfg.EncryptionKeyFile, err)
		}
		acc.EncryptionKeyFile = cfg.EncryptionKeyFile
		acc.KeyFileData = data
		acc.KeyPassphrase = cfg.EncryptionKeyPassphrase
	}
	return acc, nil
}

// GetJobHistoryForAPI is the run history, as the Reports page reads it.
func (a *App) GetJobHistoryForAPI() ([]map[string]interface{}, error) {
	h, err := a.GetJobHistory()
	if err != nil {
		return nil, err
	}
	var out []map[string]interface{}
	err = remarshal(h, &out)
	return out, err
}

// PublicServersForAPI is the computer's PBS servers for GET /servers/public
// (api.PublicServersAPI), which anyone on the computer may read: where each
// server is and how it is reached, sorted by name. No token ID, username,
// secret, password or key file: a user on their own settings picks one and
// logs in with their own.
func (a *App) PublicServersForAPI() []api.PublicServer {
	return publicServers(a.config)
}

// publicServers is cfg's PBS servers without their logins, sorted by name
// (then ID).
func publicServers(cfg *Config) []api.PublicServer {
	list := []api.PublicServer{}
	if cfg == nil {
		return list
	}
	for id, s := range cfg.PBSServers {
		if s == nil {
			continue
		}
		login := "token"
		if s.AuthID == "" && s.Username != "" {
			login = "password"
		}
		sid := s.ID
		if sid == "" {
			sid = id
		}
		list = append(list, api.PublicServer{
			ID: sid, Name: s.Name, BaseURL: s.BaseURL, Datastore: s.Datastore, Namespace: s.Namespace,
			Fingerprint: s.CertFingerprint, Login: login, Encrypted: s.EncryptionKeyFile != "",
		})
	}
	sort.Slice(list, func(i, j int) bool {
		ni, nj := strings.ToLower(list[i].Name), strings.ToLower(list[j].Name)
		if ni != nj {
			return ni < nj
		}
		return list[i].ID < list[j].ID
	})
	return list
}

// LastRunOutcomeForAPI is the outcome of the last runs for the service's
// public status (GET /status/public, api.PublicStatusAPI), which anyone on
// the computer may read: only whether the most recent run of at least one
// Backup Set failed, and when the most recent backup ended. No names.
func (a *App) LastRunOutcomeForAPI() (failed bool, ended string) {
	jobs, err := a.GetScheduledJobs()
	if err != nil {
		writeDebugLog(fmt.Sprintf("[API] public status: reading the Backup Sets: %v", err))
	}
	hist, err := a.GetJobHistory()
	if err != nil {
		writeDebugLog(fmt.Sprintf("[API] public status: reading the history: %v", err))
		return false, ""
	}
	return lastRunOutcome(jobs, hist)
}

// lastRunOutcome reads hist (newest first, as stored): failed when the most
// recent backup of at least one of the Backup Sets in jobs failed, ended the
// time of the most recent backup of any kind. Restores, and runs still
// marked running, do not count; a one-off backup counts only for ended.
func lastRunOutcome(jobs []ScheduledJob, hist []JobHistory) (failed bool, ended string) {
	sets := map[string]bool{}
	for _, j := range jobs {
		sets[j.Name] = true
	}
	seen := map[string]bool{}
	for _, h := range hist {
		if h.Kind == "restore" || h.Status == "running" {
			continue
		}
		if ended == "" {
			ended = h.Timestamp
		}
		if h.Trigger == "oneoff" || !sets[h.Name] || seen[h.Name] {
			continue
		}
		seen[h.Name] = true
		if h.Status == "failed" {
			failed = true
		}
	}
	return failed, ended
}

// AppendJobHistoryFromAPI records an entry the app made (a restore, an
// undelete, a roll back).
func (a *App) AppendJobHistoryFromAPI(entry map[string]interface{}) error {
	var h JobHistory
	if err := remarshal(entry, &h); err != nil {
		return fmt.Errorf("history entry: %w", err)
	}
	return a.appendJobHistory(h)
}

// GetMessageLogForAPI is the service's Message Log, newest first.
func (a *App) GetMessageLogForAPI() ([]map[string]interface{}, error) {
	entries, err := a.GetMessageLog()
	if err != nil {
		return nil, err
	}
	var out []map[string]interface{}
	err = remarshal(entries, &out)
	return out, err
}

// AppendMessageFromAPI adds the app's entry to the Message Log, stamped with
// the time it arrives.
func (a *App) AppendMessageFromAPI(entry map[string]interface{}) error {
	var e MessageLogEntry
	if err := remarshal(entry, &e); err != nil {
		return fmt.Errorf("message entry: %w", err)
	}
	e.Timestamp = time.Now().Format(time.RFC3339)
	appendMessageEntry(e)
	return nil
}

// SetScheduledJobEnabledForAPI turns a Backup Set on or off.
func (a *App) SetScheduledJobEnabledForAPI(id string, enabled bool) error {
	return a.SetScheduledJobEnabled(id, enabled)
}

// RunScheduledJobForAPI runs a Backup Set now, here in the service, which
// records its history and last run and does its post-backup actions.
func (a *App) RunScheduledJobForAPI(id string) error {
	return a.RunScheduledJobNow(id)
}

// GetScheduledJobsFullForAPI lists the Backup Sets with every field.
func (a *App) GetScheduledJobsFullForAPI() ([]map[string]interface{}, error) {
	jobs, err := a.GetScheduledJobs()
	if err != nil {
		return nil, err
	}
	var out []map[string]interface{}
	err = remarshal(jobs, &out)
	return out, err
}

// ListLogsForAPI lists the service's logs.
func (a *App) ListLogsForAPI() (json.RawMessage, error) {
	files, err := a.ListLogFiles()
	if err != nil {
		return nil, err
	}
	return json.Marshal(files)
}

// ReadLogForAPI reads part of one of the service's logs.
func (a *App) ReadLogForAPI(query json.RawMessage) (json.RawMessage, error) {
	var q LogQuery
	if err := json.Unmarshal(query, &q); err != nil {
		return nil, fmt.Errorf("log query: %w", err)
	}
	v, err := a.ReadLog(q)
	if err != nil {
		return nil, err
	}
	return json.Marshal(v)
}

// FindRunLogForAPI finds the log of backupID's run at at (Unix ms).
func (a *App) FindRunLogForAPI(backupID string, at int64) (json.RawMessage, error) {
	f, err := a.FindRunLog(backupID, at)
	if err != nil {
		return nil, err
	}
	return json.Marshal(f)
}

// ExportSettingsForAPI is a settings export file built from the service's
// settings.
func (a *App) ExportSettingsForAPI(includeSecrets bool) ([]byte, error) {
	return a.buildSettingsExport(includeSecrets)
}

// ImportSettingsFromAPI merges an export file into the service's settings.
func (a *App) ImportSettingsFromAPI(data []byte) (string, error) {
	return a.importSettingsData(data)
}

// SaveKeyForAPI stores a key file the app made or imported in the service's
// keys folder, owner-only, never over an existing file, and returns its path.
func (a *App) SaveKeyForAPI(name string, data []byte) (string, error) {
	dir, err := getConfigDir()
	if err != nil {
		return "", err
	}
	keys := filepath.Join(dir, "keys")
	if name == "" {
		name = fmt.Sprintf("encryption-key-%s.json", time.Now().Format("20060102-150405"))
	}
	if filepath.Base(name) != name || strings.HasPrefix(name, ".") {
		return "", fmt.Errorf("invalid key file name %q", name)
	}
	if len(data) == 0 {
		return "", fmt.Errorf("the key file is empty")
	}
	if err := os.MkdirAll(keys, 0o700); err != nil {
		return "", err
	}
	_ = restrictKeyAccess(keys, true)
	path := filepath.Join(keys, name)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600) // #nosec G304 -- a name checked above, in the keys folder
	if err != nil {
		if os.IsExist(err) {
			return "", fmt.Errorf("%s already exists: it is not overwritten, as that would make the snapshots already encrypted with it unreadable", path)
		}
		return "", err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	if err := restrictKeyAccess(path, false); err != nil {
		writeDebugLog(fmt.Sprintf("SaveKeyForAPI: could not restrict access to %s: %v", path, err))
	}
	return path, nil
}

// apiCalls are the app methods the service runs for the app (POST
// /call/{name}): the settings setters, Backup Set changes and the tests. The
// service runs its own copy of the method on its own settings, with the same
// merge rules (an empty secret keeps the stored one) and checks as without a
// service. Nothing else can be called.
var apiCalls = map[string]func(a *App, args []json.RawMessage) (interface{}, error){
	"SaveConfig": func(a *App, args []json.RawMessage) (interface{}, error) {
		var c Config
		if err := arg(args, 0, &c); err != nil {
			return nil, err
		}
		return nil, a.SaveConfig(&c)
	},
	"AddPBSServer": func(a *App, args []json.RawMessage) (interface{}, error) {
		var s PBSServer
		if err := arg(args, 0, &s); err != nil {
			return nil, err
		}
		return nil, a.AddPBSServer(&s)
	},
	"UpdatePBSServer": func(a *App, args []json.RawMessage) (interface{}, error) {
		var s PBSServer
		if err := arg(args, 0, &s); err != nil {
			return nil, err
		}
		return nil, a.UpdatePBSServer(&s)
	},
	"DeletePBSServer": func(a *App, args []json.RawMessage) (interface{}, error) {
		var id string
		if err := arg(args, 0, &id); err != nil {
			return nil, err
		}
		return nil, a.DeletePBSServer(id)
	},
	"SetDefaultPBSServer": func(a *App, args []json.RawMessage) (interface{}, error) {
		var id string
		if err := arg(args, 0, &id); err != nil {
			return nil, err
		}
		return nil, a.SetDefaultPBSServer(id)
	},
	"TestConnection": func(a *App, args []json.RawMessage) (interface{}, error) {
		var c Config
		if err := arg(args, 0, &c); err != nil {
			return nil, err
		}
		return nil, a.TestConnection(&c)
	},
	"TestPBSConnection": func(a *App, args []json.RawMessage) (interface{}, error) {
		var id string
		if err := arg(args, 0, &id); err != nil {
			return nil, err
		}
		return nil, a.TestPBSConnection(id)
	},
	"SetSMTPSettings": func(a *App, args []json.RawMessage) (interface{}, error) {
		var host, port, user, pass, from string
		var insecure bool
		if err := args6(args, &host, &port, &user, &pass, &insecure, &from); err != nil {
			return nil, err
		}
		return nil, a.SetSMTPSettings(host, port, user, pass, insecure, from)
	},
	"SendTestEmail": func(a *App, args []json.RawMessage) (interface{}, error) {
		var to string
		if err := arg(args, 0, &to); err != nil {
			return nil, err
		}
		return nil, a.SendTestEmail(to)
	},
	"SetParallelRestore": func(a *App, args []json.RawMessage) (interface{}, error) {
		var on bool
		if err := arg(args, 0, &on); err != nil {
			return nil, err
		}
		return nil, a.SetParallelRestore(on)
	},
	"SetMaxConcurrentRuns": func(a *App, args []json.RawMessage) (interface{}, error) {
		var n int
		if err := arg(args, 0, &n); err != nil {
			return nil, err
		}
		return nil, a.SetMaxConcurrentRuns(n)
	},
	"SetSpeedLimits": func(a *App, args []json.RawMessage) (interface{}, error) {
		var up, down, disk int
		if err := arg(args, 0, &up); err != nil {
			return nil, err
		}
		if err := arg(args, 1, &down); err != nil {
			return nil, err
		}
		if err := arg(args, 2, &disk); err != nil {
			return nil, err
		}
		return nil, a.SetSpeedLimits(up, down, disk)
	},
	"SetCompressionLevel": func(a *App, args []json.RawMessage) (interface{}, error) {
		var level string
		if err := arg(args, 0, &level); err != nil {
			return nil, err
		}
		return nil, a.SetCompressionLevel(level)
	},
	"SetParallelBackupRead": func(a *App, args []json.RawMessage) (interface{}, error) {
		var on bool
		var workers int
		if err := arg(args, 0, &on); err != nil {
			return nil, err
		}
		if err := arg(args, 1, &workers); err != nil {
			return nil, err
		}
		return nil, a.SetParallelBackupRead(on, workers)
	},
	"SaveScheduledJob": func(a *App, args []json.RawMessage) (interface{}, error) {
		var j ScheduledJob
		if err := arg(args, 0, &j); err != nil {
			return nil, err
		}
		return nil, a.SaveScheduledJob(j)
	},
	"UpdateScheduledJob": func(a *App, args []json.RawMessage) (interface{}, error) {
		var j ScheduledJob
		if err := arg(args, 0, &j); err != nil {
			return nil, err
		}
		return nil, a.UpdateScheduledJob(j)
	},
	"RunScheduledJobNowRead": func(a *App, args []json.RawMessage) (interface{}, error) {
		var id, read string
		if err := arg(args, 0, &id); err != nil {
			return nil, err
		}
		if err := arg(args, 1, &read); err != nil {
			return nil, err
		}
		return nil, a.RunScheduledJobNowRead(id, read)
	},
	// Keys in the service's own folder only (keyInService): the service
	// runs as SYSTEM or root, and must not read, create or describe a file
	// anywhere else for whoever holds the token.
	"InspectEncryptionKeyFile": func(a *App, args []json.RawMessage) (interface{}, error) {
		p, err := serviceKeyArg(args)
		if err != nil {
			return nil, err
		}
		return a.InspectEncryptionKeyFile(p), nil
	},
	"RestrictEncryptionKeyFileAccess": func(a *App, args []json.RawMessage) (interface{}, error) {
		p, err := serviceKeyArg(args)
		if err != nil {
			return nil, err
		}
		return a.RestrictEncryptionKeyFileAccess(p)
	},
	"GenerateEncryptionKeyFile": func(a *App, args []json.RawMessage) (interface{}, error) {
		p, err := serviceKeyArg(args)
		if err != nil {
			return nil, err
		}
		return a.GenerateEncryptionKeyFile(p)
	},
	"EncryptionPaperKey": func(a *App, args []json.RawMessage) (interface{}, error) {
		p, err := serviceKeyArg(args)
		if err != nil {
			return nil, err
		}
		return a.EncryptionPaperKey(p)
	},
	"EncryptionKeyFileData": func(a *App, args []json.RawMessage) (interface{}, error) {
		p, err := serviceKeyArg(args)
		if err != nil {
			return nil, err
		}
		return encryptionKeyFileData(p)
	},
	"StoreImportedKey": func(a *App, args []json.RawMessage) (interface{}, error) {
		var out string
		if err := arg(args, 0, &out); err != nil {
			return nil, err
		}
		return a.storeKeyFromApp(out)
	},
	// The disk picker: the app, unelevated, cannot open the disks.
	"ListPhysicalDisks": func(a *App, args []json.RawMessage) (interface{}, error) {
		return physicalDisksWithPartitions()
	},
	"DeleteScheduledJob": func(a *App, args []json.RawMessage) (interface{}, error) {
		var id string
		if err := arg(args, 0, &id); err != nil {
			return nil, err
		}
		return nil, a.DeleteScheduledJob(id)
	},
}

// serviceKeyArg is a key call's path argument, refused unless it is inside
// this service's settings folder.
func serviceKeyArg(args []json.RawMessage) (string, error) {
	var p string
	if err := arg(args, 0, &p); err != nil {
		return "", err
	}
	p = strings.TrimSpace(p)
	dir, err := getConfigDir()
	if err != nil {
		return "", err
	}
	if p == "" || !pathWithin(p, dir) {
		return "", fmt.Errorf("the service only handles keys in its own folder %s", dir)
	}
	return p, nil
}

// arg decodes argument i into v.
func arg(args []json.RawMessage, i int, v interface{}) error {
	if i >= len(args) {
		return fmt.Errorf("argument %d missing", i+1)
	}
	if err := json.Unmarshal(args[i], v); err != nil {
		return fmt.Errorf("argument %d: %w", i+1, err)
	}
	return nil
}

// args6 decodes the six arguments of SetSMTPSettings.
func args6(args []json.RawMessage, host, port, user, pass *string, insecure *bool, from *string) error {
	for i, v := range []interface{}{host, port, user, pass, insecure, from} {
		if err := arg(args, i, v); err != nil {
			return err
		}
	}
	return nil
}

// CallForAPI runs the app method name (one of apiCalls) with args, the JSON
// array of its arguments, and returns its result as JSON.
func (a *App) CallForAPI(name string, raw json.RawMessage) (json.RawMessage, error) {
	call, ok := apiCalls[name]
	if !ok {
		return nil, api.ErrUnknownCall
	}
	// The app's stand-in for a secret the service holds must never come
	// back as a value: it would replace the real one.
	if bytes.Contains(raw, []byte(`\u0000stored-in-service`)) {
		return nil, fmt.Errorf("%s: a stored secret's placeholder was sent as a value", name)
	}
	var args []json.RawMessage
	if err := json.Unmarshal(raw, &args); err != nil {
		return nil, fmt.Errorf("%s: arguments: %w", name, err)
	}
	writeDebugLog(fmt.Sprintf("[API] call %s", name))
	out, err := call(a, args)
	if err != nil {
		return nil, err
	}
	if out == nil {
		return nil, nil
	}
	return json.Marshal(out)
}
