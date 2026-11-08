//go:build windows

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func TestUserHomeOfWindows(t *testing.T) {
	old := profilesDirectory
	profilesDirectory = func() string { return `C:\Users` }
	t.Cleanup(func() { profilesDirectory = old })
	for _, c := range []struct {
		dir, home string
	}{
		{`C:\Users\alice\Documents`, `C:\Users\alice`},
		{`c:\users\Alice\Pictures\2026`, `C:\Users\Alice`},
		{`C:\Users\alice`, `C:\Users\alice`},
		{`C:\Users\Public\Documents`, ``},
		{`C:\Users`, ``},
		{`C:\TestData\Backup`, ``},
		{`D:\Users\alice\x`, ``},
	} {
		got, ok := userHomeOf(c.dir)
		if (c.home == "") == ok || !strings.EqualFold(got, c.home) {
			t.Errorf("userHomeOf(%q) = %q %v, want %q", c.dir, got, ok, c.home)
		}
	}
	// So the safety folder goes in the profile, or at the root of the volume.
	if got := rollbackSafetyRoot(`C:\Users\alice\Documents`); !strings.EqualFold(got, `C:\Users\alice\.pbs-rollback`) {
		t.Errorf("profile folder: safety %s", got)
	}
	if got := rollbackSafetyRoot(`C:\TestData\Backup`); !strings.EqualFold(got, `C:\.pbs-rollback`) {
		t.Errorf("folder outside profiles: safety %s", got)
	}
	if roots := rollbackSafetyRoots(`C:\Users\alice\Documents`); len(roots) != 2 || !strings.EqualFold(roots[1], `C:\.pbs-rollback`) {
		t.Errorf("earlier location not looked at: %v", roots)
	}
}

// The safety folder lets in SYSTEM, the Administrators and this account, and
// inherits nothing from the folder above.
func TestSecureSafetyFolderWindows(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".pbs-rollback")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := secureSafetyFolder(dir); err != nil {
		t.Fatal(err)
	}
	sd, err := windows.GetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	ctl, _, err := sd.Control()
	if err != nil {
		t.Fatal(err)
	}
	if ctl&windows.SE_DACL_PROTECTED == 0 {
		t.Error("the folder still inherits permissions from above")
	}
	// Compare the accounts by SID: the SDDL text shows some of them by
	// alias (the built-in Administrator as "LA").
	dacl, _, err := sd.DACL()
	if err != nil {
		t.Fatal(err)
	}
	u, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"S-1-5-18": false, "S-1-5-32-544": false, u.User.Sid.String(): false}
	for i := 0; i < int(dacl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, uint32(i), &ace); err != nil {
			t.Fatal(err)
		}
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart)).String()
		if _, ok := want[sid]; !ok {
			t.Errorf("%s: lets in %s", sd.String(), sid)
		}
		want[sid] = true
	}
	for sid, seen := range want {
		if !seen {
			t.Errorf("%s: no entry for %s", sd.String(), sid)
		}
	}
}
