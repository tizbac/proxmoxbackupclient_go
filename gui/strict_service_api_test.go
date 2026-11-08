package main

// Tests of the service's side of the strict model (service_api.go,
// service_delegate.go, token_elevated.go): what the service runs for an app,
// what it hands out, and the app's copy of its settings.

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/salvanza/salvanza_client/gui/api"
)

const (
	strictURL    = "https://pbs.test:8007"
	strictAuthID = "backup@pbs!gui"
	strictSecret = "tok-secret-1"
)

// strictHome gives the test a settings folder of its own: getConfigDir finds
// the home folder through HOME on Linux and USERPROFILE on Windows.
func strictHome(t *testing.T) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
}

// strictServiceApp is the service's App, on settings of its own.
func strictServiceApp(t *testing.T) *App {
	t.Helper()
	strictHome(t)
	return &App{isServiceProcess: true, config: &Config{PBSServers: map[string]*PBSServer{}}}
}

func strictTokenServer(id string) *PBSServer {
	return &PBSServer{ID: id, Name: "Server " + id, BaseURL: strictURL, AuthID: strictAuthID, Secret: strictSecret, Datastore: "store1"}
}

// resetServiceGlobals undoes what enterServiceMode set for the whole
// process, so later tests log and connect as before.
func resetServiceGlobals() {
	api.SetUnauthorizedHook(nil)
	api.SetTokenOverride("")
	api.SetFetchedTokenOnly(false)
	messageSink.Store((func(MessageLogEntry) error)(nil))
}

// strictPair is a service App behind a real api.Server, a client of it and,
// once connected (strict_service_e2e_test.go), an app that works through it.
//
// Every request runs under mu, and the test reads the service's state under
// mu too (service): the race detector sees no ordering through the loopback
// socket, only through the lock.
type strictPair struct {
	svc    *App
	app    *App
	client *api.Client
	// tokenPath is the service's token file, which this test process may
	// read (as an elevated app reads it).
	tokenPath string
	url       string // where the service listens

	mu   sync.Mutex
	seen []string // "METHOD /path" of each request the service answered
}

func newStrictService(t *testing.T) *strictPair {
	t.Helper()
	p := &strictPair{svc: strictServiceApp(t)}
	tokenPath := filepath.Join(t.TempDir(), "api-token")
	tok, err := api.EnsureToken(tokenPath)
	if err != nil {
		t.Fatal(err)
	}
	h := api.NewServer("127.0.0.1:0", p.svc, tok, "test").Handler()
	hs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.mu.Lock()
		defer p.mu.Unlock()
		p.seen = append(p.seen, r.Method+" "+r.URL.Path)
		h.ServeHTTP(w, r)
	}))
	t.Cleanup(hs.Close)
	p.client = api.NewClientAt(hs.URL, tokenPath)
	p.tokenPath = tokenPath
	p.url = hs.URL
	return p
}

// service runs f on the service's App, between requests.
func (p *strictPair) service(f func(s *App)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	f(p.svc)
}

// saw fails the test unless the service answered each of reqs.
func (p *strictPair) saw(t *testing.T, reqs ...string) {
	t.Helper()
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, want := range reqs {
		found := false
		for _, s := range p.seen {
			if s == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("the service was never asked %q (asked: %v)", want, p.seen)
		}
	}
}

// The app's stand-in for a secret it does not hold must never reach the
// service as a value: saved, it would replace the real secret.
func TestCallForAPIRefusesStoredSecretPlaceholder(t *testing.T) {
	svc := strictServiceApp(t)
	svc.config.PBSServers["pbs1"] = strictTokenServer("pbs1")
	svc.config.SMTPPassword = "smtp-pw"

	calls := []struct {
		name string
		args []interface{}
	}{
		{"UpdatePBSServer", []interface{}{&PBSServer{ID: "pbs1", Name: "x", BaseURL: strictURL, AuthID: strictAuthID, Secret: storedSecret, Datastore: "store1"}}},
		{"AddPBSServer", []interface{}{&PBSServer{ID: "pbs2", Name: "y", BaseURL: strictURL, Username: "u@pbs", Password: storedSecret, Datastore: "store1"}}},
		{"SaveConfig", []interface{}{&Config{BaseURL: strictURL, AuthID: strictAuthID, Secret: storedSecret, Datastore: "store1"}}},
		{"SetSMTPSettings", []interface{}{"smtp.test", "587", "mailer", storedSecret, false, "from@test"}},
	}
	for _, c := range calls {
		raw, err := json.Marshal(c.args)
		if err != nil {
			t.Fatal(err)
		}
		out, err := svc.CallForAPI(c.name, raw)
		if err == nil || !strings.Contains(err.Error(), "placeholder") {
			t.Errorf("%s with the placeholder = %s, %v; want it refused", c.name, out, err)
		}
	}
	if got := svc.config.PBSServers["pbs1"].Secret; got != strictSecret {
		t.Errorf("server secret is now %q", got)
	}
	if _, ok := svc.config.PBSServers["pbs2"]; ok {
		t.Error("a server was added with the placeholder as its password")
	}
	if svc.config.SMTPPassword != "smtp-pw" || svc.config.Secret != "" {
		t.Errorf("settings changed: smtp %q, secret %q", svc.config.SMTPPassword, svc.config.Secret)
	}
}

// Only the methods on the service's list run: not the API's own handlers,
// nor anything else the App has.
func TestCallForAPIRefusesUnknownNames(t *testing.T) {
	svc := strictServiceApp(t)
	for _, name := range []string{"", "ExportSettingsForAPI", "PBSAccessForAPI", "SaveKeyForAPI", "GetFullConfigForAPI", "LoadConfig", "saveconfig"} {
		if _, err := svc.CallForAPI(name, json.RawMessage(`[]`)); !errors.Is(err, api.ErrUnknownCall) {
			t.Errorf("CallForAPI(%q) = %v, want api.ErrUnknownCall", name, err)
		}
	}
}

// Over HTTP an unknown name is a 404 naming it, and a refused placeholder
// comes back as the service's error.
func TestCallRouteRefusals(t *testing.T) {
	p := newStrictService(t)
	err := p.client.Call("ExportSettingsForAPI", nil, true)
	if err == nil || !strings.Contains(err.Error(), "unknown call: ExportSettingsForAPI") {
		t.Errorf("unknown call over HTTP = %v", err)
	}
	err = p.client.Call("SetSMTPSettings", nil, "h", "25", "u", storedSecret, false, "f")
	if err == nil || !strings.Contains(err.Error(), "placeholder") {
		t.Errorf("placeholder over HTTP = %v", err)
	}
	p.service(func(s *App) {
		if s.config.SMTPHost != "" {
			t.Errorf("SMTP host set to %q by a refused call", s.config.SMTPHost)
		}
	})
}

// Arguments that are missing, of the wrong type, or not an array are refused
// before the method runs.
func TestCallForAPIArgumentErrors(t *testing.T) {
	svc := strictServiceApp(t)
	cases := []struct {
		name, raw, want string
	}{
		{"UpdatePBSServer", `[]`, "argument 1 missing"},
		{"SetParallelRestore", `null`, "argument 1 missing"},
		{"SetParallelBackupRead", `[true]`, "argument 2 missing"},
		{"SetMaxConcurrentRuns", `["four"]`, "argument 1"},
		{"DeletePBSServer", `[42]`, "argument 1"},
		{"SetSMTPSettings", `["h","25","u","p","no","f"]`, "argument 5"},
		{"SetSMTPSettings", `["h","25","u","p",false]`, "argument 6 missing"},
		{"SaveConfig", `{"baseurl":"https://pbs.test:8007"}`, "arguments"},
		{"SaveScheduledJob", `[{"id":5}]`, "argument 1"},
	}
	for _, c := range cases {
		out, err := svc.CallForAPI(c.name, json.RawMessage(c.raw))
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s(%s) = %s, %v; want an error with %q", c.name, c.raw, out, err, c.want)
		}
	}
	if svc.config.MaxConcurrentRuns != 0 || svc.config.ParallelRestore || svc.config.SMTPHost != "" {
		t.Errorf("a refused call changed the settings: %+v", svc.config)
	}
}

// The service's settings document carries no secret, only *_set markers,
// and the app's copy built from it holds storedSecret exactly where one is
// stored.
func TestConfigFromServiceDocPlacesStoredSecret(t *testing.T) {
	svc := strictServiceApp(t)
	svc.config.Secret = "legacy-secret"
	svc.config.SMTPPassword = "smtp-pw"
	svc.config.PBSServers = map[string]*PBSServer{
		"tok":  {ID: "tok", Name: "Token", BaseURL: strictURL, AuthID: strictAuthID, Secret: strictSecret, EncryptionKeyPassphrase: "key-pp", Datastore: "store1"},
		"user": {ID: "user", Name: "User", BaseURL: strictURL, Username: "u@pbs", Password: "user-pw", Datastore: "store1"},
		"bare": {ID: "bare", Name: "Bare", BaseURL: strictURL, AuthID: strictAuthID, Datastore: "store1"},
	}

	doc := svc.GetFullConfigForAPI()
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"legacy-secret", "smtp-pw", strictSecret, "key-pp", "user-pw"} {
		if strings.Contains(string(b), s) {
			t.Errorf("the settings document holds the secret %q: %s", s, b)
		}
	}

	cfg, err := configFromServiceDoc(doc)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Secret != storedSecret || cfg.SMTPPassword != storedSecret {
		t.Errorf("top level: secret %q, smtp %q; want the placeholder", cfg.Secret, cfg.SMTPPassword)
	}
	if cfg.EncryptionKeyPassphrase != "" || cfg.EncryptionKeyPassphraseSet {
		t.Errorf("top-level passphrase %q (set %v), none is stored", cfg.EncryptionKeyPassphrase, cfg.EncryptionKeyPassphraseSet)
	}
	want := map[string][3]string{ // secret, password, passphrase
		"tok":  {storedSecret, "", storedSecret},
		"user": {"", storedSecret, ""},
		"bare": {"", "", ""},
	}
	if len(cfg.PBSServers) != len(want) {
		t.Fatalf("servers: %v", cfg.PBSServers)
	}
	for id, w := range want {
		s := cfg.PBSServers[id]
		if s == nil {
			t.Fatalf("server %s missing", id)
		}
		if got := [3]string{s.Secret, s.Password, s.EncryptionKeyPassphrase}; got != w {
			t.Errorf("server %s: secret/password/passphrase = %q, want %q", id, got, w)
		}
		if s.SecretSet || s.PasswordSet || s.EncryptionKeyPassphraseSet {
			t.Errorf("server %s keeps its *_set markers: %+v", id, s)
		}
	}
}

// Only a true boolean marker counts; a null server is dropped.
func TestConfigFromServiceDocOddDocuments(t *testing.T) {
	cfg, err := configFromServiceDoc(map[string]interface{}{
		"secret_set":                    "true",
		"smtp_password_set":             false,
		"encryption_key_passphrase_set": true,
		"pbs_servers": map[string]interface{}{
			"gone": nil,
			"s":    map[string]interface{}{"id": "s", "secret_set": false, "password_set": false},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Secret != "" || cfg.SMTPPassword != "" {
		t.Errorf("secret %q, smtp %q from markers that are not true", cfg.Secret, cfg.SMTPPassword)
	}
	if cfg.EncryptionKeyPassphrase != storedSecret || cfg.EncryptionKeyPassphraseSet {
		t.Errorf("passphrase %q (set %v)", cfg.EncryptionKeyPassphrase, cfg.EncryptionKeyPassphraseSet)
	}
	if _, ok := cfg.PBSServers["gone"]; ok || len(cfg.PBSServers) != 1 {
		t.Errorf("servers: %v", cfg.PBSServers)
	}
	if s := cfg.PBSServers["s"]; s.Secret != "" || s.Password != "" {
		t.Errorf("server s: %+v", s)
	}

	empty, err := configFromServiceDoc(map[string]interface{}{})
	if err != nil || empty.PBSServers == nil || empty.Secret != "" {
		t.Errorf("empty document: %+v, %v", empty, err)
	}
}

// A token server's access is its token (PBS gives no ticket for one); a
// username server's is a fresh ticket, never its password; the key comes only
// when asked for.
func TestPBSAccessForAPI(t *testing.T) {
	svc := strictServiceApp(t)

	var pbsMu sync.Mutex
	var gotPassword string
	pbs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api2/json/access/ticket" {
			http.NotFound(w, r)
			return
		}
		_ = r.ParseForm()
		pbsMu.Lock()
		gotPassword = r.PostForm.Get("password")
		pbsMu.Unlock()
		_, _ = w.Write([]byte(`{"data":{"ticket":"PBS:u@pbs:tick","CSRFPreventionToken":"csrf-1","username":"u@pbs"}}`))
	}))
	defer pbs.Close()

	keyPath := filepath.Join(t.TempDir(), "encryption-key.json")
	keyData := []byte(`{"kdf":null,"data":"a2V5"}`)
	if err := os.WriteFile(keyPath, keyData, 0o600); err != nil {
		t.Fatal(err)
	}
	tok := strictTokenServer("tok")
	tok.EncryptionKeyFile = keyPath
	tok.EncryptionKeyPassphrase = "key-pp"
	lost := strictTokenServer("lost")
	lost.EncryptionKeyFile = filepath.Join(t.TempDir(), "missing.json")
	svc.config.PBSServers = map[string]*PBSServer{
		"tok":  tok,
		"lost": lost,
		"user": {ID: "user", Name: "User", BaseURL: pbs.URL, Username: "u@pbs", Password: "user-pw", Datastore: "store1"},
		"nopw": {ID: "nopw", Name: "No password", BaseURL: pbs.URL, Username: "u@pbs", Datastore: "store1"},
	}
	svc.config.DefaultPBSID = "tok"

	acc, err := svc.PBSAccessForAPI(api.PBSAccessRequest{ID: "tok"})
	if err != nil {
		t.Fatal(err)
	}
	if acc.AuthID != strictAuthID || acc.Secret != strictSecret || acc.Ticket != "" || acc.Username != "" {
		t.Errorf("token server access: %+v", acc)
	}
	if acc.KeyFileData != nil || acc.KeyPassphrase != "" || acc.EncryptionKeyFile != "" {
		t.Errorf("key handed out unasked: %+v", acc)
	}

	// "" is the default server.
	acc, err = svc.PBSAccessForAPI(api.PBSAccessRequest{WithKey: true})
	if err != nil {
		t.Fatal(err)
	}
	if string(acc.KeyFileData) != string(keyData) || acc.KeyPassphrase != "key-pp" || acc.EncryptionKeyFile != keyPath {
		t.Errorf("key access: file %q, data %q, passphrase %q", acc.EncryptionKeyFile, acc.KeyFileData, acc.KeyPassphrase)
	}

	acc, err = svc.PBSAccessForAPI(api.PBSAccessRequest{ID: "user"})
	if err != nil {
		t.Fatal(err)
	}
	if acc.Ticket != "PBS:u@pbs:tick" || acc.CSRFToken != "csrf-1" || acc.Username != "u@pbs" || acc.Secret != "" || acc.AuthID != "" {
		t.Errorf("username server access: %+v", acc)
	}
	if b, _ := json.Marshal(acc); strings.Contains(string(b), "user-pw") {
		t.Errorf("username server access carries the password: %s", b)
	}
	pbsMu.Lock()
	if gotPassword != "user-pw" {
		t.Errorf("the service logged in with password %q", gotPassword)
	}
	pbsMu.Unlock()

	if _, err := svc.PBSAccessForAPI(api.PBSAccessRequest{ID: "nopw"}); err == nil {
		t.Error("a username server with no password stored gave access")
	}
	if _, err := svc.PBSAccessForAPI(api.PBSAccessRequest{ID: "missing"}); err == nil {
		t.Error("an unknown server gave access")
	}
	if _, err := svc.PBSAccessForAPI(api.PBSAccessRequest{ID: "lost", WithKey: true}); err == nil {
		t.Error("a key file that is gone gave no error")
	}
	if _, err := svc.PBSAccessForAPI(api.PBSAccessRequest{ID: "lost"}); err != nil {
		t.Errorf("without the key, a server whose key is gone: %v", err)
	}

	empty := strictServiceApp(t)
	if _, err := empty.PBSAccessForAPI(api.PBSAccessRequest{}); err == nil {
		t.Error("no server set up, yet access was given")
	}
}

// The elevated copy writes only into a plain file named by an absolute path.
func TestCheckHandoffPath(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "handoff")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := checkHandoffPath(file); err != nil {
		t.Errorf("the app's own handoff file refused: %v", err)
	}
	for name, p := range map[string]string{
		"relative":  "handoff",
		"directory": dir,
		"missing":   filepath.Join(dir, "missing"),
	} {
		if err := checkHandoffPath(p); err == nil {
			t.Errorf("%s path %q accepted", name, p)
		}
	}
}

func TestCheckHandoffPathRefusesSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("creating a symbolic link needs a privilege on Windows")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := checkHandoffPath(link); err == nil {
		t.Error("a link to a regular file accepted")
	}
	dangling := filepath.Join(dir, "dangling")
	if err := os.Symlink(filepath.Join(dir, "nowhere"), dangling); err != nil {
		t.Fatal(err)
	}
	if err := checkHandoffPath(dangling); err == nil {
		t.Error("a dangling link accepted")
	}
}

func TestReadHandoff(t *testing.T) {
	dir := t.TempDir()
	write := func(name, s string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(s), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	if tok, err := readHandoff(write("empty", "")); err == nil {
		t.Errorf("empty handoff file gave token %q", tok)
	}
	if tok, err := readHandoff(write("blank", " \r\n\t")); err == nil {
		t.Errorf("blank handoff file gave token %q", tok)
	}
	if tok, err := readHandoff(write("token", "abc123\r\n")); err != nil || tok != "abc123" {
		t.Errorf("readHandoff = %q, %v", tok, err)
	}
	if _, err := readHandoff(filepath.Join(dir, "missing")); err == nil {
		t.Error("a missing handoff file gave no error")
	}
}
