package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// deployTestDir points the settings folder at a fresh temporary one and
// returns it. getConfigDir reads %ProgramData% first on every OS.
func deployTestDir(t *testing.T) string {
	t.Helper()
	t.Setenv("ProgramData", t.TempDir())
	dir, err := getConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

// deployBundle is an Export Settings file from computer "SRC-PC": one PBS
// server with its token, and four Backup Sets.
func deployBundle(t *testing.T) SettingsBundle {
	t.Helper()
	return SettingsBundle{
		BundleVersion: currentSettingsBundleVersion,
		SourceHost:    "SRC-PC",
		Config: &Config{
			DefaultPBSID: "pbs1",
			PBSServers: map[string]*PBSServer{
				"pbs1": {ID: "pbs1", BaseURL: "https://pbs.example:8007", AuthID: "backup@pbs!deploy", Secret: "s3cret-token", Datastore: "store"},
			},
		},
		ScheduledJobs: []ScheduledJob{
			{ID: "j1", Name: "Documents", BackupID: "src-pc", BackupDirs: []string{`C:\Users`}, BackupType: "directory", PBSServerID: "pbs1", Enabled: true, LastRun: "2026-10-01T02:00:00Z", MetadataRunsSinceFullRead: 5},
			{ID: "j2", Name: "Shared share", BackupID: "fileshare", BackupDirs: []string{`D:\Share`}, BackupType: "directory", PBSServerID: "pbs1", Enabled: true},
			{ID: "j3", Name: "No ID", BackupDirs: []string{`C:\Data`}, BackupType: "directory", PBSServerID: "pbs1", Enabled: true},
			{ID: "j4", Name: "Projects", BackupID: "src-pc-projects", BackupDirs: []string{`C:\Projects`}, BackupType: "directory", PBSServerID: "pbs1", Enabled: true},
		},
	}
}

func writeBundle(t *testing.T, b SettingsBundle) string {
	t.Helper()
	raw, err := json.Marshal(b)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(t.TempDir(), "pbc-settings.json")
	if err := os.WriteFile(p, raw, 0600); err != nil {
		t.Fatal(err)
	}
	return p
}

func jobsByID(t *testing.T, a *App) map[string]ScheduledJob {
	t.Helper()
	jobs, err := a.GetScheduledJobs()
	if err != nil {
		t.Fatal(err)
	}
	m := map[string]ScheduledJob{}
	for _, j := range jobs {
		m[j.ID] = j
	}
	return m
}

func TestApplySettingsArg(t *testing.T) {
	for _, c := range []struct {
		args []string
		path string
		ok   bool
	}{
		{[]string{"-apply-settings", `C:\x.json`}, `C:\x.json`, true},
		{[]string{"--apply-settings", `\\srv\d\x.json`}, `\\srv\d\x.json`, true},
		{[]string{"-apply-settings=C:\\a b.json"}, `C:\a b.json`, true},
		{[]string{"-apply-settings"}, "", true},
		{[]string{"-service", "install"}, "", false},
		{[]string{"apply-settings", "x"}, "", false},
		{nil, "", false},
	} {
		p, ok := applySettingsArg(c.args)
		if p != c.path || ok != c.ok {
			t.Errorf("%q: got %q %v, want %q %v", c.args, p, ok, c.path, c.ok)
		}
	}
}

func TestDeployedBackupID(t *testing.T) {
	for _, c := range []struct{ id, src, this, want string }{
		{"src-pc", "SRC-PC", "NEW-PC", ""}, // the exporting computer's name: this computer's
		{"SRC-PC", "SRC-PC", "NEW-PC", ""}, //   in any case
		{"src-pc-projects", "SRC-PC", "NEW-PC", "new-pc-projects"},
		{"SRC-PC-Projects", "SRC-PC", "NEW-PC", "new-pc-Projects"},
		{"src-pcx", "SRC-PC", "NEW-PC", "src-pcx"}, // only whole words
		{"fileshare", "SRC-PC", "NEW-PC", "fileshare"},
		{"", "SRC-PC", "NEW-PC", ""},
		{"src-pc-data", "", "NEW-PC", "src-pc-data"},           // older export: no source name
		{"office-pc-data", "OFFICE_PC", "LAB 7", "lab-7-data"}, // as the editor slugs the name
		{"office_pc-data", "OFFICE_PC", "LAB 7", "office_pc-data"},
		{"src-pc-" + strings.Repeat("x", 60), "SRC-PC", "A-VERY-LONG-NEW-NAME", "a-very-long-new-name-" + strings.Repeat("x", 39)},
	} {
		if got := deployedBackupID(c.id, c.src, c.this); got != c.want {
			t.Errorf("deployedBackupID(%q, %q, %q) = %q, want %q", c.id, c.src, c.this, got, c.want)
		}
	}
}

func TestApplySettingsFile(t *testing.T) {
	deployTestDir(t)
	path := writeBundle(t, deployBundle(t))
	a := &App{config: LoadConfig()}
	servers, sets, err := a.applySettingsFile(path)
	if err != nil || servers != 1 || sets != 4 {
		t.Fatalf("applySettingsFile: %d %d %v", servers, sets, err)
	}
	if srv := LoadConfig().PBSServers["pbs1"]; srv == nil || srv.Secret != "s3cret-token" || srv.AuthID != "backup@pbs!deploy" {
		t.Fatalf("server not applied: %+v", srv)
	}
	if LoadConfig().DefaultPBSID != "pbs1" {
		t.Errorf("a computer with no default server should take the bundle's")
	}
	host, _ := os.Hostname()
	jobs := jobsByID(t, a)
	for id, want := range map[string]string{"j1": "", "j2": "fileshare", "j3": "", "j4": hostSlug(host) + "-projects"} {
		if got := jobs[id].BackupID; got != want {
			t.Errorf("set %s: backup ID %q, want %q", id, got, want)
		}
	}
	if j := jobs["j1"]; j.LastRun != "" || j.MetadataRunsSinceFullRead != 0 {
		t.Errorf("the exporting computer's run state came along: %q %d", j.LastRun, j.MetadataRunsSinceFullRead)
	}
	// The file the administrator keeps is read, not moved or changed.
	if _, err := os.Stat(path); err != nil {
		t.Errorf("source file gone: %v", err)
	}
	if entries, _ := a.GetMessageLog(); len(entries) == 0 || entries[0].MessageKey != MsgDeploySettingsApplied {
		t.Errorf("no Message Log entry for the applied settings: %+v", entries)
	} else if entries[0].MessageParams["servers"] != float64(1) || entries[0].MessageParams["sets"] != float64(4) {
		t.Errorf("counts in the Message Log entry: %+v", entries[0].MessageParams)
	}
}

// Deploying to a computer that has its own settings: its default server, the
// key passphrase it remembers, and the sets' run state on it are kept.
func TestApplySettingsFileKeepsLocalSettings(t *testing.T) {
	deployTestDir(t)
	a := &App{config: LoadConfig()}
	a.config.DefaultPBSID = "local"
	a.config.PBSServers = map[string]*PBSServer{
		"local": {ID: "local", BaseURL: "https://local:8007", AuthID: "a@pbs!t", Secret: "local-secret", Datastore: "d"},
		"pbs1":  {ID: "pbs1", BaseURL: "https://pbs.example:8007", AuthID: "backup@pbs!deploy", Secret: "old", Datastore: "store", EncryptionKeyPassphrase: "remembered"},
	}
	if err := a.config.Save(); err != nil {
		t.Fatal(err)
	}
	b := deployBundle(t)
	b.Config.PBSServers["pbs1"].Secret = "" // exported without passwords
	path := writeBundle(t, b)
	if _, _, err := a.applySettingsFile(path); err != nil {
		t.Fatal(err)
	}
	// Run once on this computer, then the same file again.
	jobs, _ := a.GetScheduledJobs()
	for i := range jobs {
		if jobs[i].ID == "j1" {
			jobs[i].LastRun, jobs[i].MetadataRunsSinceFullRead = "2026-10-20T02:00:00Z", 2
		}
	}
	raw, _ := json.Marshal(jobs)
	jp, _ := getScheduledJobsPath()
	if err := os.WriteFile(jp, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.applySettingsFile(path); err != nil {
		t.Fatal(err)
	}

	cfg := LoadConfig()
	if cfg.DefaultPBSID != "local" {
		t.Errorf("default server changed to %q", cfg.DefaultPBSID)
	}
	if s := cfg.PBSServers["pbs1"]; s.Secret != "old" || s.EncryptionKeyPassphrase != "remembered" {
		t.Errorf("local secrets lost: secret %q passphrase %q", s.Secret, s.EncryptionKeyPassphrase)
	}
	if cfg.PBSServers["local"] == nil {
		t.Errorf("a server not in the file was removed")
	}
	if j := jobsByID(t, a)["j1"]; j.LastRun != "2026-10-20T02:00:00Z" || j.MetadataRunsSinceFullRead != 2 {
		t.Errorf("this computer's run state lost on a second deployment: %q %d", j.LastRun, j.MetadataRunsSinceFullRead)
	}
}

// Import... in Preferences (moving to a new computer) keeps the backup ID: it
// is the old computer's backups that should carry on.
func TestImportKeepsSourceBackupID(t *testing.T) {
	deployTestDir(t)
	a := &App{config: LoadConfig()}
	if _, _, err := a.mergeSettingsBundle(deployBundle(t), false); err != nil {
		t.Fatal(err)
	}
	jobs := jobsByID(t, a)
	if jobs["j1"].BackupID != "src-pc" || jobs["j4"].BackupID != "src-pc-projects" {
		t.Errorf("interactive import changed backup IDs: %q %q", jobs["j1"].BackupID, jobs["j4"].BackupID)
	}
}

func TestApplySettingsFileRefuses(t *testing.T) {
	deployTestDir(t)
	bad := filepath.Join(t.TempDir(), "bad.json")
	nullServer := filepath.Join(t.TempDir(), "null.json")
	os.WriteFile(bad, []byte(`{not json`), 0600)
	os.WriteFile(nullServer, []byte(`{"bundleVersion":1,"config":{"pbs_servers":{"x":null}},"scheduledJobs":[{"id":"j","name":"n"}]}`), 0600)
	for name, path := range map[string]string{
		"missing file":  filepath.Join(t.TempDir(), "nope.json"),
		"not json":      bad,
		"null server":   nullServer,
		"no path given": "",
	} {
		a := &App{config: LoadConfig()}
		if _, _, err := a.applySettingsFile(path); err == nil {
			t.Errorf("%s: applied", name)
		}
		if len(LoadConfig().PBSServers) != 0 || len(jobsByID(t, a)) != 0 {
			t.Errorf("%s: settings changed", name)
		}
		if runApplySettings(path) != 1 {
			t.Errorf("%s: exit code not 1", name)
		}
	}
	a := &App{config: LoadConfig()}
	if _, _, err := a.applySettingsFile(""); err == nil || !strings.Contains(err.Error(), "no settings file given") {
		t.Errorf("no path: %v", err)
	}
	entries, _ := a.GetMessageLog()
	if len(entries) == 0 || entries[0].MessageKey != MsgDeploySettingsFailed {
		t.Errorf("Message Log should say the settings were not applied: %+v", entries)
	}
}

// The settings folder made into a link to somewhere else is refused.
func TestApplySettingsFileRefusesLinkedFolder(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symbolic links need a privilege on Windows")
	}
	pd := t.TempDir()
	t.Setenv("ProgramData", pd)
	elsewhere := t.TempDir()
	if err := os.Symlink(elsewhere, filepath.Join(pd, "ProxmoxBackupClient")); err != nil {
		t.Fatal(err)
	}
	a := &App{}
	if _, _, err := a.applySettingsFile(writeBundle(t, deployBundle(t))); err == nil || !strings.Contains(err.Error(), "not a plain folder") {
		t.Fatalf("want refusal, got %v", err)
	}
	if left, _ := os.ReadDir(elsewhere); len(left) != 0 {
		t.Errorf("wrote into the link's target: %v", left)
	}
}

// The template shipped in installer/ applies, and gives a daily set backing
// up under this computer's name.
func TestDeploySettingsTemplate(t *testing.T) {
	deployTestDir(t)
	a := &App{config: LoadConfig()}
	if _, _, err := a.applySettingsFile(filepath.Join("..", "installer", "deploy-settings.example.json")); err != nil {
		t.Fatal(err)
	}
	srv := LoadConfig().PBSServers["office"]
	if srv == nil || srv.BaseURL != "https://pbs.example.lan:8007" || srv.Datastore != "clients" {
		t.Fatalf("template server not applied: %+v", srv)
	}
	job, ok := jobsByID(t, a)["deploy-users"]
	if !ok {
		t.Fatal("template Backup Set not applied")
	}
	if job.BackupID != "" || job.PBSServerID != "office" || job.ScheduleMode != "scheduled" || len(job.Schedules) != 1 || job.NextRun == "" {
		t.Errorf("template set: id %q server %q mode %q schedules %+v next %q", job.BackupID, job.PBSServerID, job.ScheduleMode, job.Schedules, job.NextRun)
	}
}
