package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

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

// fakeLogService stands in for the background service's log requests. It
// keeps what was asked, and whether any request carried the token.
type fakeLogService struct {
	url string

	mu        sync.Mutex
	asked     []string
	withToken bool
}

// newFakeLogService answers GET /logs with list, POST /logs/read with one
// line naming the log, POST /logs/bundle with bundle, and 401 to the rest, as
// the service does to an app that holds no token.
func newFakeLogService(t *testing.T, list []LogFileInfo, bundle []byte) *fakeLogService {
	t.Helper()
	s := &fakeLogService{}
	hs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.asked = append(s.asked, r.Method+" "+r.URL.Path)
		if r.Header.Get("X-Salvanza-Token") != "" {
			s.withToken = true
		}
		s.mu.Unlock()
		var out interface{}
		switch r.Method + " " + r.URL.Path {
		case "GET /logs":
			out = list
		case "POST /logs/read":
			var q LogQuery
			_ = json.NewDecoder(r.Body).Decode(&q)
			out = LogView{Lines: []string{"from the service: " + q.Name}, Matching: 1, Focus: -1}
		case "POST /logs/bundle":
			out = api.LogBundleResponse{Data: bundle}
		default:
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	}))
	t.Cleanup(hs.Close)
	s.url = hs.URL
	return s
}

// app is an app that holds no token (a person who is not an administrator)
// and reaches this service.
func (s *fakeLogService) app(t *testing.T) *App {
	t.Helper()
	// An elevation prompt for reading a log would be a fault.
	api.SetUnauthorizedHook(func(string) (string, error) {
		t.Error("reading the service's logs ran the elevation hook")
		return "", errors.New("no")
	})
	t.Cleanup(func() { api.SetUnauthorizedHook(nil) })
	return &App{apiClient: api.NewClientAt(s.url, filepath.Join(t.TempDir(), "no-token"))}
}

func (s *fakeLogService) saw(want string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, a := range s.asked {
		if a == want {
			return true
		}
	}
	return false
}

// logZip is a zip as the service's /logs/bundle gives: files by name.
func logZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate, Modified: time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(w, files[name]); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// An app that holds no token gets the service's logs from the service: listed
// once (not again from a folder this process happens to read), read through
// the service, and with no elevation prompt.
func TestServiceLogsComeFromTheServiceWithoutTheToken(t *testing.T) {
	own := withLogFolder(t)
	svcDir := t.TempDir()
	withServiceFolder(t, svcDir)
	writeLogLines(t, filepath.Join(own, "service-gui.log"), []string{"[GUI] [2026-11-12 10:00:00] hello"}, false)
	writeLogLines(t, filepath.Join(svcDir, "backup-service.log"), []string{"[BACKUP] [2026-11-12 10:01:00] in the folder"}, false)
	svc := newFakeLogService(t, []LogFileInfo{{Folder: 0, Name: "backup-service.log", Kind: "backup", Size: 44, Modified: time.Now().UnixMilli()}}, nil)
	a := svc.app(t)

	files, err := a.ListLogFiles()
	if err != nil {
		t.Fatal(err)
	}
	var fromService, fromFolder, fromApp int
	for _, f := range files {
		switch {
		case f.Name == "backup-service.log" && f.Folder == serviceLogFolder && f.Service:
			fromService++
		case f.Name == "backup-service.log":
			fromFolder++
		case f.Name == "service-gui.log" && f.Folder == 0:
			fromApp++
		}
	}
	if fromService != 1 || fromFolder != 0 || fromApp != 1 {
		t.Fatalf("from the service %d, from its folder %d, the app's %d; want 1, 0, 1: %+v", fromService, fromFolder, fromApp, files)
	}
	v, err := a.ReadLog(LogQuery{Folder: serviceLogFolder, Name: "backup-service.log"})
	if err != nil || len(v.Lines) != 1 || v.Lines[0] != "from the service: backup-service.log" {
		t.Fatalf("reading the service's log: %v, %+v", err, v)
	}
	if st := a.GetServiceLogsStatus(); !st.Listed || st.State != "readable" {
		t.Fatalf("status %+v, want listed", st)
	}
	if !svc.saw("GET /logs") || !svc.saw("POST /logs/read") {
		t.Fatalf("the service was asked %v", svc.asked)
	}
	if svc.withToken {
		t.Fatal("test setup: the app sent a token")
	}
}

// With the service stopped, a process that may read its folder (an
// administrator, root) lists the logs from there.
func TestServiceLogsFolderIsReadWhenTheServiceDoesNotAnswer(t *testing.T) {
	withLogFolder(t)
	svcDir := t.TempDir()
	withServiceFolder(t, svcDir)
	writeLogLines(t, filepath.Join(svcDir, "backup-service.log"), []string{"[BACKUP] [2026-11-12 10:01:00] in the folder"}, false)
	a := &App{apiClient: api.NewClientAt("http://127.0.0.1:1", filepath.Join(t.TempDir(), "no-token"))}
	files, err := a.ListLogFiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 1 || files[0].Folder != 1 || files[0].Name != "backup-service.log" || !files[0].Service {
		t.Fatalf("listed %+v, want the log in the service's folder", files)
	}
	if st := a.GetServiceLogsStatus(); !st.Listed || st.State != "readable" {
		t.Fatalf("status %+v", st)
	}
}

// The saved zip has the service's logs from the service, once.
func TestSaveLogsTakesTheServicesLogsFromTheService(t *testing.T) {
	own := withLogFolder(t)
	svcDir := t.TempDir()
	withServiceFolder(t, svcDir)
	writeLogLines(t, filepath.Join(own, "service-gui.log"), []string{"[GUI] [2026-11-12 10:00:00] mine"}, false)
	writeLogLines(t, filepath.Join(svcDir, "backup-service.log"), []string{"[BACKUP] [2026-11-12 10:01:00] in the folder"}, false)
	svc := newFakeLogService(t, nil, logZip(t, map[string]string{
		"backup-service.log": "[BACKUP] [2026-11-12 10:01:00] from the service\n",
		"info.txt":           "the service's readme\n",
	}))
	data, err := svc.app(t).buildLogBundle(0)
	if err != nil {
		t.Fatal(err)
	}
	names := zipNames(t, data)
	if len(names) != 4 || names["app/service-gui.log"] == nil || names["service/backup-service.log"] == nil ||
		names["service/info-service.txt"] == nil || names["info.txt"] == nil {
		t.Fatalf("zip entries: %v", zipKeys(names))
	}
	if got := zipText(t, names["service/backup-service.log"]); !strings.Contains(got, "from the service") {
		t.Fatalf("the service's log is not the one the service gave: %q", got)
	}
	if info := zipText(t, names["info.txt"]); !strings.Contains(info, "1 file(s) from the service") {
		t.Fatalf("info.txt does not say the service's files are in: %q", info)
	}
	if !svc.saw("POST /logs/bundle") {
		t.Fatalf("the service was asked %v", svc.asked)
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
	if got := len(logFolders()); got != 1 {
		t.Fatalf("%d folders, want 1", got)
	}
	if err := a.OpenServiceLogsFolder(); err == nil {
		t.Fatal("opening a folder that is not there should fail")
	}
}

// withFileManagerRights says, for a test, whether the file manager has this
// process's rights, and records what OpenServiceLogsFolder opens.
func withFileManagerRights(t *testing.T, lacks bool) *string {
	t.Helper()
	oldLacks, oldOpen := fileManagerLacksOurRights, openFolderInFileManager
	opened := new(string)
	fileManagerLacksOurRights = func() bool { return lacks }
	openFolderInFileManager = func(dir string) error { *opened = dir; return nil }
	t.Cleanup(func() { fileManagerLacksOurRights, openFolderInFileManager = oldLacks, oldOpen })
	return opened
}

func TestOpenServiceLogsFolderOpensTheFolderWhenTheFileManagerMayReadIt(t *testing.T) {
	own := withLogFolder(t)
	svc := t.TempDir()
	withServiceFolder(t, svc)
	opened := withFileManagerRights(t, false)
	a := &App{}
	if st := a.GetServiceLogsStatus(); st.Copy {
		t.Fatalf("status says a copy would be opened: %+v", st)
	}
	if err := a.OpenServiceLogsFolder(); err != nil {
		t.Fatal(err)
	}
	if *opened != svc {
		t.Fatalf("opened %q, want the service's folder %q", *opened, svc)
	}
	if _, err := os.Stat(filepath.Join(own, serviceLogsCopyName)); err == nil {
		t.Fatal("a copy was made though the folder itself was opened")
	}
}

// An app restarted as administrator on Windows: File Explorer is not, and
// would offer to give the user the service's settings folder for good. The
// logs are copied to the user's own folder and that is opened; the settings,
// the token and the keys are not copied.
func TestOpenServiceLogsFolderOpensACopyWhenTheFileManagerLacksRights(t *testing.T) {
	own := withLogFolder(t)
	svc := t.TempDir()
	withServiceFolder(t, svc)
	opened := withFileManagerRights(t, true)
	writeLogLines(t, filepath.Join(svc, "service-service.log"), []string{"[SERVICE] [2026-11-13 10:00:00] started"}, false)
	writeLogLines(t, filepath.Join(svc, "backup-20261113-100100-docs.log.gz"), []string{"[BACKUP] [2026-11-13 10:01:00] a run"}, true)
	for _, private := range []string{"config.json", "api-token", "scheduled_jobs.json"} {
		if err := os.WriteFile(filepath.Join(svc, private), []byte("private"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(svc, "keys"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(svc, "keys", "backup-key.log"), []byte("private"), 0o600); err != nil {
		t.Fatal(err)
	}

	a := &App{}
	if st := a.GetServiceLogsStatus(); !st.Copy || st.State != "readable" {
		t.Fatalf("status %+v, want readable with copy", st)
	}
	if err := a.OpenServiceLogsFolder(); err != nil {
		t.Fatal(err)
	}
	copyDir := filepath.Join(own, serviceLogsCopyName)
	if *opened != copyDir {
		t.Fatalf("opened %q, want the copy %q", *opened, copyDir)
	}
	names := func() []string {
		entries, err := os.ReadDir(copyDir)
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, e := range entries {
			out = append(out, e.Name())
		}
		return out
	}
	if got := strings.Join(names(), ","); got != "backup-20261113-100100-docs.log.gz,service-service.log" {
		t.Fatalf("the copy holds %q, want the two logs and nothing else", got)
	}
	want, _ := os.ReadFile(filepath.Join(svc, "service-service.log"))
	got, _ := os.ReadFile(filepath.Join(copyDir, "service-service.log"))
	if !bytes.Equal(got, want) || len(got) == 0 {
		t.Fatalf("the copied log differs: %q, want %q", got, want)
	}
	si, _ := os.Stat(filepath.Join(svc, "service-service.log"))
	ci, _ := os.Stat(filepath.Join(copyDir, "service-service.log"))
	if !ci.ModTime().Equal(si.ModTime()) {
		t.Fatalf("the copy's time %v, want the log's %v", ci.ModTime(), si.ModTime())
	}

	// A second click replaces the copy: a log that has gone is not left in it.
	if err := os.Remove(filepath.Join(svc, "backup-20261113-100100-docs.log.gz")); err != nil {
		t.Fatal(err)
	}
	if err := a.OpenServiceLogsFolder(); err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(names(), ","); got != "service-service.log" {
		t.Fatalf("after the second copy it holds %q, want service-service.log", got)
	}

	// The copy is a folder in the user's log folder: it is not listed as logs again.
	files, err := a.ListLogFiles()
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, f := range files {
		if f.Name == "service-service.log" {
			seen++
		}
	}
	if seen != 1 {
		t.Fatalf("service-service.log listed %d times, want once", seen)
	}
}

func TestServiceLogsFolderSameAsOwnIsNotDoubled(t *testing.T) {
	own := withLogFolder(t)
	withServiceFolder(t, own)
	if got := len(logFolders()); got != 1 {
		t.Fatalf("the service's own folder was listed twice: %d", got)
	}
}

// lockedServiceFolder makes the service's folder one this user may not read.
func lockedServiceFolder(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs a folder this user may not read (not root, not Windows)")
	}
	dir := filepath.Join(t.TempDir(), "locked")
	if err := os.Mkdir(dir, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	withServiceFolder(t, dir)
	if _, state := serviceLogsAccess(); state != "locked" {
		t.Fatalf("state = %q, want locked", state)
	}
}

// The folder is locked and the service gives no logs (it is stopped): only
// then is an administrator needed.
func TestServiceLogsFolderLockedAndNoServiceAsksForAdmin(t *testing.T) {
	withLogFolder(t)
	lockedServiceFolder(t)
	if len(logFolders()) != 1 {
		t.Fatal("a locked folder must not be listed")
	}
	stopped := &App{apiClient: api.NewClientAt("http://127.0.0.1:1", filepath.Join(t.TempDir(), "no-token"))}
	for name, a := range map[string]*App{"no service client": {}, "service stopped": stopped} {
		if err := a.OpenServiceLogsFolder(); err == nil || err.Error() != "SERVICE_LOGS_NEED_ADMIN" {
			t.Fatalf("%s: got %v, want SERVICE_LOGS_NEED_ADMIN", name, err)
		}
		if st := a.GetServiceLogsStatus(); st.State != "locked" || st.Listed || st.Copy {
			t.Fatalf("%s: status %+v", name, st)
		}
	}
	data, err := stopped.buildLogBundle(0)
	if err != nil {
		t.Fatal(err)
	}
	if info := zipText(t, zipNames(t, data)["info.txt"]); !strings.Contains(info, "the service is not running") {
		t.Fatalf("info.txt does not say why the service's logs are missing:\n%s", info)
	}
}

// The folder is locked and the service answers: a person who is not an
// administrator gets a copy of the logs from the service, in their own log
// folder, and the folder stays as it was.
func TestOpenServiceLogsFolderOpensACopyFromTheServiceWhenLocked(t *testing.T) {
	own := withLogFolder(t)
	lockedServiceFolder(t)
	opened := withFileManagerRights(t, false)
	list := []LogFileInfo{{Folder: 0, Name: "service-service.log", Kind: "app", Size: 10, Modified: time.Now().UnixMilli()}}
	svc := newFakeLogService(t, list, logZip(t, map[string]string{
		"service-service.log": "[SERVICE] [2026-11-13 10:00:00] started\n",
		"info.txt":            "the service's readme\n",
	}))
	a := svc.app(t)
	if st := a.GetServiceLogsStatus(); st.State != "locked" || !st.Listed || !st.Copy {
		t.Fatalf("status %+v, want locked, listed, copy", st)
	}
	if err := a.OpenServiceLogsFolder(); err != nil {
		t.Fatal(err)
	}
	copyDir := filepath.Join(own, serviceLogsCopyName)
	if *opened != copyDir {
		t.Fatalf("opened %q, want the copy %q", *opened, copyDir)
	}
	got, err := os.ReadFile(filepath.Join(copyDir, "service-service.log"))
	if err != nil || !strings.Contains(string(got), "started") {
		t.Fatalf("the copied log: %q, %v", got, err)
	}
	if _, state := serviceLogsAccess(); state != "locked" {
		t.Fatalf("the service's folder is now %q", state)
	}
}

// What a zip says decides neither where a file is written nor how much: only
// log files, under their own plain names, up to the limit.
func TestUnpackServiceLogsWritesOnlyLogFilesInItsFolder(t *testing.T) {
	root := t.TempDir()
	dst := filepath.Join(root, "user", serviceLogsCopyName)
	if err := os.MkdirAll(dst, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dst, "backup-old.log"), []byte("left by an earlier copy"), 0o600); err != nil {
		t.Fatal(err)
	}
	data := logZip(t, map[string]string{
		"service-service.log":                "one\n",
		"backup-20261113-100100-docs.log.gz": "two\n",
		"info.txt":                           "a readme, not a log",
		"config.json":                        "settings",
		"../backup-escaped.log":              "outside",
		"sub/backup-nested.log":              "nested",
		`..\backup-backslash.log`:            "outside",
		"backup-bad name.log":                "a name the viewer would refuse",
	})
	out, err := unpackServiceLogs(data, dst, 1<<20)
	if err != nil || out != dst {
		t.Fatalf("unpackServiceLogs = %q, %v", out, err)
	}
	var found []string
	err = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			rel, _ := filepath.Rel(root, p)
			found = append(found, filepath.ToSlash(rel))
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "user/service-logs-copy/backup-20261113-100100-docs.log.gz,user/service-logs-copy/service-service.log"
	if got := strings.Join(found, ","); got != want {
		t.Fatalf("written: %q\nwant:    %q", got, want)
	}
	info, err := os.Stat(filepath.Join(dst, "service-service.log"))
	if err != nil || !info.ModTime().Equal(time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("the copy's time: %v, %v", info, err)
	}

	// No more than the limit is written, however much the zip unpacks to.
	big := logZip(t, map[string]string{
		"backup-a.log": strings.Repeat("a", 5000),
		"backup-b.log": strings.Repeat("b", 5000),
		"backup-c.log": strings.Repeat("c", 5000),
	})
	if _, err := unpackServiceLogs(big, dst, 7000); err != nil {
		t.Fatal(err)
	}
	var total int64
	entries, err := os.ReadDir(dst)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		fi, err := e.Info()
		if err != nil {
			t.Fatal(err)
		}
		total += fi.Size()
	}
	if total != 7000 {
		t.Fatalf("%d bytes written, want the limit of 7000", total)
	}

	if _, err := unpackServiceLogs([]byte("not a zip"), dst, 1<<20); err == nil {
		t.Fatal("garbage was unpacked")
	}
}
