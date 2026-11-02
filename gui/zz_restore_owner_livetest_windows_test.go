//go:build windows

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

// Opt-in (PBS_OWNER_TEST=1; run elevated): applyNTFSMetadata puts back an
// owner that is not the restoring account, even one this computer does not
// know (seen on deepthought: "This security ID may not be assigned as the
// owner"), and keeps a protected DACL protected.
func TestLiveRestoreForeignOwner(t *testing.T) {
	if os.Getenv("PBS_OWNER_TEST") != "1" {
		t.Skip("set PBS_OWNER_TEST=1 (elevated)")
	}
	dir := t.TempDir()
	f := filepath.Join(dir, "owned.txt")
	if err := os.WriteFile(f, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	const unknown = "S-1-5-21-1111111111-2222222222-3333333333-1001"
	sddl := "O:" + unknown + "G:BAD:P(A;;FA;;;SY)(A;;FA;;;BA)(A;;FR;;;" + unknown + ")"
	if err := applyNTFSMetadata(f, FileMetaEntry{SDDLIdx: 0}, []string{sddl}); err != nil {
		t.Fatalf("applyNTFSMetadata: %v", err)
	}
	sd, err := windows.GetNamedSecurityInfo(f, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	owner, _, _ := sd.Owner()
	if owner.String() != unknown {
		t.Errorf("owner %s, want %s", owner, unknown)
	}
	control, _, _ := sd.Control()
	if control&windows.SE_DACL_PROTECTED == 0 {
		t.Errorf("DACL lost its protection: %s", sd.String())
	}
	if s := sd.String(); strings.Contains(s, "(A;ID;") {
		t.Errorf("inherited entries crept in: %s", s)
	}
	t.Logf("restored: %s", sd.String())
}
