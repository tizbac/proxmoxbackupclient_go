//go:build windows
// +build windows

package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

// TestNTFSACLRestoreLive is a manual, opt-in live-integration test proving
// the restore-side half of NTFS ACL/attribute fidelity (see TODO.md
// "Backup Metadata & NTFS Fidelity"): capture already existed
// (gui/backup_meta_windows.go), this test exercises the new restore-side
// application (gui/restore_meta_windows.go + the wiring in
// RestoreSnapshotInline) against a real PBS round-trip, not just a JSON
// round-trip.
//
// Creates a real file with a non-default (deny-ACE) DACL and the Hidden
// attribute, backs it up, restores it to a different destination, and
// checks the restored file's DACL (via SDDL string comparison) and
// attributes match the original.
//
// Run explicitly, as Administrator (SetNamedSecurityInfo needs it):
//
//	set PBS_LIVE_TEST=1
//	go test -tags "" -run TestNTFSACLRestoreLive -v ./...
func TestNTFSACLRestoreLive(t *testing.T) {
	if os.Getenv("PBS_LIVE_TEST") == "" {
		t.Skip("set PBS_LIVE_TEST=1 to run this opt-in live integration test")
	}

	srcDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "acl-test.txt")
	if err := os.WriteFile(srcFile, []byte("ntfs acl/attrs restore round-trip test\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Give it a distinctive, non-default DACL: current user gets Full
	// Control, Everyone is explicitly DENIED Write — something that could
	// never happen from inherited defaults, so a match on restore is a real
	// signal, not a coincidence.
	testSDDL := "D:PAI(D;;WD;;;WD)(A;;FA;;;WD)"
	// D: = DACL, PAI = protected+auto-inherited, D;;WD = deny Write-Data to
	// Everyone (WD), A;;FA = allow Full Access to Everyone (WD) so the test
	// process can still read/delete it regardless of user context. What
	// matters for this test is that the exact SDDL string round-trips.
	sd, err := windows.SecurityDescriptorFromString(testSDDL)
	if err != nil {
		t.Fatalf("SecurityDescriptorFromString: %v", err)
	}
	dacl, _, err := sd.DACL()
	if err != nil || dacl == nil {
		t.Fatalf("DACL: %v", err)
	}
	if err := windows.SetNamedSecurityInfo(srcFile, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION, nil, nil, dacl, nil); err != nil {
		t.Fatalf("SetNamedSecurityInfo (setup): %v", err)
	}

	// Also mark it Hidden, a DOS attribute with nothing to do with the DACL,
	// to check both capture paths independently.
	if pathW, err := windows.UTF16PtrFromString(srcFile); err == nil {
		if err := windows.SetFileAttributes(pathW, windows.FILE_ATTRIBUTE_HIDDEN|windows.FILE_ATTRIBUTE_ARCHIVE); err != nil {
			t.Fatalf("SetFileAttributes (setup): %v", err)
		}
	}

	origSD, err := windows.GetNamedSecurityInfo(srcFile, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatalf("GetNamedSecurityInfo (read back what we just set): %v", err)
	}
	origSDDL := origSD.String()
	t.Logf("Source file DACL SDDL: %s", origSDDL)

	pbsOpts := struct {
		BaseURL, AuthID, Secret, Datastore, CertFingerprint string
	}{
		BaseURL:         "https://192.168.0.242:8007",
		AuthID:          "testclient@pbs!testclient",
		Secret:          "64376a7f-1aeb-42cb-a604-b75bf6a5561b",
		Datastore:       "teststore",
		CertFingerprint: "07:49:87:AE:76:0F:A8:E7:97:D8:1C:F0:44:1A:65:5D:2C:F3:10:49:00:EA:2C:53:E4:0B:CC:09:95:29:C5:22",
	}

	backupID := "ntfs-acl-livetest"
	var resultStatus *BackupStatus

	bOpts := BackupOptions{
		Ctx:             context.Background(),
		BaseURL:         pbsOpts.BaseURL,
		AuthID:          pbsOpts.AuthID,
		Secret:          pbsOpts.Secret,
		Datastore:       pbsOpts.Datastore,
		CertFingerprint: pbsOpts.CertFingerprint,
		BackupObjects:   []string{srcDir},
		BackupID:        backupID,
		BackupType:      "host",
		Kind:            "directory",
		Compression:     "fastest",
		OnResult:        func(s *BackupStatus) { resultStatus = s },
	}
	if err := RunBackupInline(bOpts); err != nil {
		t.Fatalf("RunBackupInline: %v", err)
	}
	if resultStatus == nil || !resultStatus.Success() {
		t.Fatalf("backup did not report success: %+v", resultStatus)
	}
	snapTime := time.Unix(resultStatus.BackupTime, 0)
	t.Logf("Backed up at %s", snapTime)

	destDir := t.TempDir()
	rOpts := RestoreOptions{
		Ctx:             context.Background(),
		BaseURL:         pbsOpts.BaseURL,
		AuthID:          pbsOpts.AuthID,
		Secret:          pbsOpts.Secret,
		Datastore:       pbsOpts.Datastore,
		CertFingerprint: pbsOpts.CertFingerprint,
		BackupID:        backupID,
		SnapshotTime:    snapTime,
		Mode:            RestoreModeAlternateAbs,
		DestPath:        destDir,
		Overwrite:       true,
		RestoreACLs:     true,
	}
	if err := RestoreSnapshotInline(rOpts); err != nil {
		t.Fatalf("RestoreSnapshotInline: %v", err)
	}

	// alternate_abs restores under destDir/<wrapper>/<relative path> — walk
	// to find the restored file rather than hardcoding the wrapper name.
	var restoredFile string
	_ = filepath.Walk(destDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && filepath.Base(path) == "acl-test.txt" {
			restoredFile = path
		}
		return nil
	})
	if restoredFile == "" {
		t.Fatalf("restored file not found under %s", destDir)
	}
	t.Logf("Restored file: %s", restoredFile)

	restoredSD, err := windows.GetNamedSecurityInfo(restoredFile, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatalf("GetNamedSecurityInfo (restored file): %v", err)
	}
	restoredSDDL := restoredSD.String()
	t.Logf("Restored file DACL SDDL: %s", restoredSDDL)
	if restoredSDDL != origSDDL {
		t.Errorf("DACL SDDL mismatch:\n  original: %s\n  restored: %s", origSDDL, restoredSDDL)
	} else {
		t.Log("PASS: DACL round-tripped correctly through backup + restore")
	}

	if pathW, err := windows.UTF16PtrFromString(restoredFile); err == nil {
		attrs, aerr := windows.GetFileAttributes(pathW)
		if aerr != nil {
			t.Fatalf("GetFileAttributes (restored file): %v", aerr)
		}
		if attrs&windows.FILE_ATTRIBUTE_HIDDEN == 0 {
			t.Errorf("restored file is missing the Hidden attribute (attrs=%x)", attrs)
		} else {
			t.Log("PASS: Hidden attribute round-tripped correctly through backup + restore")
		}
	}
}
