package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestMain keeps every test away from this computer's real settings folder
// (config.json, Backup Sets, history, Message Log, keys, caches). A test that
// wrote there replaced a real config.json with dummy values (pbs.example.com),
// and that computer's scheduled backups failed for a week. isolateSettings
// (per OS) points the folder at a scratch one before any test runs; the logs
// already go to testLogDir. An opt-in live test that needs the real folders
// calls useRealSettings.
func TestMain(m *testing.M) {
	scratch, err := os.MkdirTemp("", "pbs-gui-test-")
	if err != nil {
		fmt.Fprintln(os.Stderr, "TestMain:", err)
		os.Exit(1)
	}
	isolateSettings(scratch)

	code := m.Run()

	for _, l := range []*RotatingLogger{backupLogger, serviceLogger} {
		if l != nil {
			_ = l.Close()
		}
	}
	if logDir == testLogDir() {
		_ = os.RemoveAll(logDir)
	}
	_ = os.RemoveAll(scratch)
	os.Exit(code)
}

// The settings folder a test sees is the scratch one, whichever way
// getConfigDir finds it.
func TestSettingsFolderIsScratch(t *testing.T) {
	dir, err := getConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if !isScratchPath(dir) {
		t.Fatalf("settings folder %s is outside the test's scratch folder", dir)
	}
	if !isScratchPath(logDir) {
		t.Fatalf("log folder %s is outside the test's scratch folder", logDir)
	}
}

// isScratchPath: p is inside the temp folder TestMain and testLogDir use.
func isScratchPath(p string) bool {
	rel, err := filepath.Rel(os.TempDir(), p)
	return err == nil && rel != "." && !strings.HasPrefix(rel, "..")
}
