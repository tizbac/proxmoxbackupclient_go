package main

import (
	"archive/zip"
	"bytes"
	"os"
	"runtime"
	"sort"
	"strings"
	"testing"
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
	a := &App{}
	st := a.GetServiceLogsStatus()
	t.Logf("LIVE status: folder=%s state=%s listed=%v", st.Folder, st.State, st.Listed)

	files, err := a.ListLogFiles()
	if err != nil {
		t.Logf("LIVE list error: %v", err)
	}
	var names []string
	svc := 0
	for _, f := range files {
		where := "own"
		if f.Folder > 0 {
			where = "service-folder"
			svc++
		}
		names = append(names, where+":"+f.Name)
	}
	sort.Strings(names)
	t.Logf("LIVE listed %d files (%d from the service folder): %s", len(files), svc, strings.Join(names, ", "))

	for _, f := range files {
		if f.Folder > 0 {
			v, err := a.ReadLog(LogQuery{Folder: f.Folder, Name: f.Name, Lines: 5})
			t.Logf("LIVE read %s from the service folder: lines=%d err=%v", f.Name, len(v.Lines), err)
			break
		}
	}

	var opened string
	old := openFolderInFileManager
	openFolderInFileManager = func(dir string) error { opened = dir; return nil }
	defer func() { openFolderInFileManager = old }()
	oerr := a.OpenServiceLogsFolder()
	t.Logf("LIVE open service folder: err=%v opened=%q", oerr, opened)
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
