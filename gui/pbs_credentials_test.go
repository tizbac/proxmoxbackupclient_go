//go:build !service

package main

import (
	"strings"
	"testing"
)

func credTestConfig(t *testing.T) *Config {
	t.Helper()
	t.Setenv("ProgramData", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	c := &Config{PBSServers: map[string]*PBSServer{}}
	for _, s := range []*PBSServer{
		{ID: "tok", Name: "Token", BaseURL: "https://pbs.example:8007", AuthID: "backup@pbs!one", Secret: "secret-one", Datastore: "store"},
		{ID: "usr", Name: "User", BaseURL: "https://pbs.example:8007", Username: "alice@pbs", Password: "alice-pw", Datastore: "store"},
	} {
		if err := c.AddPBSServer(s); err != nil {
			t.Fatalf("add %s: %v", s.ID, err)
		}
	}
	return c
}

// A blank secret keeps the stored one for the same token ID only; a new
// token ID with a blank secret is refused rather than given the old token's
// secret.
func TestUpdatePBSServerSecretOnlyForSameToken(t *testing.T) {
	c := credTestConfig(t)
	a := &App{config: c}

	same := &PBSServer{ID: "tok", Name: "Token", BaseURL: "https://pbs.example:8007", AuthID: "backup@pbs!one", Datastore: "store"}
	if err := a.UpdatePBSServer(same); err != nil {
		t.Fatalf("same token, blank secret: %v", err)
	}
	if got, _ := c.GetPBSServer("tok"); got.Secret != "secret-one" {
		t.Errorf("same token: secret %q, want the stored one", got.Secret)
	}

	other := &PBSServer{ID: "tok", Name: "Token", BaseURL: "https://pbs.example:8007", AuthID: "backup@pbs!two", Datastore: "store"}
	err := a.UpdatePBSServer(other)
	if err == nil || !strings.Contains(err.Error(), "secret required") {
		t.Fatalf("new token, blank secret: err %v, want secret required", err)
	}
	if got, _ := c.GetPBSServer("tok"); got.AuthID != "backup@pbs!one" || got.Secret != "secret-one" {
		t.Errorf("refused update changed the server: %+v", got)
	}
}

// A server keeps one login only: switching between token and username and
// password drops the other's credentials.
func TestUpdatePBSServerSwitchClearsTheOtherLogin(t *testing.T) {
	c := credTestConfig(t)

	if err := c.UpdatePBSServer(&PBSServer{ID: "tok", Name: "Token", BaseURL: "https://pbs.example:8007", Secret: "secret-one", Username: "carol@pbs", Password: "carol-pw", Datastore: "store"}); err != nil {
		t.Fatalf("token to user: %v", err)
	}
	if got, _ := c.GetPBSServer("tok"); got.Secret != "" || got.AuthID != "" || got.Username != "carol@pbs" {
		t.Errorf("token to user kept the token: %+v", got)
	}

	if err := c.UpdatePBSServer(&PBSServer{ID: "usr", Name: "User", BaseURL: "https://pbs.example:8007", AuthID: "backup@pbs!two", Secret: "secret-two", Username: "alice@pbs", Password: "alice-pw", Datastore: "store"}); err != nil {
		t.Fatalf("user to token: %v", err)
	}
	if got, _ := c.GetPBSServer("usr"); got.Username != "" || got.Password != "" || got.Secret != "secret-two" {
		t.Errorf("user to token kept the password login: %+v", got)
	}
}

// A blank password keeps the stored one for the same user only.
func TestUpdatePBSServerPasswordOnlyForSameUser(t *testing.T) {
	c := credTestConfig(t)

	if err := c.UpdatePBSServer(&PBSServer{ID: "usr", Name: "User", BaseURL: "https://pbs.example:8007", Username: "alice@pbs", Datastore: "store"}); err != nil {
		t.Fatalf("same user, blank password: %v", err)
	}
	if got, _ := c.GetPBSServer("usr"); got.Password != "alice-pw" {
		t.Errorf("same user: password %q, want the stored one", got.Password)
	}

	err := c.UpdatePBSServer(&PBSServer{ID: "usr", Name: "User", BaseURL: "https://pbs.example:8007", Username: "bob@pbs", Datastore: "store"})
	if err == nil || !strings.Contains(err.Error(), "password required") {
		t.Fatalf("new user, blank password: err %v, want password required", err)
	}
	if got, _ := c.GetPBSServer("usr"); got.Username != "alice@pbs" || got.Password != "alice-pw" {
		t.Errorf("refused update changed the server: %+v", got)
	}
}
