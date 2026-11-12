package main

import (
	"bufio"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/salvanza/salvanza_client/gui/api"
)

// The log viewer (View Full Logs): the log files the app and the background
// service write, listed with what each is, and read a page at a time, so a
// "3 files skipped, see the log" can be followed up without leaving the app
// (2026-11-01).

// LogFileInfo is one log file the window can show.
type LogFileInfo struct {
	Folder     int    `json:"folder"`             // index into logFolders()
	Name       string `json:"name"`               // file name in that folder
	Kind       string `json:"kind"`               // "run" (one backup), "backup" (backups and restores), "app"
	BackupID   string `json:"backupId,omitempty"` // a run log's backup-id
	Started    int64  `json:"started,omitempty"`  // a run log's start, Unix ms
	Size       int64  `json:"size"`
	Modified   int64  `json:"modified"` // Unix ms
	Compressed bool   `json:"compressed"`
	Service    bool   `json:"service"` // written by the background service
}

// runLogName matches a backup run's own log: backup-<date>-<time>-<id>.log,
// compressed when the run ended, and its rotated parts.
var runLogName = regexp.MustCompile(`^backup-(\d{8})-(\d{6})-(.+?)\.log(\.\d{8}-\d{6})?(\.gz)?$`)

// serviceLogsFolder is where the background service keeps its logs: the
// settings folder under ProgramData on Windows, /var/log/<brand> elsewhere.
// Only the service (and an administrator) may read it.
// A variable so a test can point it at a throwaway folder.
var serviceLogsFolder = func() string {
	if runtime.GOOS == "windows" {
		programData := os.Getenv("ProgramData")
		if programData == "" {
			programData = filepath.Join(os.Getenv("SystemDrive")+string(filepath.Separator), "ProgramData")
		}
		return filepath.Join(programData, folderName())
	}
	return "/var/log/" + folderNameLower()
}

// serviceLogsAccess says whether the service has a log folder on this
// computer and whether this process may read it: "readable", "locked" (it is
// there but this process is refused: an administrator may read it) or "none".
func serviceLogsAccess() (folder, state string) {
	folder = serviceLogsFolder()
	if _, err := os.ReadDir(folder); err == nil {
		return folder, "readable"
	} else if errors.Is(err, fs.ErrPermission) {
		return folder, "locked"
	}
	return folder, "none"
}

// logFolders lists the folders logs are in: this process's, and the service's
// when this process can read it (an administrator or root; an app works
// through the service otherwise).
func logFolders() []string {
	dirs := []string{logDir}
	if folder, state := serviceLogsAccess(); state == "readable" && filepath.Clean(folder) != filepath.Clean(logDir) {
		dirs = append(dirs, folder)
	}
	return dirs
}

// logsThroughService: does this app get the service's logs from the service?
// It does when it works through the service and has its token; a window that
// is set to use the service but is not connected (the Connect banner) does not.
func (a *App) logsThroughService() bool {
	return a.delegated() && (a.mode != api.ModeService || a.svcConnected.Load())
}

// logDirs is logFolders for this app: an app that works through the service
// gets the service's logs from it (folder numbers from serviceLogFolder), so
// its own reading of the service's folder, if it could, is left out.
func (a *App) logDirs() []string {
	dirs := logFolders()
	if a.logsThroughService() && len(dirs) > 1 {
		return dirs[:1]
	}
	return dirs
}

func classifyLog(name string) (kind, backupID string, started int64, ok bool) {
	if !strings.Contains(name, ".log") {
		return "", "", 0, false
	}
	if m := runLogName.FindStringSubmatch(name); m != nil {
		t, err := time.ParseInLocation("20060102150405", m[1]+m[2], time.Local)
		if err == nil {
			return "run", m[3], t.UnixMilli(), true
		}
	}
	switch {
	case strings.HasPrefix(name, "backup-"):
		return "backup", "", 0, true
	case strings.HasPrefix(name, "service-"):
		return "app", "", 0, true
	}
	return "", "", 0, false
}

// serviceLogFolder is added to the folder number of a log the service lists,
// for an app that works through the service: its logs are in the service's
// folder, which only the service may read, so the app asks for them.
const serviceLogFolder = 100

// ListLogFiles lists the log files, newest first: this app's, and with the
// service those it lists (the backups it ran).
func (a *App) ListLogFiles() ([]LogFileInfo, error) {
	out, err := a.listLocalLogFiles()
	if err != nil || !a.delegated() {
		return out, err
	}
	var svc []LogFileInfo
	if err := a.apiClient.ListLogs(&svc); err != nil {
		writeDebugLog(fmt.Sprintf("log viewer: the service's logs: %v", err))
		return out, nil
	}
	for _, f := range svc {
		f.Folder += serviceLogFolder
		f.Service = true
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Modified > out[j].Modified })
	return out, nil
}

// listLocalLogFiles lists the log files in this process's folders.
func (a *App) listLocalLogFiles() ([]LogFileInfo, error) {
	out := []LogFileInfo{}
	for i, dir := range a.logDirs() {
		entries, err := os.ReadDir(dir)
		if err != nil {
			if i == 0 {
				return nil, err
			}
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			kind, id, started, ok := classifyLog(e.Name())
			if !ok {
				continue
			}
			info, err := e.Info()
			if err != nil {
				continue
			}
			out = append(out, LogFileInfo{
				Folder: i, Name: e.Name(), Kind: kind, BackupID: id, Started: started,
				Size: info.Size(), Modified: info.ModTime().UnixMilli(),
				Compressed: strings.HasSuffix(e.Name(), ".gz"),
				Service:    i > 0 || strings.Contains(e.Name(), "-service.log"),
			})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Modified > out[j].Modified })
	return out, nil
}

// LogQuery asks for part of a log file.
type LogQuery struct {
	Folder     int    `json:"folder"`
	Name       string `json:"name"`
	Filter     string `json:"filter"`     // only lines containing this (any case)
	HideChunks bool   `json:"hideChunks"` // leave out the "New chunk"/"Reuse chunk" lines
	Lines      int    `json:"lines"`      // how many lines (default 2000)
	// Around ("2006-01-02 15:04:05", or its first part) gives the lines
	// around the first one logged at or after that time, instead of the end.
	Around string `json:"around"`
}

// LogView is the part of a log ReadLog returns.
type LogView struct {
	Lines    []string `json:"lines"`
	Matching int      `json:"matching"` // lines that pass the filter, in the whole file
	First    int      `json:"first"`    // number (from 0) of the first line returned, among those
	Focus    int      `json:"focus"`    // index in Lines of the Around line, -1 without one
}

const (
	defaultLogLines = 2000
	maxLogLines     = 200000
	maxLogLineLen   = 4000
)

var safeLogName = regexp.MustCompile(`^[A-Za-z0-9._@+-]+$`)

// openLog opens a listed log file for reading, decompressing a .gz one.
func openLog(folder int, name string) (io.ReadCloser, error) {
	dirs := logFolders()
	if folder < 0 || folder >= len(dirs) || name != filepath.Base(name) || !safeLogName.MatchString(name) {
		return nil, fmt.Errorf("no such log file")
	}
	if _, _, _, ok := classifyLog(name); !ok {
		return nil, fmt.Errorf("no such log file")
	}
	f, err := os.Open(filepath.Join(dirs[folder], name))
	if err != nil {
		return nil, err
	}
	if !strings.HasSuffix(name, ".gz") {
		return f, nil
	}
	gz, err := gzip.NewReader(f)
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	return struct {
		io.Reader
		io.Closer
	}{gz, f}, nil
}

// logLineTime returns the "2006-01-02 15:04:05" stamp of a log line
// ("[BACKUP] [2026-10-31 23:26:02] ..."), or "".
func logLineTime(line string) string {
	i := strings.Index(line, "] [")
	if i < 0 || len(line) < i+3+19 {
		return ""
	}
	ts := line[i+3 : i+3+19]
	if ts[4] != '-' || ts[10] != ' ' || ts[13] != ':' {
		return ""
	}
	return ts
}

// ReadLog returns the last q.Lines lines of a log that pass its filter, or
// the ones around q.Around.
func (a *App) ReadLog(q LogQuery) (LogView, error) {
	if q.Folder >= serviceLogFolder && a.delegated() {
		q.Folder -= serviceLogFolder
		var v LogView
		err := a.apiClient.ReadLog(q, &v)
		return v, err
	}
	r, err := openLog(q.Folder, q.Name)
	if err != nil {
		return LogView{}, err
	}
	defer func() { _ = r.Close() }()
	n := q.Lines
	if n <= 0 {
		n = defaultLogLines
	}
	if n > maxLogLines {
		n = maxLogLines
	}
	filter := strings.ToLower(strings.TrimSpace(q.Filter))
	around := strings.TrimSpace(q.Around)

	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	ring := make([]string, 0, n)
	start := 0 // index into ring of the oldest line (once full)
	matching := 0
	focusAt := -1 // number (among matching) of the Around line
	var after []string
	half := n / 2
	for sc.Scan() {
		line := sc.Text()
		if q.HideChunks && (strings.Contains(line, "] New chunk[") || strings.Contains(line, "] Reuse chunk[")) {
			continue
		}
		if filter != "" && !strings.Contains(strings.ToLower(line), filter) {
			continue
		}
		if len(line) > maxLogLineLen {
			line = line[:maxLogLineLen] + " …"
		}
		if around != "" && focusAt < 0 {
			if ts := logLineTime(line); ts != "" && ts >= around {
				focusAt = matching
			}
		}
		matching++
		if focusAt >= 0 && around != "" {
			// Past the Around line: keep half a page after it, then stop.
			if matching-1 > focusAt+(n-half) {
				continue
			}
			after = append(after, line)
			continue
		}
		// Before it (or no Around): the last lines seen.
		limit := n
		if around != "" {
			limit = half
		}
		if len(ring) < limit {
			ring = append(ring, line)
		} else {
			ring[start] = line
			start = (start + 1) % limit
		}
	}
	if err := sc.Err(); err != nil {
		return LogView{}, err
	}
	lines := append(append([]string{}, ring[start:]...), ring[:start]...)
	view := LogView{Matching: matching, Focus: -1}
	if around != "" && focusAt >= 0 {
		view.Focus = len(lines)
		lines = append(lines, after...)
		view.First = focusAt - view.Focus
	} else {
		view.First = matching - len(lines)
	}
	view.Lines = lines
	return view, nil
}

// FindRunLog returns the run log of a backup of backupID that was running at
// at (Unix ms; a Reports entry's or Message Log line's time): the one that
// started last at or before it, or an error when there is none.
func (a *App) FindRunLog(backupID string, at int64) (LogFileInfo, error) {
	files, err := a.ListLogFiles()
	if err != nil {
		return LogFileInfo{}, err
	}
	var best LogFileInfo
	found := false
	for _, f := range files {
		if f.Kind != "run" || f.BackupID != sanitizeForFilename(backupID) || f.Started > at+60000 {
			continue
		}
		if strings.Contains(f.Name, ".log.") && !strings.HasSuffix(f.Name, ".log.gz") {
			continue // a rotated part: the run's main file is the one to open
		}
		if !found || f.Started > best.Started {
			best, found = f, true
		}
	}
	if !found {
		return LogFileInfo{}, fmt.Errorf("no log of a run of %s at that time", backupID)
	}
	return best, nil
}
