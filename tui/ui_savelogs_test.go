package tui

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

func TestSaveLogsWritesAZipAndNeverOverAnExistingFile(t *testing.T) {
	d := newDemoBackend()
	path := filepath.Join(t.TempDir(), "logs.zip")
	if err := d.SaveLogs(path); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("not a zip: %v", err)
	}
	_ = zr.Close()
	if err := d.SaveLogs(path); err == nil {
		t.Error("an existing file was overwritten")
	}
	if err := d.SaveLogs("  "); err == nil {
		t.Error("an empty path was accepted")
	}
}

func TestDefaultLogsPathIsANewZipName(t *testing.T) {
	p := defaultLogsPath()
	if !strings.HasSuffix(p, ".zip") || !strings.Contains(filepath.Base(p), "logs-") {
		t.Errorf("default %q", p)
	}
}

func TestSettingsFilePageSavesLogsFromTheWindow(t *testing.T) {
	h := newHarness(t, 130, 40)
	h.waitFor("service running")
	h.key(tcell.KeyF9, 0)
	h.waitFor("Backup options")
	for i := 0; i < 3; i++ {
		h.key(tcell.KeyPgDn, 0)
	}
	h.waitFor("Save logs")
	if !strings.Contains(h.screen(), "Logs file") {
		t.Fatalf("the page has no Logs file field:\n%s", h.screen())
	}
	dest := filepath.Join(t.TempDir(), "my-logs.zip")
	h.onUI(func() { h.a.pf.logsFile.SetText(dest) })
	h.onUI(h.a.pf.saveLogs)
	h.waitFor("Logs written to")
	if _, err := os.Stat(dest); err != nil {
		t.Fatalf("the file was not written: %v", err)
	}
	if !strings.Contains(h.screen(), "Look through them") {
		t.Errorf("the box does not warn about what the logs name:\n%s", h.screen())
	}
}
