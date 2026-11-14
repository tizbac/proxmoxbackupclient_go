package main

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeLog makes a log file of the given size and age.
func writeLog(t *testing.T, dir, name string, size int, age time.Duration) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, bytes.Repeat([]byte("x"), size), 0o600); err != nil {
		t.Fatal(err)
	}
	when := time.Now().Add(-age)
	if err := os.Chtimes(p, when, when); err != nil {
		t.Fatal(err)
	}
}

func zipNames(t *testing.T, data []byte) map[string]*zip.File {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("not a zip: %v", err)
	}
	out := map[string]*zip.File{}
	for _, f := range zr.File {
		out[f.Name] = f
	}
	return out
}

func zipText(t *testing.T, f *zip.File) string {
	t.Helper()
	rc, err := f.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rc.Close() }()
	b, _ := io.ReadAll(rc)
	return string(b)
}

func TestCollectLogsTakesRecentLogsNewestFirstAndSkipsTheRest(t *testing.T) {
	dir := t.TempDir()
	writeLog(t, dir, "service-service.log", 10, time.Hour)
	writeLog(t, dir, "backup-20261110-030000-PC.log.gz", 10, 2*time.Hour)
	writeLog(t, dir, "backup-20261001-030000-PC.log.gz", 10, 40*24*time.Hour) // older than the window
	writeLog(t, dir, "settings.json", 10, time.Minute)                        // not a log
	writeLog(t, dir, "notes.txt", 10, time.Minute)
	files, left := collectLogs([]string{dir, filepath.Join(dir, "missing")}, 14, 1<<20)
	if left != 0 || len(files) != 2 {
		t.Fatalf("collected %+v (left out %d), want the two recent logs", files, left)
	}
	if files[0].name != "service-service.log" || files[1].name != "backup-20261110-030000-PC.log.gz" {
		t.Errorf("order %v %v, want newest first", files[0].name, files[1].name)
	}
}

func TestCollectLogsKeepsTheNewestWhenTheSizeLimitIsHit(t *testing.T) {
	dir := t.TempDir()
	writeLog(t, dir, "backup-20261110-030000-A.log", 600, time.Hour)
	writeLog(t, dir, "backup-20261109-030000-A.log", 600, 25*time.Hour)
	writeLog(t, dir, "backup-20261108-030000-A.log", 600, 49*time.Hour)
	files, left := collectLogs([]string{dir}, 14, 1000)
	if len(files) != 1 || files[0].name != "backup-20261110-030000-A.log" || left != 2 {
		t.Fatalf("collected %+v, left out %d: want only the newest and 2 left out", files, left)
	}
}

func TestCollectLogsAlwaysTakesOneFileEvenOverTheLimit(t *testing.T) {
	dir := t.TempDir()
	writeLog(t, dir, "service-service.log", 5000, time.Minute)
	files, _ := collectLogs([]string{dir}, 14, 100)
	if len(files) != 1 {
		t.Fatalf("a single big log was dropped: %+v", files)
	}
}

func withLogDir(t *testing.T, dir string) {
	t.Helper()
	old := logDir
	logDir = dir
	t.Cleanup(func() { logDir = old })
}

func TestLocalLogBundleHasTheLogsAndAReadme(t *testing.T) {
	dir := t.TempDir()
	withLogDir(t, dir)
	writeLog(t, dir, "service-service.log", 2000, time.Hour)
	writeLog(t, dir, "backup-20261110-030000-PC.log.gz", 300, 2*time.Hour)
	data, err := (&App{}).localLogBundle(0)
	if err != nil {
		t.Fatal(err)
	}
	names := zipNames(t, data)
	for _, want := range []string{"service-service.log", "backup-20261110-030000-PC.log.gz", "info.txt"} {
		if names[want] == nil {
			t.Errorf("the bundle lacks %s (has %v)", want, zipKeys(names))
		}
	}
	if got := zipText(t, names["service-service.log"]); got != strings.Repeat("x", 2000) {
		t.Errorf("the log came out different (%d bytes)", len(got))
	}
	if names["backup-20261110-030000-PC.log.gz"].Method != zip.Store {
		t.Errorf("an already compressed log was compressed again")
	}
	if names["service-service.log"].Method != zip.Deflate {
		t.Errorf("a plain log was not compressed")
	}
	info := zipText(t, names["info.txt"])
	for _, want := range []string{"Version:", "System:", "Saved:", "last 14 days", "service-service.log", "Look through them"} {
		if !strings.Contains(info, want) {
			t.Errorf("info.txt lacks %q:\n%s", want, info)
		}
	}
	// the readme does not name the computer or the user
	if host, err := os.Hostname(); err == nil && host != "" && strings.Contains(info, host) {
		t.Errorf("info.txt names the computer %q", host)
	}
}

func TestBundleOfAnEmptyFolderIsStillAValidZip(t *testing.T) {
	withLogDir(t, t.TempDir())
	data, err := (&App{}).localLogBundle(3)
	if err != nil {
		t.Fatal(err)
	}
	if names := zipNames(t, data); len(names) != 1 || names["info.txt"] == nil {
		t.Fatalf("an empty bundle has %v", zipKeys(zipNames(t, data)))
	}
}

func TestAppBundleWithoutTheServiceSaysSo(t *testing.T) {
	dir := t.TempDir()
	withLogDir(t, dir)
	// No service on this computer, whatever is installed where the test runs.
	withServiceFolder(t, filepath.Join(t.TempDir(), "no-service"))
	writeLog(t, dir, "service-gui.log", 100, time.Minute)
	data, err := (&App{}).buildLogBundle(14)
	if err != nil {
		t.Fatal(err)
	}
	names := zipNames(t, data)
	if names["app/service-gui.log"] == nil {
		t.Errorf("the app's own log is not under app/: %v", zipKeys(names))
	}
	if info := zipText(t, names["info.txt"]); !strings.Contains(info, "there is no background service") {
		t.Errorf("info.txt does not say the service's logs are missing:\n%s", info)
	}
}

func TestMergeLogZipPutsTheServicesLogsUnderServiceAndKeepsTheBytes(t *testing.T) {
	// a zip as the service builds it
	svcDir := t.TempDir()
	withLogDir(t, svcDir)
	writeLog(t, svcDir, "backup-20261110-030000-PC.log", 4000, time.Hour)
	writeLog(t, svcDir, "service-service.log", 500, time.Minute)
	svc, err := (&App{}).localLogBundle(14)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	n, err := mergeLogZip(zw, "service/", svc)
	if err != nil || n != 2 {
		t.Fatalf("merged %d files, %v, want 2", n, err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	names := zipNames(t, out.Bytes())
	if got := zipText(t, names["service/backup-20261110-030000-PC.log"]); got != strings.Repeat("x", 4000) {
		t.Errorf("a merged log came out different (%d bytes)", len(got))
	}
	if names["service/info-service.txt"] == nil {
		t.Errorf("the service's readme is not kept as info-service.txt: %v", zipKeys(names))
	}
}

func TestMergeLogZipNeverWritesOutsideItsFolder(t *testing.T) {
	var evil bytes.Buffer
	ew := zip.NewWriter(&evil)
	w, _ := ew.Create("../../outside.log")
	_, _ = w.Write([]byte("x"))
	_ = ew.Close()
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	if _, err := mergeLogZip(zw, "service/", evil.Bytes()); err != nil {
		t.Fatal(err)
	}
	_ = zw.Close()
	for name := range zipNames(t, out.Bytes()) {
		if strings.Contains(name, "..") || !strings.HasPrefix(name, "service/") {
			t.Errorf("entry %q escapes service/", name)
		}
	}
}

func TestMergeLogZipRefusesSomethingThatIsNotAZip(t *testing.T) {
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	if _, err := mergeLogZip(zw, "service/", []byte("this is not a zip")); err == nil {
		t.Fatal("garbage was accepted")
	}
}

func zipKeys(m map[string]*zip.File) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	return out
}
