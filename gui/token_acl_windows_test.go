//go:build windows

package main

import (
	"os"
	"path/filepath"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The API token file lets in SYSTEM (full control) and the Administrators
// group (read) only, and inherits nothing from its folder.
func TestSecureTokenFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "api-token")
	if err := os.WriteFile(p, []byte("tok"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := secureTokenFile(p); err != nil {
		t.Fatal(err)
	}
	sd, err := windows.GetNamedSecurityInfo(p, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	ctl, _, err := sd.Control()
	if err != nil {
		t.Fatal(err)
	}
	if ctl&windows.SE_DACL_PROTECTED == 0 {
		t.Error("the token file still inherits its folder's permissions")
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]windows.ACCESS_MASK{"S-1-5-18": fileAllAccess, "S-1-5-32-544": fileReadExecute}
	got := map[string]windows.ACCESS_MASK{}
	for i := 0; i < int(dacl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, uint32(i), &ace); err != nil {
			t.Fatal(err)
		}
		got[(*windows.SID)(unsafe.Pointer(&ace.SidStart)).String()] = ace.Mask
	}
	if len(got) != len(want) {
		t.Errorf("%s: %d entries, want SYSTEM and Administrators only", sd.String(), len(got))
	}
	for sid, mask := range want {
		if got[sid] != mask {
			t.Errorf("%s: %s has %#x, want %#x", sd.String(), sid, got[sid], mask)
		}
	}
}
