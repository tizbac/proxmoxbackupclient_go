package directorybackup

import (
	"encoding/json"
	"fmt"
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
	for _, bad := range []string{"[abc", `C:\Data\[x`, "*.[z-a", "![abc", "[abc/"} {
		c := cfg
		c.ExcludeList = []string{bad}
		if p := c.problems(); len(p) != 1 || !strings.Contains(p[0], "not a valid pattern") {
			t.Errorf("%q: problems %q", bad, p)
		}
	}
	for _, good := range []string{`C:\Users\alice\Temp`, `*\cache`, "[a-z]*.bak", "file?.txt",
		"!*.pst", "!**.pst", "!**/", "*/", "**/cache", "logs/**/*.tmp", `!C:\Users\alice\Temp\keep`} {
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

// -compression takes the five levels (and empty, default); anything else
// stops the run, rather than being taken as fastest.
func TestCompressionOption(t *testing.T) {
	base := Config{BaseURL: "https://pbs:8007", AuthID: "a@pbs!t", Secret: "s", Datastore: "ds", BackupSourceDir: "/data"}
	for _, ok := range []string{"", "none", "fastest", "default", "better", "best"} {
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

// Encrypted chunks are compressed unless the config file says
// "no-compress-encrypted": a config file that says nothing writes what
// proxmox-backup-client writes.
func TestNoCompressEncryptedOptionIsOffUnlessAsked(t *testing.T) {
	base := `"baseurl": "https://pbs:8007", "authid": "a@pbs!t", "secret": "s", "datastore": "ds", "backupdir": "/data", "keyfile": "/etc/pbs/key.json"`
	for name, c := range map[string]struct {
		extra string
		want  bool
	}{
		"left out": {"", false},
		"false":    {`, "no-compress-encrypted": false`, false},
		"true":     {`, "no-compress-encrypted": true`, true},
	} {
		var cfg Config
		if err := json.Unmarshal([]byte("{"+base+c.extra+"}"), &cfg); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if cfg.NoCompressEncrypted != c.want {
			t.Errorf("%s: NoCompressEncrypted = %v, want %v", name, cfg.NoCompressEncrypted, c.want)
		}
		if p := cfg.problems(); len(p) != 0 {
			t.Errorf("%s: %q", name, p)
		}
	}
	// the app's own key means the opposite and is not the tools': a settings
	// file of the app given by mistake must not turn compression off
	var cfg Config
	if err := json.Unmarshal([]byte("{"+base+`, "compress_encrypted": false}`), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.NoCompressEncrypted {
		t.Error(`"compress_encrypted": false (the app's key) was read as the tools' "no-compress-encrypted"`)
	}
}

// -read-ahead takes 0 (off) to 32 files, as the app does, for folders only.
func TestReadAheadOption(t *testing.T) {
	base := Config{BaseURL: "https://pbs:8007", AuthID: "a@pbs!t", Secret: "s", Datastore: "ds", BackupSourceDir: "/data"}
	for _, ok := range []int{0, 1, 8, MaxReadAhead} {
		c := base
		c.ReadAhead = ok
		if p := c.problems(); len(p) != 0 {
			t.Errorf("%d: %q", ok, p)
		}
	}
	for _, bad := range []int{-1, MaxReadAhead + 1} {
		c := base
		c.ReadAhead = bad
		if p := c.problems(); len(p) != 1 || !strings.Contains(p[0], fmt.Sprintf("-read-ahead is %d", bad)) {
			t.Errorf("%d: %q", bad, p)
		}
	}
	c := base
	c.BackupSourceDir, c.BackupStreamName, c.ReadAhead = "", "db.sql", 4
	if p := c.problems(); len(p) != 1 || !strings.Contains(p[0], "-backupstream") {
		t.Errorf("read-ahead with a stream: %q", p)
	}
	var fromFile Config
	if err := json.Unmarshal([]byte(`{"read-ahead": 8}`), &fromFile); err != nil || fromFile.ReadAhead != 8 {
		t.Errorf("config file: %v, %d", err, fromFile.ReadAhead)
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
