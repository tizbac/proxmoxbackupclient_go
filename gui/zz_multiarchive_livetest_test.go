package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// TestMultiArchiveLive is a manual, opt-in live-integration test for the
// 2026-09-21 single-session multi-archive restructure of RunBackupInline
// (see project_windows_pbs_client_fork.md). It drives RunBackupInline
// directly — bypassing the Wails GUI/webview layer entirely, which isn't
// practical to automate on a headless VM — against the ISOLATED test PBS
// (192.168.0.242, datastore teststore). It is gated behind an env var so it
// never runs as part of a normal `go test ./...` and never touches
// production PBS by accident.
//
// Run explicitly (on the test Windows client, 192.168.0.243):
//
//	set PBS_LIVE_TEST=1
//	multiarchive_livetest.test.exe -test.v -test.run TestMultiArchiveLive
func TestMultiArchiveLive(t *testing.T) {
	if os.Getenv("PBS_LIVE_TEST") == "" {
		t.Skip("set PBS_LIVE_TEST=1 to run this opt-in live integration test")
	}

	tmp := t.TempDir()
	dirA := filepath.Join(tmp, "folderA")
	dirB := filepath.Join(tmp, "folderB")
	if err := os.MkdirAll(dirA, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(dirB, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dirA, "hello.txt"), []byte("hello from folder A\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dirB, "world.txt"), []byte("world from folder B\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// A subfolder in B, to exercise the recursive walk too.
	if err := os.MkdirAll(filepath.Join(dirB, "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dirB, "sub", "nested.txt"), []byte("nested file\n"), 0644); err != nil {
		t.Fatal(err)
	}

	var resultStatus *BackupStatus
	opts := BackupOptions{
		Ctx:             context.Background(),
		BaseURL:         "https://192.168.0.242:8007",
		AuthID:          "testclient@pbs!testclient",
		Secret:          "64376a7f-1aeb-42cb-a604-b75bf6a5561b",
		Datastore:       "teststore",
		CertFingerprint: "07:49:87:AE:76:0F:A8:E7:97:D8:1C:F0:44:1A:65:5D:2C:F3:10:49:00:EA:2C:53:E4:0B:CC:09:95:29:C5:22",
		BackupObjects:   []string{dirA, dirB},
		BackupID:        "multiarchive-livetest",
		BackupType:      "host",
		Kind:            "directory",
		Compression:     "fastest",
		OnResult:        func(s *BackupStatus) { resultStatus = s },
		OnProgress: func(pct float64, msg string) {
			t.Logf("[progress %.0f%%] %s", pct*100, msg)
		},
	}

	err := RunBackupInline(opts)
	if err != nil {
		t.Fatalf("RunBackupInline failed: %v", err)
	}
	if resultStatus == nil {
		t.Fatal("no result status delivered via OnResult")
	}
	t.Logf("Outcome=%s Directories=%d NewChunks=%d ReusedChunks=%d TotalBytes=%d Message=%s",
		resultStatus.Outcome, len(resultStatus.Directories), resultStatus.NewChunks,
		resultStatus.ReusedChunks, resultStatus.TotalBytes, resultStatus.Message)

	if len(resultStatus.Directories) != 2 {
		t.Fatalf("expected 2 directories in result, got %d: %+v", len(resultStatus.Directories), resultStatus.Directories)
	}
	for _, d := range resultStatus.Directories {
		if !d.OK {
			t.Fatalf("directory %s reported failure: %s", d.Path, d.Error)
		}
	}
	if !resultStatus.Success() {
		t.Fatalf("expected a successful outcome, got %s: %s", resultStatus.Outcome, resultStatus.Message)
	}

	fmt.Println("TestMultiArchiveLive: PASS — both folders landed in one combined snapshot")
}
