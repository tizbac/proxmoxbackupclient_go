//go:build !service

package main

// The app working through the service, end to end: an app that entered
// service mode (service_connect.go) and a service App behind a real
// api.Server, each on its own App, over HTTP.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tizbac/proxmoxbackupclient_go/gui/api"
)

// newStrictPair is newStrictService with an app connected to it, as after
// the elevated fetch (connected: it holds the token).
func newStrictPair(t *testing.T) *strictPair {
	t.Helper()
	p := newStrictService(t)
	p.app = &App{apiClient: p.client}
	t.Cleanup(resetServiceGlobals)
	p.app.enterServiceMode(true)
	return p
}

func strictJob(id, dir string) ScheduledJob {
	return ScheduledJob{
		ID: id, Name: "strict " + id, ScheduleTime: "02:00", BackupDirs: []string{dir},
		BackupID: "strict-" + id, BackupType: "directory", TriggerMode: "daily", Compression: "fastest",
	}
}

// Saving from the app with an empty secret keeps the one the service holds,
// and the window is told only that one is stored.
func TestStrictAppSettingsKeepServiceSecrets(t *testing.T) {
	p := newStrictPair(t)
	app := p.app

	if err := app.SaveConfig(&Config{BaseURL: strictURL, AuthID: strictAuthID, Secret: strictSecret, Datastore: "store1"}); err != nil {
		t.Fatal(err)
	}
	if err := app.SaveConfig(&Config{BaseURL: strictURL, AuthID: strictAuthID, Datastore: "store2"}); err != nil {
		t.Fatalf("SaveConfig with an empty secret: %v", err)
	}
	p.service(func(s *App) {
		if s.config.Secret != strictSecret || s.config.Datastore != "store2" {
			t.Errorf("service settings: secret %q, datastore %q", s.config.Secret, s.config.Datastore)
		}
	})
	if app.config.Secret != storedSecret || app.config.Datastore != "store2" {
		t.Errorf("app's copy: secret %q, datastore %q", app.config.Secret, app.config.Datastore)
	}

	if err := app.AddPBSServer(strictTokenServer("pbs1")); err != nil {
		t.Fatal(err)
	}
	renamed := &PBSServer{ID: "pbs1", Name: "Renamed", BaseURL: strictURL, AuthID: strictAuthID, Datastore: "store1"}
	if err := app.UpdatePBSServer(renamed); err != nil {
		t.Fatalf("UpdatePBSServer with an empty secret: %v", err)
	}
	// The stored secret is the old token's: a new token ID needs its own.
	other := &PBSServer{ID: "pbs1", Name: "Other", BaseURL: strictURL, AuthID: "backup@pbs!other", Datastore: "store1"}
	if err := app.UpdatePBSServer(other); err == nil {
		t.Error("a new token ID was saved with no secret")
	}
	p.service(func(s *App) {
		srv := s.config.PBSServers["pbs1"]
		if srv == nil || srv.Secret != strictSecret || srv.Name != "Renamed" || srv.AuthID != strictAuthID {
			t.Errorf("service's server: %+v", srv)
		}
	})
	if s := app.config.PBSServers["pbs1"]; s == nil || s.Secret != storedSecret || s.Name != "Renamed" {
		t.Errorf("app's copy of the server: %+v", s)
	}

	if err := app.SetSMTPSettings("smtp.test", "587", "mailer", "smtp-pw", false, "from@test"); err != nil {
		t.Fatal(err)
	}
	if err := app.SetSMTPSettings("smtp.test", "465", "mailer", "", false, "from@test"); err != nil {
		t.Fatal(err)
	}
	p.service(func(s *App) {
		if s.config.SMTPPassword != "smtp-pw" || s.config.SMTPPort != "465" {
			t.Errorf("service SMTP: password %q, port %q", s.config.SMTPPassword, s.config.SMTPPort)
		}
	})

	m := app.GetConfigWithHostname()
	if m["secret_set"] != true || m["secret"] != "" || m["smtp_password_set"] != true || m["datastore"] != "store2" {
		t.Errorf("GetConfigWithHostname: %v", m)
	}
	shown, err := json.Marshal([]interface{}{m, app.ListPBSServers()})
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{strictSecret, "smtp-pw", "stored-in-service"} {
		if strings.Contains(string(shown), s) {
			t.Errorf("the window is shown %q: %s", s, shown)
		}
	}
	if servers := app.ListPBSServers(); len(servers) != 1 || !servers[0].SecretSet {
		t.Errorf("ListPBSServers: %+v", servers)
	}
	p.saw(t, "POST /call/SaveConfig", "POST /call/AddPBSServer", "POST /call/UpdatePBSServer", "POST /call/SetSMTPSettings", "GET /config")
}

// PBS access in service mode comes from the service, for that one operation.
func TestStrictAppPBSAccessThroughService(t *testing.T) {
	p := newStrictPair(t)
	p.service(func(s *App) {
		s.config.PBSServers["pbs1"] = strictTokenServer("pbs1")
		s.config.DefaultPBSID = "pbs1"
	})
	cfg, err := p.app.resolvePBS("pbs1")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BaseURL != strictURL || cfg.AuthID != strictAuthID || cfg.Secret != strictSecret || cfg.Datastore != "store1" {
		t.Errorf("resolvePBS: %+v", cfg)
	}
	if _, err := p.app.resolvePBS("missing"); err == nil {
		t.Error("an unknown server resolved")
	}
	p.saw(t, "POST /pbs/access")
}

func TestStrictAppBackupSetsThroughService(t *testing.T) {
	p := newStrictPair(t)
	app := p.app
	job := strictJob("set1", t.TempDir())

	if err := app.SaveScheduledJob(job); err != nil {
		t.Fatal(err)
	}
	jobs, err := app.GetScheduledJobs()
	if err != nil || len(jobs) != 1 || jobs[0].Name != "strict set1" || !jobs[0].Enabled || jobs[0].NextRun == "" {
		t.Fatalf("after save: %+v, %v", jobs, err)
	}
	p.service(func(s *App) {
		if got, err := s.GetScheduledJobs(); err != nil || len(got) != 1 || got[0].ID != "set1" {
			t.Errorf("service's sets: %+v, %v", got, err)
		}
	})

	job.Name = "renamed"
	if err := app.UpdateScheduledJob(job); err != nil {
		t.Fatal(err)
	}
	if err := app.SetScheduledJobEnabled("set1", false); err != nil {
		t.Fatal(err)
	}
	jobs, err = app.GetScheduledJobs()
	if err != nil || len(jobs) != 1 || jobs[0].Name != "renamed" || jobs[0].Enabled {
		t.Fatalf("after update and disable: %+v, %v", jobs, err)
	}

	if err := app.DeleteScheduledJob("set1"); err != nil {
		t.Fatal(err)
	}
	jobs, err = app.GetScheduledJobs()
	if err != nil || len(jobs) != 0 {
		t.Fatalf("after delete: %+v, %v", jobs, err)
	}
	p.saw(t, "POST /call/SaveScheduledJob", "POST /call/UpdateScheduledJob", "POST /jobs/enabled", "POST /call/DeleteScheduledJob", "GET /jobs/full")
}

// The app's history entries and Message Log lines go to the service's, and
// it reads them from there.
func TestStrictAppHistoryAndMessageLogInService(t *testing.T) {
	p := newStrictPair(t)
	app := p.app

	entry := JobHistory{ID: "restore-1", Name: "Restore", Timestamp: time.Now().Format(time.RFC3339), Status: "success", Message: "restored 3 files"}
	if err := app.appendJobHistory(entry); err != nil {
		t.Fatal(err)
	}
	hist, err := app.GetJobHistory()
	if err != nil {
		t.Fatal(err)
	}
	if !strictHasHistory(hist, entry) {
		t.Errorf("app's history: %+v", hist)
	}
	p.service(func(s *App) {
		if h, err := s.GetJobHistory(); err != nil || !strictHasHistory(h, entry) {
			t.Errorf("service's history: %+v, %v", h, err)
		}
	})

	LogMessage("Restore", "info", "strict model message", "set1", "", nil)
	entries, err := app.GetMessageLog()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range entries {
		if e.Message == "strict model message" && e.Source == "Restore" && e.JobName == "set1" && e.Timestamp != "" {
			found = true
		}
	}
	if !found {
		t.Errorf("Message Log: %+v", entries)
	}
	// Both Apps share this process's settings folder, so reading the entry
	// back is not enough: it must have been posted.
	p.saw(t, "POST /history/append", "GET /history", "POST /messages", "GET /messages")
}

func strictHasHistory(h []JobHistory, want JobHistory) bool {
	for _, e := range h {
		if e.ID == want.ID && e.Message == want.Message && e.Status == want.Status {
			return true
		}
	}
	return false
}

// The service's logs are listed with their folder moved up by
// serviceLogFolder and read through the service.
func TestStrictAppLogViewerShowsServiceLogs(t *testing.T) {
	p := newStrictPair(t)
	if err := os.MkdirAll(logDir, 0o700); err != nil {
		t.Fatal(err)
	}
	name := "backup-20261102-101010-strictlogs.log"
	path := filepath.Join(logDir, name)
	if err := os.WriteFile(path, []byte("[BACKUP] [2026-11-02 10:10:10] strict log line\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })

	files, err := p.app.ListLogFiles()
	if err != nil {
		t.Fatal(err)
	}
	var local, svc *LogFileInfo
	for i := range files {
		if files[i].Name != name {
			continue
		}
		if files[i].Folder >= serviceLogFolder {
			svc = &files[i]
		} else {
			local = &files[i]
		}
	}
	if local == nil || svc == nil {
		t.Fatalf("listed: %+v", files)
	}
	if svc.Folder != local.Folder+serviceLogFolder || !svc.Service || local.Service {
		t.Errorf("local %+v, service %+v", *local, *svc)
	}

	v, err := p.app.ReadLog(LogQuery{Folder: svc.Folder, Name: name})
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Lines) != 1 || !strings.Contains(v.Lines[0], "strict log line") {
		t.Errorf("service log read: %+v", v)
	}
	p.saw(t, "GET /logs", "POST /logs/read")

	// Without the service there is no such folder.
	if _, err := (&App{}).ReadLog(LogQuery{Folder: svc.Folder, Name: name}); err == nil {
		t.Error("a standalone app read a service log folder")
	}
}

// delegated: only an app that entered service mode, never the service and
// never an App whose mode is merely the zero value (api.ModeService).
func TestDelegatedOnlyAfterEnterServiceMode(t *testing.T) {
	strictHome(t)
	client := api.NewClientAt("http://127.0.0.1:1", filepath.Join(t.TempDir(), "api-token"))

	if (&App{}).delegated() {
		t.Error("a zero App is delegated")
	}
	if a := (&App{apiClient: client}); a.mode != api.ModeService || a.delegated() {
		t.Errorf("an App in the zero mode (%v) with a client is delegated: %v", a.mode, a.delegated())
	}
	svc := &App{isServiceProcess: true, apiClient: client}
	svc.serviceSettings.Store(true)
	if svc.delegated() {
		t.Error("the service process is delegated")
	}
	noClient := &App{}
	noClient.serviceSettings.Store(true)
	if noClient.delegated() {
		t.Error("an App with no client is delegated")
	}

	t.Cleanup(resetServiceGlobals)
	a := &App{apiClient: client}
	a.enterServiceMode(false)
	if !a.delegated() {
		t.Error("not delegated after enterServiceMode")
	}
	if a.IsServiceConnected() {
		t.Error("connected without a token")
	}
	a.leaveServiceMode()
	if a.delegated() {
		t.Error("still delegated after leaveServiceMode")
	}
}

// A key in the service's folder is the service's to read, and only while the
// app works through the service.
func TestKeyInServiceOnlyForServiceFolder(t *testing.T) {
	strictHome(t)
	dir, err := serviceStateDir()
	if err != nil {
		t.Fatal(err)
	}
	inside := filepath.Join(dir, "keys", "encryption-key.json")
	client := api.NewClientAt("http://127.0.0.1:1", filepath.Join(t.TempDir(), "api-token"))

	a := &App{apiClient: client}
	if a.keyInService(inside) {
		t.Error("keyInService before service mode")
	}
	svc := &App{isServiceProcess: true, apiClient: client}
	svc.serviceSettings.Store(true)
	if svc.keyInService(inside) {
		t.Error("keyInService in the service itself")
	}

	t.Cleanup(resetServiceGlobals)
	a.enterServiceMode(false)
	cases := []struct {
		path string
		want bool
	}{
		{inside, true},
		{filepath.Join(dir, "k.json"), true},
		{"", false},
		{filepath.Join(t.TempDir(), "k.json"), false},
		{filepath.Join(dir, "..", "elsewhere", "k.json"), false},
		{filepath.Join(dir+"-other", "keys", "k.json"), false},
		{filepath.Join(dir, "keys", "..", "..", "out", "k.json"), false},
	}
	for _, c := range cases {
		if got := a.keyInService(c.path); got != c.want {
			t.Errorf("keyInService(%q) = %v, want %v", c.path, got, c.want)
		}
	}
}

// Inspecting a key in the service's folder asks the service; one elsewhere
// is read by the app itself.
func TestStrictAppInspectsServiceKeyThroughService(t *testing.T) {
	p := newStrictPair(t)
	dir, err := serviceStateDir()
	if err != nil {
		t.Fatal(err)
	}
	inside := filepath.Join(dir, "keys", "no-such-key.json")
	if info := p.app.InspectEncryptionKeyFile(inside); info.Usable {
		t.Errorf("a missing key is usable: %+v", info)
	}
	p.saw(t, "POST /call/InspectEncryptionKeyFile")

	p.mu.Lock()
	before := len(p.seen)
	p.mu.Unlock()
	p.app.InspectEncryptionKeyFile(filepath.Join(t.TempDir(), "elsewhere.json"))
	p.mu.Lock()
	after := len(p.seen)
	p.mu.Unlock()
	if after != before {
		t.Error("a key outside the service's folder was inspected through the service")
	}
}
