package main

import (
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/salvanza/salvanza_client/gui/api"
)

// A live test: what this process, run as some user, sees of the INSTALLED
// service's logs. Off unless SALVANZA_LIVE_LOGS=1 (it reads the real service
// folder). Run it as an administrator and as a standard user, and compare.
func TestLiveServiceLogsAccess(t *testing.T) {
	if os.Getenv("SALVANZA_LIVE_LOGS") != "1" {
		t.Skip("SALVANZA_LIVE_LOGS=1 to run against the installed service's logs")
	}
	// The test harness points ProgramData at a throwaway folder; this test wants the real one.
	real := `C:\ProgramData\Salvanza`
	if runtime.GOOS != "windows" {
		real = "/var/log/salvanza"
	}
	withServiceFolder(t, real)
	// As the app: it asks the installed service, here with no token at all,
	// and must never need an elevation prompt for a log.
	hooked := false
	api.SetUnauthorizedHook(func(string) (string, error) { hooked = true; return "", errors.New("no") })
	defer api.SetUnauthorizedHook(nil)
	a := &App{apiClient: api.NewClient(filepath.Join(t.TempDir(), "no-token"))}
	defer func() { t.Logf("LIVE elevation asked for: %v", hooked) }()
	st := a.GetServiceLogsStatus()
	t.Logf("LIVE status: folder=%s state=%s listed=%v copy=%v", st.Folder, st.State, st.Listed, st.Copy)

	files, err := a.ListLogFiles()
	if err != nil {
		t.Logf("LIVE list error: %v", err)
	}
	var names []string
	svc := 0
	for _, f := range files {
		where := "own"
		switch {
		case f.Folder >= serviceLogFolder:
			where = "from-service"
			svc++
		case f.Folder > 0:
			where = "service-folder"
			svc++
		}
		names = append(names, where+":"+f.Name)
	}
	sort.Strings(names)
	t.Logf("LIVE listed %d files (%d of the service's): %s", len(files), svc, strings.Join(names, ", "))

	for _, f := range files {
		if f.Folder > 0 {
			v, err := a.ReadLog(LogQuery{Folder: f.Folder, Name: f.Name, Lines: 5})
			t.Logf("LIVE read %s (folder %d): lines=%d err=%v", f.Name, f.Folder, len(v.Lines), err)
			break
		}
	}

	var opened string
	old := openFolderInFileManager
	openFolderInFileManager = func(dir string) error { opened = dir; return nil }
	defer func() { openFolderInFileManager = old }()
	oerr := a.OpenServiceLogsFolder()
	t.Logf("LIVE open service folder: err=%v opened=%q", oerr, opened)
	if opened != "" && opened != real {
		if es, err := os.ReadDir(opened); err == nil {
			var copied []string
			for _, e := range es {
				copied = append(copied, e.Name())
			}
			sort.Strings(copied)
			t.Logf("LIVE the copy holds %d files: %s", len(copied), strings.Join(copied, ", "))
		}
	}
	opened = ""
	oerr = a.OpenLogsFolder()
	t.Logf("LIVE open own folder: err=%v opened=%q", oerr, opened)

	data, err := a.buildLogBundle(0)
	if err != nil {
		t.Logf("LIVE bundle error: %v", err)
		return
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	var entries []string
	for _, f := range zr.File {
		entries = append(entries, f.Name)
	}
	sort.Strings(entries)
	t.Logf("LIVE bundle entries: %s", strings.Join(entries, ", "))
}
