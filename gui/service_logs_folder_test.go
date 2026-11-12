package main

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/salvanza/salvanza_client/gui/api"
)

// withServiceFolder points the service's log folder at dir for a test.
func withServiceFolder(t *testing.T, dir string) {
	t.Helper()
	old := serviceLogsFolder
	serviceLogsFolder = func() string { return dir }
	t.Cleanup(func() { serviceLogsFolder = old })
}

func TestServiceLogsFolderIsListedWhenReadable(t *testing.T) {
	own := withLogFolder(t)
	svc := t.TempDir()
	withServiceFolder(t, svc)
	writeLogLines(t, filepath.Join(own, "service-gui.log"), []string{"[GUI] [2026-11-12 10:00:00] hello"}, false)
	writeLogLines(t, filepath.Join(svc, "backup-service.log"), []string{"[BACKUP] [2026-11-12 10:01:00] from the service"}, false)

	if _, state := serviceLogsAccess(); state != "readable" {
		t.Fatalf("state = %q, want readable", state)
	}
	a := &App{}
	files, err := a.ListLogFiles()
	if err != nil {
		t.Fatal(err)
	}
	var fromService, fromApp int
	for _, f := range files {
		if f.Folder == 1 && f.Name == "backup-service.log" && f.Service {
			fromService++
		}
		if f.Folder == 0 && f.Name == "service-gui.log" {
			fromApp++
		}
	}
	if fromService != 1 || fromApp != 1 {
		t.Fatalf("service files %d, app files %d, want 1 and 1: %+v", fromService, fromApp, files)
	}
	v, err := a.ReadLog(LogQuery{Folder: 1, Name: "backup-service.log"})
	if err != nil || len(v.Lines) != 1 {
		t.Fatalf("reading the service's log: %v, %+v", err, v)
	}
}

func TestServiceLogsFolderNotListedTwiceWhenThroughTheService(t *testing.T) {
	withLogFolder(t)
	withServiceFolder(t, t.TempDir())
	a := &App{}
	if got := len(a.logDirs()); got != 2 {
		t.Fatalf("not delegated: %d folders, want 2", got)
	}
	a.serviceSettings.Store(true)
	a.apiClient = api.NewClient(filepath.Join(t.TempDir(), "token"))
	if !a.delegated() {
		t.Fatal("test setup: the app should work through the service")
	}
	a.mode = api.ModeService
	a.svcConnected.Store(true)
	if got := len(a.logDirs()); got != 1 {
		t.Fatalf("connected: %d folders, want 1 (the service's come through it)", got)
	}
	// Set to use the service but not connected (the Connect banner): the
	// service's logs do not come through it, so a process that may read the
	// folder lists it itself, and one that may not is told it is locked.
	a.svcConnected.Store(false)
	if got := len(a.logDirs()); got != 2 {
		t.Fatalf("not connected: %d folders, want 2", got)
	}
	if st := a.GetServiceLogsStatus(); !st.Listed || st.State != "readable" {
		t.Fatalf("not connected, readable: %+v", st)
	}
}

func TestSaveLogsPutsAnAdministratorsServiceLogsUnderService(t *testing.T) {
	own := withLogFolder(t)
	svc := t.TempDir()
	withServiceFolder(t, svc)
	writeLogLines(t, filepath.Join(own, "service-gui.log"), []string{"[GUI] [2026-11-12 10:00:00] mine"}, false)
	writeLogLines(t, filepath.Join(svc, "backup-service.log"), []string{"[BACKUP] [2026-11-12 10:01:00] the service's"}, false)
	data, err := (&App{}).buildLogBundle(0)
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	var info string
	for _, f := range zr.File {
		got[f.Name] = true
		if f.Name == "info.txt" {
			rc, _ := f.Open()
			b, _ := io.ReadAll(rc)
			_ = rc.Close()
			info = string(b)
		}
	}
	if !got["app/service-gui.log"] || !got["service/backup-service.log"] || got["app/backup-service.log"] {
		t.Fatalf("zip entries: %v", got)
	}
	if !strings.Contains(info, "1 file(s) from the service") {
		t.Fatalf("info.txt does not say the service's files are in: %q", info)
	}
}

func TestServiceLogsFolderAbsent(t *testing.T) {
	withLogFolder(t)
	withServiceFolder(t, filepath.Join(t.TempDir(), "nothing-here"))
	if _, state := serviceLogsAccess(); state != "none" {
		t.Fatalf("state = %q, want none", state)
	}
	a := &App{}
	if got := len(a.logDirs()); got != 1 {
		t.Fatalf("%d folders, want 1", got)
	}
	if err := a.OpenServiceLogsFolder(); err == nil {
		t.Fatal("opening a folder that is not there should fail")
	}
}

func TestServiceLogsFolderSameAsOwnIsNotDoubled(t *testing.T) {
	own := withLogFolder(t)
	withServiceFolder(t, own)
	if got := len(logFolders()); got != 1 {
		t.Fatalf("the service's own folder was listed twice: %d", got)
	}
}

func TestServiceLogsFolderLockedAsksForAdmin(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs a folder this user may not read (not root, not Windows)")
	}
	withLogFolder(t)
	dir := filepath.Join(t.TempDir(), "locked")
	if err := os.Mkdir(dir, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	withServiceFolder(t, dir)
	if _, state := serviceLogsAccess(); state != "locked" {
		t.Fatalf("state = %q, want locked", state)
	}
	a := &App{}
	if len(a.logDirs()) != 1 {
		t.Fatal("a locked folder must not be listed")
	}
	if err := a.OpenServiceLogsFolder(); err == nil || err.Error() != "SERVICE_LOGS_NEED_ADMIN" {
		t.Fatalf("got %v, want SERVICE_LOGS_NEED_ADMIN", err)
	}
	if st := a.GetServiceLogsStatus(); st.State != "locked" || st.Listed {
		t.Fatalf("status %+v", st)
	}
}
