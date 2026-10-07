package main

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// newServiceTestApp pins a PRIVATE config dir and returns an App standing in
// for the service: the single writer of config.json, with an empty stored
// configuration. SaveFullConfigFromAPI is the endpoint the GUI pushes to.
func newServiceTestApp(t *testing.T) *App {
	t.Helper()
	SetConfigDir(t.TempDir())
	t.Cleanup(func() { SetConfigDir("") })
	return &App{config: &Config{PBSServers: make(map[string]*PBSServer)}}
}

// serverEntry builds a pbs_servers entry for a document, overriding the
// default (valid) values.
func serverEntry(fields map[string]interface{}) map[string]interface{} {
	entry := map[string]interface{}{
		"name":      "PBS Production",
		"baseurl":   "https://pbs.example.com:8007",
		"datastore": "backups",
	}
	for k, v := range fields {
		entry[k] = v
	}
	return entry
}

// docFor builds a whole configuration document. servers == nil means the key
// is absent (a client that does not manage servers); use an empty map to send
// an explicit "there are no servers".
func docFor(servers map[string]interface{}, extra ...map[string]interface{}) map[string]interface{} {
	doc := map[string]interface{}{
		"baseurl":             "",
		"authid":              "",
		"secret":              "",
		"datastore":           "",
		"default_pbs_id":      "",
		"encryption_key_file": "",
	}
	for _, e := range extra {
		for k, v := range e {
			doc[k] = v
		}
	}
	if servers != nil {
		doc["pbs_servers"] = servers
	}
	return doc
}

// configFileBytes returns config.json as stored, for atomicity assertions.
func configFileBytes(t *testing.T) string {
	t.Helper()
	path, err := getConfigPath()
	if err != nil {
		t.Fatalf("getConfigPath: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	return string(data)
}

// The historical bug: a document with servers but WITHOUT legacy top-level
// credentials was rejected with "Either username or authid is required!",
// making every multi-PBS save impossible.
func TestSaveFullConfigFromAPI_PureMultiPBSSaveSucceeds(t *testing.T) {
	a := newServiceTestApp(t)

	doc := docFor(map[string]interface{}{
		"pbs1": serverEntry(map[string]interface{}{
			"authid": "backup@pbs!nightly",
			"secret": "token-secret-123",
		}),
	})

	if err := a.SaveFullConfigFromAPI(doc); err != nil {
		t.Fatalf("SaveFullConfigFromAPI() error = %v", err)
	}

	srv := a.config.PBSServers["pbs1"]
	if srv == nil {
		t.Fatalf("server pbs1 was not stored")
	}
	if srv.Secret != "token-secret-123" {
		t.Errorf("Secret = %q, want the transmitted one", srv.Secret)
	}
	if srv.Username != "" || srv.Password != "" {
		t.Errorf("user/password = %q/%q, want empty on a token entry", srv.Username, srv.Password)
	}
	// The *set markers are a frontend hint derived on read, never stored.
	if srv.SecretSet || srv.PasswordSet || srv.IsOnline {
		t.Errorf("runtime markers persisted: SecretSet=%v PasswordSet=%v IsOnline=%v",
			srv.SecretSet, srv.PasswordSet, srv.IsOnline)
	}
	// A single server becomes the default automatically.
	if a.config.DefaultPBSID != "pbs1" {
		t.Errorf("DefaultPBSID = %q, want pbs1", a.config.DefaultPBSID)
	}

	// And it really is on disk (the service restarts from this file).
	loaded := LoadConfig()
	got := loaded.PBSServers["pbs1"]
	if got == nil {
		t.Fatalf("server pbs1 missing from config.json")
	}
	if got.Secret != "token-secret-123" {
		t.Errorf("stored Secret = %q, want token-secret-123", got.Secret)
	}
}

// The user/password method saves just as well as the token one.
func TestSaveFullConfigFromAPI_UserPasswordServerSaves(t *testing.T) {
	a := newServiceTestApp(t)

	doc := docFor(map[string]interface{}{
		"pbs1": serverEntry(map[string]interface{}{
			"username": "backup@pam",
			"password": "hunter2",
		}),
	})

	if err := a.SaveFullConfigFromAPI(doc); err != nil {
		t.Fatalf("SaveFullConfigFromAPI() error = %v", err)
	}

	srv := a.config.PBSServers["pbs1"]
	if srv == nil {
		t.Fatalf("server pbs1 was not stored")
	}
	if srv.Username != "backup@pam" || srv.Password != "hunter2" {
		t.Errorf("credentials = %q/%q, want backup@pam/hunter2", srv.Username, srv.Password)
	}
	if srv.AuthID != "" || srv.Secret != "" {
		t.Errorf("token credentials = %q/%q, want empty on a user/pass entry", srv.AuthID, srv.Secret)
	}
}

// The GUI never receives the stored secrets, so an empty credential means
// "keep what is stored" — for the very same token/user only.
func TestSaveFullConfigFromAPI_EmptyCredentialKeepsStoredOne(t *testing.T) {
	t.Run("token", func(t *testing.T) {
		a := newServiceTestApp(t)
		first := docFor(map[string]interface{}{
			"pbs1": serverEntry(map[string]interface{}{
				"authid": "backup@pbs!nightly",
				"secret": "stored-secret",
			}),
		})
		if err := a.SaveFullConfigFromAPI(first); err != nil {
			t.Fatalf("first save: %v", err)
		}

		// Second save: same authid, no secret (the GUI does not have it).
		second := docFor(map[string]interface{}{
			"pbs1": serverEntry(map[string]interface{}{
				"authid": "backup@pbs!nightly",
				"secret": "",
			}),
		})
		if err := a.SaveFullConfigFromAPI(second); err != nil {
			t.Fatalf("second save: %v", err)
		}
		if got := a.config.PBSServers["pbs1"].Secret; got != "stored-secret" {
			t.Errorf("Secret = %q, want it kept", got)
		}
	})

	t.Run("password", func(t *testing.T) {
		a := newServiceTestApp(t)
		first := docFor(map[string]interface{}{
			"pbs1": serverEntry(map[string]interface{}{
				"username": "backup@pam",
				"password": "stored-password",
			}),
		})
		if err := a.SaveFullConfigFromAPI(first); err != nil {
			t.Fatalf("first save: %v", err)
		}

		second := docFor(map[string]interface{}{
			"pbs1": serverEntry(map[string]interface{}{
				"username": "backup@pam",
				"password": "",
			}),
		})
		if err := a.SaveFullConfigFromAPI(second); err != nil {
			t.Fatalf("second save: %v", err)
		}
		if got := a.config.PBSServers["pbs1"].Password; got != "stored-password" {
			t.Errorf("Password = %q, want it kept", got)
		}
	})

	t.Run("another token cannot reuse the stored secret", func(t *testing.T) {
		a := newServiceTestApp(t)
		first := docFor(map[string]interface{}{
			"pbs1": serverEntry(map[string]interface{}{
				"authid": "backup@pbs!nightly",
				"secret": "stored-secret",
			}),
		})
		if err := a.SaveFullConfigFromAPI(first); err != nil {
			t.Fatalf("first save: %v", err)
		}

		second := docFor(map[string]interface{}{
			"pbs1": serverEntry(map[string]interface{}{
				"authid": "backup@pbs!OTHER",
				"secret": "",
			}),
		})
		err := a.SaveFullConfigFromAPI(second)
		if err == nil {
			t.Fatalf("save accepted a new authid without a secret")
		}
		if !strings.Contains(err.Error(), "secret") {
			t.Errorf("error = %q, want it to ask for the new secret", err)
		}
		if got := a.config.PBSServers["pbs1"].Secret; got != "stored-secret" {
			t.Errorf("stored Secret = %q, want it untouched after the rejection", got)
		}
	})

	t.Run("another user cannot reuse the stored password", func(t *testing.T) {
		a := newServiceTestApp(t)
		first := docFor(map[string]interface{}{
			"pbs1": serverEntry(map[string]interface{}{
				"username": "backup@pam",
				"password": "stored-password",
			}),
		})
		if err := a.SaveFullConfigFromAPI(first); err != nil {
			t.Fatalf("first save: %v", err)
		}

		second := docFor(map[string]interface{}{
			"pbs1": serverEntry(map[string]interface{}{
				"username": "root@pam",
				"password": "",
			}),
		})
		err := a.SaveFullConfigFromAPI(second)
		if err == nil {
			t.Fatalf("save accepted a new username without a password")
		}
		if !strings.Contains(err.Error(), "mot de passe") {
			t.Errorf("error = %q, want it to ask for the new password", err)
		}
	})
}

// Switching method must drop the other one's credentials: a user/password entry
// keeping a stale token secret would authenticate as the wrong principal.
func TestSaveFullConfigFromAPI_SwitchingMethodDropsOtherCredentials(t *testing.T) {
	t.Run("token to user/pass", func(t *testing.T) {
		a := newServiceTestApp(t)
		first := docFor(map[string]interface{}{
			"pbs1": serverEntry(map[string]interface{}{
				"authid": "backup@pbs!nightly",
				"secret": "stored-secret",
			}),
		})
		if err := a.SaveFullConfigFromAPI(first); err != nil {
			t.Fatalf("first save: %v", err)
		}

		second := docFor(map[string]interface{}{
			"pbs1": serverEntry(map[string]interface{}{
				"username": "backup@pam",
				"password": "new-password",
			}),
		})
		if err := a.SaveFullConfigFromAPI(second); err != nil {
			t.Fatalf("second save: %v", err)
		}

		srv := a.config.PBSServers["pbs1"]
		if srv.AuthID != "" || srv.Secret != "" {
			t.Errorf("token credentials = %q/%q, want them dropped", srv.AuthID, srv.Secret)
		}
		if srv.Username != "backup@pam" || srv.Password != "new-password" {
			t.Errorf("credentials = %q/%q, want backup@pam/new-password", srv.Username, srv.Password)
		}
	})

	t.Run("user/pass to token", func(t *testing.T) {
		a := newServiceTestApp(t)
		first := docFor(map[string]interface{}{
			"pbs1": serverEntry(map[string]interface{}{
				"username": "backup@pam",
				"password": "stored-password",
			}),
		})
		if err := a.SaveFullConfigFromAPI(first); err != nil {
			t.Fatalf("first save: %v", err)
		}

		second := docFor(map[string]interface{}{
			"pbs1": serverEntry(map[string]interface{}{
				"authid": "backup@pbs!nightly",
				"secret": "token-secret",
			}),
		})
		if err := a.SaveFullConfigFromAPI(second); err != nil {
			t.Fatalf("second save: %v", err)
		}

		srv := a.config.PBSServers["pbs1"]
		if srv.Username != "" || srv.Password != "" {
			t.Errorf("user/password = %q/%q, want them dropped", srv.Username, srv.Password)
		}
		if srv.AuthID != "backup@pbs!nightly" || srv.Secret != "token-secret" {
			t.Errorf("token = %q/%q, want backup@pbs!nightly/token-secret", srv.AuthID, srv.Secret)
		}
	})
}

// A rejected document must leave both the live config and config.json alone —
// the service would otherwise be left holding a config it never saved.
func TestSaveFullConfigFromAPI_RejectionIsAtomic(t *testing.T) {
	a := newServiceTestApp(t)
	good := docFor(map[string]interface{}{
		"pbs1": serverEntry(map[string]interface{}{
			"authid": "backup@pbs!nightly",
			"secret": "stored-secret",
		}),
	})
	if err := a.SaveFullConfigFromAPI(good); err != nil {
		t.Fatalf("setup save: %v", err)
	}

	beforeFile := configFileBytes(t)
	beforePtr := a.config

	bad := map[string]interface{}{
		"pbs_servers": map[string]interface{}{
			"pbs1": serverEntry(map[string]interface{}{
				"authid": "backup@pbs!nightly",
				"secret": "",
			}),
			// Invalid entry: no datastore, no credentials at all.
			"broken": map[string]interface{}{
				"name":    "Broken",
				"baseurl": "not-a-url",
			},
		},
	}
	err := a.SaveFullConfigFromAPI(bad)
	if err == nil {
		t.Fatalf("SaveFullConfigFromAPI() accepted an invalid document")
	}

	if a.config != beforePtr {
		t.Errorf("live config was replaced by a rejected document")
	}
	if got := configFileBytes(t); got != beforeFile {
		t.Errorf("config.json changed on a rejected save:\n got %s\nwant %s", got, beforeFile)
	}
	if got := a.config.PBSServers["pbs1"]; got == nil || got.Secret != "stored-secret" {
		t.Errorf("stored server was altered by a rejected save: %+v", got)
	}
}

// A document without the pbs_servers key comes from a client that does not
// manage servers: it must not wipe the stored ones.
func TestSaveFullConfigFromAPI_OmittedServersAreKept(t *testing.T) {
	a := newServiceTestApp(t)
	good := docFor(map[string]interface{}{
		"pbs1": serverEntry(map[string]interface{}{
			"authid": "backup@pbs!nightly",
			"secret": "stored-secret",
		}),
	})
	if err := a.SaveFullConfigFromAPI(good); err != nil {
		t.Fatalf("setup save: %v", err)
	}

	// The key is absent: e.g. the legacy settings form pushing SMTP settings.
	doc := docFor(nil, map[string]interface{}{"smtp_host": "mail.example.com"})
	if err := a.SaveFullConfigFromAPI(doc); err != nil {
		t.Fatalf("SaveFullConfigFromAPI() error = %v", err)
	}
	if len(a.config.PBSServers) != 1 {
		t.Errorf("servers = %d, want the stored one kept", len(a.config.PBSServers))
	}
	if a.config.SMTPHost != "mail.example.com" {
		t.Errorf("SMTPHost = %q, want mail.example.com", a.config.SMTPHost)
	}
}

// An explicit empty map means "the GUI deleted every server" — including the
// default selection.
func TestSaveFullConfigFromAPI_EmptyServersClearsThem(t *testing.T) {
	a := newServiceTestApp(t)
	good := docFor(map[string]interface{}{
		"pbs1": serverEntry(map[string]interface{}{
			"authid": "backup@pbs!nightly",
			"secret": "stored-secret",
		}),
	})
	if err := a.SaveFullConfigFromAPI(good); err != nil {
		t.Fatalf("setup save: %v", err)
	}
	if a.config.DefaultPBSID == "" {
		t.Fatalf("setup: no default server id")
	}

	if err := a.SaveFullConfigFromAPI(docFor(map[string]interface{}{})); err != nil {
		t.Fatalf("SaveFullConfigFromAPI() error = %v", err)
	}
	if len(a.config.PBSServers) != 0 {
		t.Errorf("servers = %d, want 0", len(a.config.PBSServers))
	}
	if a.config.DefaultPBSID != "" {
		t.Errorf("DefaultPBSID = %q, want it cleared", a.config.DefaultPBSID)
	}
	if loaded := LoadConfig(); len(loaded.PBSServers) != 0 {
		t.Errorf("config.json still has %d servers", len(loaded.PBSServers))
	}
}

// The legacy single-PBS block is validated only when it actually carries a
// URL: a URL that can never authenticate is an error, an empty block is not.
func TestSaveFullConfigFromAPI_LegacyBlockValidation(t *testing.T) {
	t.Run("url without credentials is rejected", func(t *testing.T) {
		a := newServiceTestApp(t)
		doc := docFor(nil, map[string]interface{}{
			"baseurl":   "https://legacy.example.com:8007",
			"datastore": "backups",
		})
		err := a.SaveFullConfigFromAPI(doc)
		if err == nil {
			t.Fatalf("save accepted a legacy URL without credentials")
		}
		if err.Error() != "Either username or authid is required!" {
			t.Errorf("error = %q, want the legacy credential error", err)
		}
	})

	t.Run("legacy token is kept and validated", func(t *testing.T) {
		a := newServiceTestApp(t)
		doc := docFor(nil, map[string]interface{}{
			"baseurl":   "https://legacy.example.com:8007",
			"authid":    "root@pbs!legacy",
			"secret":    "legacy-secret",
			"datastore": "backups",
		})
		if err := a.SaveFullConfigFromAPI(doc); err != nil {
			t.Fatalf("SaveFullConfigFromAPI() error = %v", err)
		}
		if a.config.AuthID != "root@pbs!legacy" || a.config.Secret != "legacy-secret" {
			t.Errorf("legacy credentials = %q/%q, want them stored", a.config.AuthID, a.config.Secret)
		}

		// A follow-up push without the secret (the GUI never receives it) keeps it.
		doc2 := docFor(nil, map[string]interface{}{
			"baseurl":   "https://legacy.example.com:8007",
			"authid":    "root@pbs!legacy",
			"secret":    "",
			"datastore": "backups",
		})
		if err := a.SaveFullConfigFromAPI(doc2); err != nil {
			t.Fatalf("second save: %v", err)
		}
		if a.config.Secret != "legacy-secret" {
			t.Errorf("Secret = %q, want it kept", a.config.Secret)
		}
	})

	t.Run("empty legacy block is accepted", func(t *testing.T) {
		a := newServiceTestApp(t)
		if err := a.SaveFullConfigFromAPI(docFor(map[string]interface{}{
			"pbs1": serverEntry(map[string]interface{}{
				"username": "backup@pam",
				"password": "hunter2",
			}),
		})); err != nil {
			t.Fatalf("SaveFullConfigFromAPI() error = %v", err)
		}
	})
}

// The whole GUI→service round-trip: what the GUI builds from its config is what
// the service stores, and what the service hands back lets the GUI keep pushing
// (including the encryption key PATH, which is not a secret and must survive —
// dropping it silently disables encryption on the next save).
func TestFullConfigDocumentRoundTrip(t *testing.T) {
	service := newServiceTestApp(t)

	// A real, passphrase-less key file: both the legacy and the per-server
	// paths are validated (an unusable key must not reach config.json).
	keyPath := writeKeyFile(t, t.TempDir(), "backup.key")

	guiSide := &Config{
		BaseURL:           "",
		EncryptionKeyFile: keyPath,
		Datastore:         "",
		BackupDir:         "/srv/backups",
		BackupID:          "workstation-01",
		PBSServers: map[string]*PBSServer{
			"pbs1": {
				ID:                "pbs1",
				Name:              "PBS Production",
				BaseURL:           "https://pbs.example.com:8007",
				AuthID:            "backup@pbs!nightly",
				Secret:            "typed-this-session",
				EncryptionKeyFile: keyPath,
				Datastore:         "backups",
			},
			"pbs2": {
				ID:        "pbs2",
				Name:      "PBS Remote",
				BaseURL:   "https://remote.example.com:8007",
				Username:  "backup@pam",
				Password:  "typed-this-session-too",
				Datastore: "backups",
			},
		},
		DefaultPBSID: "pbs1",
	}

	if err := service.SaveFullConfigFromAPI(guiSide.fullConfigDocument()); err != nil {
		t.Fatalf("SaveFullConfigFromAPI() error = %v", err)
	}

	if got := service.config.PBSServers["pbs1"].Secret; got != "typed-this-session" {
		t.Errorf("pbs1 Secret = %q, want the transmitted secret", got)
	}
	if got := service.config.PBSServers["pbs2"].Password; got != "typed-this-session-too" {
		t.Errorf("pbs2 Password = %q, want the transmitted password", got)
	}
	if got := service.config.PBSServers["pbs1"].EncryptionKeyFile; got != keyPath {
		t.Errorf("pbs1 EncryptionKeyFile = %q, want it stored", got)
	}

	// What the GUI gets back: sanitized (no credentials), markers set, and the
	// key path intact so the next push does not clear it.
	back := parseFullConfig(service.GetFullConfigForAPI())
	if back.EncryptionKeyFile != keyPath {
		t.Errorf("EncryptionKeyFile after round-trip = %q, want it preserved", back.EncryptionKeyFile)
	}
	if back.PBSServers["pbs1"] == nil || back.PBSServers["pbs1"].SecretSet != true {
		t.Errorf("pbs1 secret_set marker lost: %+v", back.PBSServers["pbs1"])
	}
	if back.PBSServers["pbs2"] == nil || back.PBSServers["pbs2"].PasswordSet != true {
		t.Errorf("pbs2 password_set marker lost: %+v", back.PBSServers["pbs2"])
	}
	if back.PBSServers["pbs1"].Secret != "" || back.PBSServers["pbs2"].Password != "" {
		t.Errorf("credentials leaked to the GUI: %+v", back.PBSServers["pbs1"])
	}

	// And pushing that back (without any credential) keeps everything stored.
	if err := service.SaveFullConfigFromAPI(back.fullConfigDocument()); err != nil {
		t.Fatalf("second push: %v", err)
	}
	if got := service.config.PBSServers["pbs1"].Secret; got != "typed-this-session" {
		t.Errorf("Secret after second push = %q, want it kept", got)
	}
	if got := service.config.PBSServers["pbs2"].Password; got != "typed-this-session-too" {
		t.Errorf("Password after second push = %q, want it kept", got)
	}
	if got := service.config.PBSServers["pbs1"].EncryptionKeyFile; got != keyPath {
		t.Errorf("EncryptionKeyFile after second push = %q, want it kept", got)
	}

	// config.json must never contain the markers as persisted state.
	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(configFileBytes(t)), &raw); err != nil {
		t.Fatalf("decode config.json: %v", err)
	}
	servers, _ := raw["pbs_servers"].(map[string]interface{})
	entry, _ := servers["pbs1"].(map[string]interface{})
	if entry == nil {
		t.Fatalf("pbs1 missing from config.json")
	}
	if _, ok := entry["secret_set"]; ok {
		t.Errorf("secret_set persisted in config.json: %v", entry["secret_set"])
	}
}
