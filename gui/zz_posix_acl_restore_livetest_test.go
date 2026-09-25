//go:build linux
// +build linux

package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// TestPosixACLRestoreLive is a manual, opt-in live-integration test proving
// Linux extended-attribute/POSIX-ACL fidelity end to end (see TODO.md
// "Linux side has no equivalent at all"): creates a real file, gives it a
// non-default POSIX ACL via the real `setfacl` tool (so the test doesn't
// just check that Go can read back what Go itself wrote) plus a generic
// user.* xattr, backs it up, restores it, and confirms both round-trip.
//
// Run explicitly, as root (xattr/ACL operations need it on most
// filesystems):
//
//	sudo PBS_LIVE_TEST=1 go test -run TestPosixACLRestoreLive -v ./...
func TestPosixACLRestoreLive(t *testing.T) {
	if os.Getenv("PBS_LIVE_TEST") == "" {
		t.Skip("set PBS_LIVE_TEST=1 to run this opt-in live integration test")
	}
	if _, err := exec.LookPath("setfacl"); err != nil {
		t.Skip("setfacl not installed, skipping (apt install acl)")
	}

	srcDir := t.TempDir()
	srcFile := filepath.Join(srcDir, "acl-test.txt")
	if err := os.WriteFile(srcFile, []byte("posix acl/xattr restore round-trip test\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// A named-user ACE that could never come from the default owner/group/
	// other bits alone — a real signal if it round-trips, not a coincidence.
	// UID 12345 need not exist; setfacl accepts a bare numeric UID.
	cmd := exec.Command("setfacl", "-m", "u:12345:rwx", srcFile)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("setfacl: %v: %s", err, out)
	}

	// Also set a plain user.* xattr, unrelated to ACLs, to check the
	// generic-xattr capture path independently of the ACL-specific one.
	if err := unix.Setxattr(srcFile, "user.proxmox_test_marker", []byte("hello from the test"), 0); err != nil {
		t.Fatalf("Setxattr (setup): %v", err)
	}

	origACL, err := getxattrForTest(srcFile, "system.posix_acl_access")
	if err != nil {
		t.Fatalf("reading back the ACL we just set: %v", err)
	}
	t.Logf("Source file system.posix_acl_access: %d bytes", len(origACL))

	pbsOpts := struct {
		BaseURL, AuthID, Secret, Datastore, CertFingerprint string
	}{
		BaseURL:         "https://192.168.0.242:8007",
		AuthID:          "testclient@pbs!testclient",
		Secret:          "64376a7f-1aeb-42cb-a604-b75bf6a5561b",
		Datastore:       "teststore",
		CertFingerprint: "07:49:87:AE:76:0F:A8:E7:97:D8:1C:F0:44:1A:65:5D:2C:F3:10:49:00:EA:2C:53:E4:0B:CC:09:95:29:C5:22",
	}

	backupID := "posix-acl-livetest"
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

	restoredACL, err := getxattrForTest(restoredFile, "system.posix_acl_access")
	if err != nil {
		t.Fatalf("reading restored file's ACL: %v", err)
	}
	if !bytes.Equal(origACL, restoredACL) {
		t.Errorf("system.posix_acl_access mismatch:\n  original (%d bytes): %x\n  restored (%d bytes): %x",
			len(origACL), origACL, len(restoredACL), restoredACL)
	} else {
		t.Log("PASS: POSIX ACL (system.posix_acl_access) round-tripped correctly through backup + restore")
	}

	// Confirm via the real `getfacl` tool too, not just a raw byte compare —
	// proves the kernel actually accepts and interprets what we wrote back,
	// not just that we copied bytes nobody validated.
	out, err := exec.Command("getfacl", "--omit-header", restoredFile).CombinedOutput()
	if err != nil {
		t.Fatalf("getfacl: %v: %s", err, out)
	}
	t.Logf("getfacl output:\n%s", out)
	if !bytes.Contains(out, []byte("user:12345:rwx")) {
		t.Errorf("getfacl output does not show the restored named-user ACE (user:12345:rwx):\n%s", out)
	} else {
		t.Log("PASS: getfacl confirms the named-user ACE is really there, not just raw bytes")
	}

	restoredMarker, err := getxattrForTest(restoredFile, "user.proxmox_test_marker")
	if err != nil {
		t.Fatalf("reading restored file's user.proxmox_test_marker: %v", err)
	}
	if string(restoredMarker) != "hello from the test" {
		t.Errorf("user.proxmox_test_marker mismatch: got %q", string(restoredMarker))
	} else {
		t.Log("PASS: generic user.* xattr round-tripped correctly through backup + restore")
	}
}

func getxattrForTest(path, name string) ([]byte, error) {
	for size := 256; ; size *= 4 {
		buf := make([]byte, size)
		n, err := unix.Getxattr(path, name, buf)
		if err == unix.ERANGE {
			continue
		}
		if err != nil {
			return nil, err
		}
		return buf[:n], nil
	}
}
