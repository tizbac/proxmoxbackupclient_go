//go:build !service

package main

// The computer's PBS servers for a user on their own settings
// (ComputerServers, GET /servers/public): where each server is, never how to
// log in to it.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/salvanza/salvanza_client/gui/api"
)

// The list holds no token ID, username, secret, password or key, even
// though the computer's settings hold them all, and is sorted by name.
func TestPublicServersLeaveOutLogins(t *testing.T) {
	cfg := &Config{PBSServers: map[string]*PBSServer{
		"office": {ID: "office", Name: "Office", BaseURL: "https://pbs.example.com:8007", CertFingerprint: "aa:bb",
			AuthID: "backup@pbs!office-token", Secret: "secret-token-value", Datastore: "main", Namespace: "office",
			EncryptionKeyFile: `C:\ProgramData\keys\office-key.json`, EncryptionKeyPassphrase: "key-passphrase"},
		"archive": {ID: "archive", Name: "archive", BaseURL: "https://offsite.example.com:8007",
			Username: "someone@pbs", Password: "login-password", Datastore: "dr"},
		"empty": nil,
	}}
	list := publicServers(cfg)
	if len(list) != 2 {
		t.Fatalf("%d servers, want 2: %+v", len(list), list)
	}
	if list[0].ID != "archive" || list[1].ID != "office" {
		t.Fatalf("order %s, %s; want archive, office", list[0].ID, list[1].ID)
	}
	if list[0].Login != "password" || list[1].Login != "token" || list[0].Encrypted || !list[1].Encrypted {
		t.Fatalf("logins or encryption wrong: %+v", list)
	}
	if list[1].Fingerprint != "aa:bb" || list[1].Namespace != "office" || list[1].Datastore != "main" {
		t.Fatalf("office server %+v", list[1])
	}
	b, err := json.Marshal(list)
	if err != nil {
		t.Fatal(err)
	}
	for _, leak := range []string{"backup@pbs", "office-token", "secret-token-value", "someone@pbs", "login-password", "office-key.json", "key-passphrase", "ProgramData"} {
		if strings.Contains(string(b), leak) {
			t.Errorf("the list holds %q: %s", leak, b)
		}
	}
	if got := publicServers(nil); got == nil || len(got) != 0 {
		t.Errorf("publicServers(nil) = %v, want an empty list", got)
	}
}

// ComputerServers answers only on the user's own settings, and says so when
// the service gives no list.
func TestComputerServersOnlyOnOwnSettings(t *testing.T) {
	hs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != api.PublicServersPath || r.Header.Get("X-Salvanza-Token") != "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode([]api.PublicServer{{ID: "office", Name: "Office", BaseURL: "https://pbs.example.com:8007", Datastore: "main", Login: "token"}})
	}))
	t.Cleanup(hs.Close)
	a := &App{apiClient: api.NewClientAt(hs.URL, filepath.Join(t.TempDir(), "api-token")), mode: api.ModeService}
	if _, err := a.ComputerServers(); err == nil {
		t.Fatal("ComputerServers answered on the computer's settings")
	}
	a.mode = api.ModeStandalone
	a.ownSettings.Store(true)
	list, err := a.ComputerServers()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != "office" {
		t.Fatalf("list %+v", list)
	}

	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	url := down.URL
	down.Close()
	a.apiClient = api.NewClientAt(url, filepath.Join(t.TempDir(), "api-token"))
	if _, err := a.ComputerServers(); err == nil || !strings.Contains(err.Error(), "did not give") {
		t.Fatalf("ComputerServers with no service = %v, want an error", err)
	}
}
