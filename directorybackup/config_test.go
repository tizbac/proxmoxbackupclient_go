package main

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestConfigDirs(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
		want []string
	}{
		{"none", Config{}, nil},
		{"single legacy", Config{BackupSourceDir: "/a"}, []string{"/a"}},
		{"list only", Config{BackupSourceDirs: []string{"/a", "/b"}}, []string{"/a", "/b"}},
		{"both, dedup, order", Config{BackupSourceDir: "/a", BackupSourceDirs: []string{"/b", "/a", ""}}, []string{"/a", "/b"}},
	}
	for _, c := range cases {
		if got := c.cfg.Dirs(); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: Dirs() = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestDirListFlag(t *testing.T) {
	var f dirListFlag
	for _, v := range []string{"/a", "/b"} {
		if err := f.Set(v); err != nil {
			t.Fatal(err)
		}
	}
	if !reflect.DeepEqual([]string(f), []string{"/a", "/b"}) || f.String() != "/a,/b" {
		t.Errorf("dirListFlag = %v", f)
	}
}

// TestConfigDirsWithKeyFile: one key file serves every directory of a
// several-directory run. The key is a run-wide setting, not a per-directory
// one, so the JSON config must keep both lists and stay valid.
func TestConfigDirsWithKeyFile(t *testing.T) {
	raw := `{
		"baseurl": "https://pbs:8007",
		"authid": "user@pbs!tok",
		"secret": "s",
		"datastore": "ds",
		"backupdirs": ["/data/a", "/data/b", "/data/c"],
		"keyfile": "/etc/pbs/key.json",
		"keyfilepassphrase": "hunter2"
	}`
	var cfg Config
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatal(err)
	}
	if want := []string{"/data/a", "/data/b", "/data/c"}; !reflect.DeepEqual(cfg.Dirs(), want) {
		t.Errorf("Dirs() = %v, want %v", cfg.Dirs(), want)
	}
	if cfg.KeyFile != "/etc/pbs/key.json" || cfg.KeyFilePassphrase != "hunter2" {
		t.Errorf("key settings lost: %q %q", cfg.KeyFile, cfg.KeyFilePassphrase)
	}
	if !cfg.valid() {
		t.Error("config with several backupdirs and a keyfile should be valid")
	}
}

// The config file's "exclude" list is read, and a malformed pattern, which
// would never match, or -exclude with only a stream to back up, stop the run.
func TestExcludeConfig(t *testing.T) {
	var cfg Config
	if err := json.Unmarshal([]byte(`{"baseurl": "https://pbs:8007", "authid": "a@pbs!t", "secret": "s",
		"datastore": "ds", "backupdir": "/data", "exclude": ["*.tmp", "node_modules", "logs/*.log", ""]}`), &cfg); err != nil {
		t.Fatal(err)
	}
	if want := []string{"*.tmp", "node_modules", "logs/*.log", ""}; !reflect.DeepEqual(cfg.ExcludeList, want) {
		t.Errorf("ExcludeList = %q", cfg.ExcludeList)
	}
	if p := cfg.problems(); len(p) != 0 {
		t.Errorf("good patterns: %q", p)
	}
	for _, bad := range []string{"[abc", `C:\Data\[x`, "*.[z-a"} {
		c := cfg
		c.ExcludeList = []string{bad}
		if p := c.problems(); len(p) != 1 || !strings.Contains(p[0], "not a valid pattern") {
			t.Errorf("%q: problems %q", bad, p)
		}
	}
	for _, good := range []string{`C:\Users\alice\Temp`, `*\cache`, "[a-z]*.bak", "file?.txt"} {
		if p := excludePatternProblem(good); p != "" {
			t.Errorf("%q: %s", good, p)
		}
	}
	// "C:\Temp\" as the last argument reaches the tool as C:\Temp" on Windows.
	if p := excludePatternProblemOn(`C:\Temp"`, "windows"); !strings.Contains(p, `[C:\Temp"]`) {
		t.Errorf("quote on Windows: %q", p)
	}
	if p := excludePatternProblemOn(`it's "quoted"`, "linux"); p != "" {
		t.Errorf("quote on Linux, a legal name: %q", p)
	}
	c := cfg
	c.BackupSourceDir, c.BackupStreamName = "", "db.sql"
	if p := c.problems(); len(p) != 1 || !strings.Contains(p[0], "-backupstream") {
		t.Errorf("exclude with a stream: %q", p)
	}
}

// -compression takes the four levels (and empty, default); anything else
// stops the run, rather than being taken as fastest.
func TestCompressionOption(t *testing.T) {
	base := Config{BaseURL: "https://pbs:8007", AuthID: "a@pbs!t", Secret: "s", Datastore: "ds", BackupSourceDir: "/data"}
	for _, ok := range []string{"", "fastest", "default", "better", "best"} {
		c := base
		c.Compression = ok
		if p := c.problems(); len(p) != 0 {
			t.Errorf("%q: %q", ok, p)
		}
	}
	c := base
	c.Compression = "9"
	if p := c.problems(); len(p) != 1 || !strings.Contains(p[0], `-compression is "9"`) {
		t.Errorf("a bad level: %q", p)
	}
	var fromFile Config
	if err := json.Unmarshal([]byte(`{"compression": "better"}`), &fromFile); err != nil || fromFile.Compression != "better" {
		t.Errorf("config file: %v, %q", err, fromFile.Compression)
	}
}

// TestSMTPRelayWithoutLogin: the smtp block of issue #74, a relay on port 25
// with no username or password, is valid; incomplete blocks say what is
// missing.
func TestSMTPRelayWithoutLogin(t *testing.T) {
	raw := `{
		"baseurl": "https://pbs:8007", "authid": "user@pbs!tok", "secret": "s",
		"datastore": "ds", "backupdir": "/data",
		"smtp": {"host": "mailhost.example.com", "port": "25", "username": "", "password": "",
			"insecure": false, "mails": [{"from": "noreply@example.com", "to": "user@example.com"}]}
	}`
	var cfg Config
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		t.Fatal(err)
	}
	if !cfg.valid() {
		t.Errorf("relay without login: %s", cfg.SMTP.problem())
	}
	for name, s := range map[string]SMTPConfig{
		"no host":           {Port: "25", Mails: []MailSendConfig{{From: "a@x", To: "b@x"}}},
		"password, no user": {Host: "h", Port: "25", Password: "p", Mails: []MailSendConfig{{From: "a@x", To: "b@x"}}},
		"no mails":          {Host: "h", Port: "25"},
		"no to":             {Host: "h", Port: "25", Mails: []MailSendConfig{{From: "a@x"}}},
	} {
		if s.problem() == "" {
			t.Errorf("%s: want a problem", name)
		}
	}
}
