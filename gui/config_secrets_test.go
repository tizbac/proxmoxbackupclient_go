package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
)

// tempConfigPath points getConfigDir at a temp folder (useTempConfigDir) and
// returns the config.json path there.
func tempConfigPath(t *testing.T) string {
	t.Helper()
	useTempConfigDir(t)
	p, err := getConfigPath()
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func configWithEverySecret() *Config {
	return &Config{
		Secret:                  "legacy-token-secret",
		EncryptionKeyPassphrase: "legacy-key-passphrase",
		SMTPPassword:            "smtp-password-value",
		PBSServers: map[string]*PBSServer{
			"a": {ID: "a", Name: "A", BaseURL: "https://a:8007", AuthID: "u@pbs!t", Secret: "server-a-token-secret",
				EncryptionKeyPassphrase: "server-a-key-passphrase", Datastore: "d"},
			"b": {ID: "b", Name: "B", BaseURL: "https://b:8007", Username: "u@pbs", Password: "server-b-password", Datastore: "d"},
		},
	}
}

var everySecret = []string{"legacy-token-secret", "legacy-key-passphrase", "smtp-password-value",
	"server-a-token-secret", "server-a-key-passphrase", "server-b-password"}

func TestConfigSaveSealsSecretsAndLoadOpensThem(t *testing.T) {
	path := tempConfigPath(t)
	c := configWithEverySecret()
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	if c.PBSServers["a"].Secret != "server-a-token-secret" {
		t.Fatal("Save changed the in-memory config")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range everySecret {
		if inFile := strings.Contains(string(raw), s); inFile == canSealSecrets {
			t.Errorf("%q in config.json = %v, want %v", s, inFile, !canSealSecrets)
		}
	}
	if got := strings.Count(string(raw), sealedPrefix); canSealSecrets && got != len(everySecret) {
		t.Errorf("%d sealed values in config.json, want %d", got, len(everySecret))
	}

	back := LoadConfig()
	if !reflect.DeepEqual(back.secretValues(), c.secretValues()) {
		t.Errorf("loaded secrets %v, want %v", back.secretValues(), c.secretValues())
	}
}

// secretValues flattens secretFields for comparing two configs.
func (c *Config) secretValues() map[string]string {
	out := map[string]string{}
	for k, p := range c.secretFields() {
		out[k] = *p
	}
	return out
}

func TestLoadConfigSealsAnOldPlainConfig(t *testing.T) {
	path := tempConfigPath(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	plain := `{"pbs_servers":{"a":{"id":"a","name":"A","baseurl":"https://a:8007","authid":"u@pbs!t","secret":"old-plain-secret","datastore":"d","namespace":""}},"default_pbs_id":"a","usevss":true}`
	if err := os.WriteFile(path, []byte(plain), 0o600); err != nil {
		t.Fatal(err)
	}
	sealOnLoadOnce = sync.Once{}
	t.Cleanup(func() { sealOnLoadOnce = sync.Once{} })

	c := LoadConfig()
	if c.PBSServers["a"].Secret != "old-plain-secret" {
		t.Fatalf("secret = %q", c.PBSServers["a"].Secret)
	}
	raw, _ := os.ReadFile(path)
	if still := strings.Contains(string(raw), "old-plain-secret"); still == canSealSecrets {
		t.Errorf("plain secret still in config.json after load = %v, want %v", still, !canSealSecrets)
	}
}

func TestUnreadableSealedSecretIsKeptNotLost(t *testing.T) {
	path := tempConfigPath(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	const foreign = "dpapi:bm90LWEtcmVhbC1ibG9i" // "not-a-real-blob"
	cfg := `{"pbs_servers":{"a":{"id":"a","name":"A","baseurl":"https://a:8007","authid":"u@pbs!t","secret":"` + foreign + `","datastore":"d","namespace":""}},"usevss":true}`
	if err := os.WriteFile(path, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	c := LoadConfig()
	if c.PBSServers["a"].Secret != foreign {
		t.Fatalf("secret = %q, want it kept sealed", c.PBSServers["a"].Secret)
	}
	c.SMTPHost = "mail.example.com"
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), foreign) {
		t.Error("a save dropped the secret it could not unseal")
	}
}

// Every persisted secret must be in secretFields, or it would be written in
// plain text. A new field whose JSON name says secret, password or
// passphrase fails here until it is added there.
func TestSecretFieldsCoverEveryPersistedSecret(t *testing.T) {
	c := &Config{PBSServers: map[string]*PBSServer{"x": {}}}
	listed := map[uintptr]bool{}
	for _, p := range c.secretFields() {
		listed[reflect.ValueOf(p).Pointer()] = true
	}
	check := func(v reflect.Value, where string) {
		for i := 0; i < v.NumField(); i++ {
			f := v.Type().Field(i)
			tag := strings.Split(f.Tag.Get("json"), ",")[0]
			if f.Type.Kind() != reflect.String || tag == "-" || tag == "" {
				continue
			}
			if !strings.Contains(tag, "secret") && !strings.Contains(tag, "password") && !strings.Contains(tag, "passphrase") {
				continue
			}
			if !listed[v.Field(i).Addr().Pointer()] {
				t.Errorf("%s.%s (json %q) is not sealed: add it to secretFields", where, f.Name, tag)
			}
		}
	}
	check(reflect.ValueOf(c).Elem(), "Config")
	check(reflect.ValueOf(c.PBSServers["x"]).Elem(), "PBSServer")
}
