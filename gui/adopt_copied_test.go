package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func writeAged(t *testing.T, dir, name, content string, age time.Duration) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	when := time.Now().Add(-age)
	if err := os.Chtimes(p, when, when); err != nil {
		t.Fatal(err)
	}
}

// Nimbus Backup 0.4 copied its 0.3 folder (C:\ProgramData\NimbusBackup) to
// C:\ProgramData\ProxmoxBackupClient, carried on there, and left the old
// folder in place with nimbusCopiedMarker. The Nimbus Backup brand's folder
// is that old folder's name: its out-of-date config.json must not stop the
// takeover of the current settings, and it is kept inside the new folder.
func TestAdoptWindowsStateSetsAsideCopiedNimbusFolder(t *testing.T) {
	programData := t.TempDir()
	nimbus := filepath.Join(programData, "NimbusBackup")
	current := filepath.Join(programData, legacyFolder)

	writeAged(t, nimbus, "config.json", `{"from":"nimbus-0.3"}`, 72*time.Hour)
	writeAged(t, nimbus, nimbusCopiedMarker, "copied", 72*time.Hour)
	writeAged(t, current, "config.json", `{"from":"nimbus-0.4"}`, time.Hour)
	writeAged(t, filepath.Join(current, "keys"), "encryption-key.json", `{}`, time.Hour)

	adoptWindowsState(programData, nimbus)

	if data, _ := os.ReadFile(filepath.Join(nimbus, "config.json")); string(data) != `{"from":"nimbus-0.4"}` {
		t.Fatalf("Nimbus folder has %s, want the current (0.4) settings", data)
	}
	if _, err := os.Stat(filepath.Join(nimbus, "keys", "encryption-key.json")); err != nil {
		t.Fatalf("encryption key not taken over: %v", err)
	}
	if _, err := os.Stat(current); !os.IsNotExist(err) {
		t.Fatalf("current folder still there after the takeover: %v", err)
	}
	kept, _ := filepath.Glob(filepath.Join(nimbus, "NimbusBackup.copied-*", "config.json"))
	if len(kept) != 1 {
		t.Fatalf("out-of-date settings not kept inside the folder: %v", kept)
	}
	if data, _ := os.ReadFile(kept[0]); string(data) != `{"from":"nimbus-0.3"}` {
		t.Errorf("kept copy has %s, want the 0.3 settings", data)
	}
	if left, _ := filepath.Glob(filepath.Join(programData, "NimbusBackup.copied-*")); len(left) != 0 {
		t.Errorf("set-aside folder left in ProgramData: %v", left)
	}
}

// A folder that was copied away is never the one taken over, even when its
// config.json is the newest: another brand installed over Nimbus Backup 0.4
// takes over C:\ProgramData\ProxmoxBackupClient.
func TestAdoptFolderSkipsCopiedAwayCandidate(t *testing.T) {
	programData := t.TempDir()
	dst := filepath.Join(programData, "SomeBrand")
	stale := filepath.Join(programData, "NimbusBackup")
	current := filepath.Join(programData, legacyFolder)

	writeAged(t, current, "config.json", `{"from":"current"}`, 48*time.Hour)
	writeAged(t, stale, "config.json", `{"from":"stale"}`, time.Hour)
	writeAged(t, stale, nimbusCopiedMarker, "copied", time.Hour)

	from, err := adoptFolder(dst, []string{stale, current})
	if err != nil {
		t.Fatal(err)
	}
	if !samePath(from, current) {
		t.Fatalf("took over %q, want %q", from, current)
	}
	if data, _ := os.ReadFile(filepath.Join(dst, "config.json")); string(data) != `{"from":"current"}` {
		t.Fatalf("took over %s, want the current settings", data)
	}
}

// With nothing else to take over, a copied-away folder stays as it is: its
// settings are the only ones the computer has.
func TestCopiedAwayFolderKeptWithNothingToTakeOver(t *testing.T) {
	programData := t.TempDir()
	nimbus := filepath.Join(programData, "NimbusBackup")
	writeAged(t, nimbus, "config.json", `{"from":"nimbus-0.3"}`, time.Hour)
	writeAged(t, nimbus, nimbusCopiedMarker, "copied", time.Hour)

	adoptWindowsState(programData, nimbus)

	if data, _ := os.ReadFile(filepath.Join(nimbus, "config.json")); string(data) != `{"from":"nimbus-0.3"}` {
		t.Fatalf("Nimbus folder has %s, want its own settings left alone", data)
	}
	if left, _ := filepath.Glob(filepath.Join(programData, "NimbusBackup.copied-*")); len(left) != 0 {
		t.Errorf("folder set aside with nothing to take over: %v", left)
	}
}

// When the copied-away folder cannot be renamed (here: ProgramData is read
// only, as a file held open does on Windows), its entries are moved into a
// folder inside it and the current settings are copied in.
func TestAdoptWindowsStateCopiedAwayFolderNotRenamable(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs a folder whose entries cannot be renamed: not on Windows or as root")
	}
	programData := t.TempDir()
	nimbus := filepath.Join(programData, "NimbusBackup")
	current := filepath.Join(programData, legacyFolder)

	writeAged(t, nimbus, "config.json", `{"from":"nimbus-0.3"}`, 72*time.Hour)
	writeAged(t, nimbus, nimbusCopiedMarker, "copied", 72*time.Hour)
	writeAged(t, current, "config.json", `{"from":"nimbus-0.4"}`, time.Hour)
	writeAged(t, current, "scheduled_jobs.json", `[]`, time.Hour)

	if err := os.Chmod(programData, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(programData, 0o755) })

	adoptWindowsState(programData, nimbus)

	if data, _ := os.ReadFile(filepath.Join(nimbus, "config.json")); string(data) != `{"from":"nimbus-0.4"}` {
		t.Fatalf("Nimbus folder has %s, want the current (0.4) settings", data)
	}
	if _, err := os.Stat(filepath.Join(nimbus, "scheduled_jobs.json")); err != nil {
		t.Fatalf("Backup Sets not taken over: %v", err)
	}
	if copiedAway(nimbus) {
		t.Error("marker left at the top of the folder after the takeover")
	}
	kept, _ := filepath.Glob(filepath.Join(nimbus, "NimbusBackup.copied-*", "config.json"))
	if len(kept) != 1 {
		t.Fatalf("out-of-date settings not kept inside the folder: %v", kept)
	}
	if data, _ := os.ReadFile(kept[0]); string(data) != `{"from":"nimbus-0.3"}` {
		t.Errorf("kept copy has %s, want the 0.3 settings", data)
	}
}
