package main

import (
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The log viewer reads the test log folder: these tests swap in their own.
func withLogFolder(t *testing.T) string {
	t.Helper()
	old := logDir
	logDir = t.TempDir()
	t.Cleanup(func() { logDir = old })
	return logDir
}

func writeLogLines(t *testing.T, path string, lines []string, gz bool) {
	t.Helper()
	data := []byte(strings.Join(lines, "\n") + "\n")
	if !gz {
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	w := gzip.NewWriter(f)
	if _, err := w.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

// A run's log: 1000 lines a second apart from 10:00:00, every other one a
// chunk line.
func runLogLines() []string {
	base := time.Date(2026, 11, 1, 10, 0, 0, 0, time.Local)
	var lines []string
	for i := 0; i < 1000; i++ {
		ts := base.Add(time.Duration(i) * time.Second).Format("2006-01-02 15:04:05")
		if i%2 == 0 {
			lines = append(lines, fmt.Sprintf("[BACKUP] [%s] New chunk[%064d] 4194304 bytes", ts, i))
		} else {
			lines = append(lines, fmt.Sprintf("[BACKUP] [%s] line %d", ts, i))
		}
	}
	return lines
}

func TestLogFilesListed(t *testing.T) {
	dir := withLogFolder(t)
	writeLogLines(t, filepath.Join(dir, "backup-20261101-100000-deepthought-data.log.gz"), runLogLines(), true)
	writeLogLines(t, filepath.Join(dir, "backup-gui.log"), []string{"[BACKUP] [2026-11-01 09:00:00] x"}, false)
	writeLogLines(t, filepath.Join(dir, "service-gui.log"), []string{"[SERVICE] [2026-11-01 09:00:00] y"}, false)
	writeLogLines(t, filepath.Join(dir, "backup-gui.log.20261031-235959.gz"), []string{"[BACKUP] [2026-10-31 23:00:00] old"}, true)
	if err := os.WriteFile(filepath.Join(dir, "scheduled_jobs.json"), []byte("[]"), 0o600); err != nil {
		t.Fatal(err)
	}
	files, err := (&App{}).ListLogFiles()
	if err != nil {
		t.Fatal(err)
	}
	kinds := map[string]string{}
	for _, f := range files {
		kinds[f.Name] = f.Kind
		if f.Kind == "run" {
			if f.BackupID != "deepthought-data" || time.UnixMilli(f.Started).Format("2006-01-02 15:04:05") != "2026-11-01 10:00:00" || !f.Compressed {
				t.Errorf("run log listed as %+v", f)
			}
		}
	}
	want := map[string]string{
		"backup-20261101-100000-deepthought-data.log.gz": "run",
		"backup-gui.log":                    "backup",
		"service-gui.log":                   "app",
		"backup-gui.log.20261031-235959.gz": "backup",
	}
	for n, k := range want {
		if kinds[n] != k {
			t.Errorf("%s listed as %q, want %q", n, kinds[n], k)
		}
	}
	if _, ok := kinds["scheduled_jobs.json"]; ok {
		t.Error("a file that is not a log was listed")
	}
}

func TestReadLogEndFilterChunksAround(t *testing.T) {
	dir := withLogFolder(t)
	name := "backup-20261101-100000-deepthought-data.log.gz"
	writeLogLines(t, filepath.Join(dir, name), runLogLines(), true)
	a := &App{}

	// The end, chunk lines hidden: the last 100 of the 500 others.
	v, err := a.ReadLog(LogQuery{Name: name, HideChunks: true, Lines: 100})
	if err != nil {
		t.Fatal(err)
	}
	if v.Matching != 500 || len(v.Lines) != 100 || v.First != 400 || !strings.HasSuffix(v.Lines[99], "line 999") || v.Focus != -1 {
		t.Fatalf("end page: matching %d, %d lines from %d, last %q, focus %d", v.Matching, len(v.Lines), v.First, v.Lines[len(v.Lines)-1], v.Focus)
	}
	for _, l := range v.Lines {
		if strings.Contains(l, "New chunk[") {
			t.Fatal("a chunk line was shown with chunk lines hidden")
		}
	}
	// With chunk lines.
	if v, _ = a.ReadLog(LogQuery{Name: name, Lines: 100}); v.Matching != 1000 {
		t.Fatalf("with chunk lines: matching %d, want 1000", v.Matching)
	}
	// A filter.
	if v, _ = a.ReadLog(LogQuery{Name: name, Filter: "LINE 99", HideChunks: true}); v.Matching != 6 {
		t.Fatalf("filter 'line 99' (any case): matching %d, want 6 (99, 991..999 odd)", v.Matching)
	}
	// Around 10:05:00 (line 300): the page is centred on the first line at or
	// after it.
	v, err = a.ReadLog(LogQuery{Name: name, HideChunks: true, Lines: 20, Around: "2026-11-01 10:05:00"})
	if err != nil {
		t.Fatal(err)
	}
	if v.Focus < 0 || !strings.HasSuffix(v.Lines[v.Focus], "line 301") || len(v.Lines) != 21 || v.Focus != 10 {
		t.Fatalf("around: %d lines, focus %d (%q)", len(v.Lines), v.Focus, func() string {
			if v.Focus >= 0 {
				return v.Lines[v.Focus]
			}
			return ""
		}())
	}
	if v.First != 150-10 {
		t.Errorf("around: first %d, want 140", v.First)
	}
}

func TestReadLogRefusesOtherFiles(t *testing.T) {
	dir := withLogFolder(t)
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	a := &App{}
	for _, name := range []string{"config.json", "../backup-gui.log", `..\service-gui.log`, "backup-gui.log/../../x.log", ""} {
		if _, err := a.ReadLog(LogQuery{Name: name}); err == nil {
			t.Errorf("ReadLog(%q) read a file that is not a listed log", name)
		}
	}
	if _, err := a.ReadLog(LogQuery{Folder: 5, Name: "backup-gui.log"}); err == nil {
		t.Error("ReadLog read from a folder that does not exist")
	}
}

func TestFindRunLog(t *testing.T) {
	dir := withLogFolder(t)
	for _, n := range []string{
		"backup-20261101-080000-deepthought-data.log.gz",
		"backup-20261101-120000-deepthought-data.log.gz",
		"backup-20261101-120000-deepthought-data.log.20261101-130000.gz", // a rotated part
		"backup-20261101-160000-deepthought-data.log",                    // running
		"backup-20261101-120500-deepthought-music.log.gz",
	} {
		writeLogLines(t, filepath.Join(dir, n), []string{"[BACKUP] [2026-11-01 12:00:00] x"}, strings.HasSuffix(n, ".gz"))
	}
	a := &App{}
	at := func(s string) int64 {
		tm, _ := time.ParseInLocation("2006-01-02 15:04:05", s, time.Local)
		return tm.UnixMilli()
	}
	cases := map[string]string{
		"2026-11-01 12:21:42": "backup-20261101-120000-deepthought-data.log.gz", // the 12:00 run ended at 12:21
		"2026-11-01 09:00:00": "backup-20261101-080000-deepthought-data.log.gz",
		"2026-11-01 16:30:00": "backup-20261101-160000-deepthought-data.log",
	}
	for when, want := range cases {
		f, err := a.FindRunLog("deepthought-data", at(when))
		if err != nil || f.Name != want {
			t.Errorf("FindRunLog at %s = %q, %v; want %q", when, f.Name, err, want)
		}
	}
	if _, err := a.FindRunLog("deepthought-data", at("2026-11-01 07:00:00")); err == nil {
		t.Error("found a run log from after the time asked for")
	}
	if _, err := a.FindRunLog("no-such-set", at("2026-11-01 12:00:00")); err == nil {
		t.Error("found a run log for a set that has none")
	}
}
