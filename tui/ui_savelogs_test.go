package tui

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
	home := t.TempDir() // the file name offered is in the home folder
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	h := newHarness(t, 130, 44)
	h.waitFor("service running")
	h.key(tcell.KeyF9, 0)
	h.waitFor("Upload limit")
	for i := 0; i < 3; i++ {
		h.key(tcell.KeyPgDn, 0)
	}
	h.waitFor("Include passwords and secrets")
	// the buttons: Previous page, Export, Import, Save logs (Tab: secrets box, then these)
	for i := 0; i < 5; i++ {
		h.key(tcell.KeyTab, 0)
	}
	h.key(tcell.KeyEnter, 0)
	h.waitFor("overwritten") // the dialog, whose file name is already filled in
	h.key(tcell.KeyTab, 0)  // the Save button
	h.key(tcell.KeyEnter, 0)
	h.waitFor("Logs written to")
	found, _ := filepath.Glob(filepath.Join(home, "salvanza-logs-*.zip"))
	if len(found) != 1 {
		t.Fatalf("expected one zip in the home folder, found %v", found)
	}
	if _, err := os.Stat(found[0]); err != nil {
		t.Fatalf("the file was not written: %v", err)
	}
	// one word, not a phrase: a dialog wraps its lines differently by the length of the path
	if !strings.Contains(h.screen(), "post") {
		t.Errorf("the box does not warn about posting the logs:\n%s", h.screen())
	}
}

func TestSaveLogsDialogCanBeCancelled(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	h := newHarness(t, 130, 44)
	h.waitFor("service running")
	h.key(tcell.KeyF9, 0)
	h.waitFor("Upload limit")
	for i := 0; i < 3; i++ {
		h.key(tcell.KeyPgDn, 0)
	}
	h.waitFor("Include passwords and secrets")
	for i := 0; i < 5; i++ {
		h.key(tcell.KeyTab, 0)
	}
	h.key(tcell.KeyEnter, 0)
	h.waitFor("overwritten")
	h.key(tcell.KeyEscape, 0)
	time.Sleep(300 * time.Millisecond)
	if strings.Contains(h.screen(), "overwritten") {
		t.Errorf("Esc did not close the dialog:\n%s", h.screen())
	}
	if found, _ := filepath.Glob(filepath.Join(home, "salvanza-logs-*.zip")); len(found) != 0 {
		t.Errorf("a file was written although the dialog was cancelled: %v", found)
	}
}
