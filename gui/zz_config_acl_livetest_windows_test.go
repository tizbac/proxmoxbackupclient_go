//go:build windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
)

// Opt-in (PBS_ACL_TEST=1; run elevated): builds a folder under ProgramData
// that inherits ProgramData's defaults, as the settings folder did, with a
// file "planted" by another owner, then checks hardenConfigDir:
//   - BUILTIN\Users and CREATOR OWNER's inherited ProgramData entries are gone;
//   - SYSTEM, Administrators and this (administrator) account remain;
//   - the planted file now belongs to Administrators and only inherits;
//   - an existing explicit, protected ACL (a key file) is left alone.
func TestLiveHardenConfigDir(t *testing.T) {
	if os.Getenv("PBS_ACL_TEST") != "1" {
		t.Skip("set PBS_ACL_TEST=1 (elevated)")
	}
	if !isAdmin() {
		t.Fatal("run elevated")
	}
	dir, err := os.MkdirTemp(os.Getenv("ProgramData"), "pbs-acl-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	cfg := filepath.Join(dir, "config.json")
	planted := filepath.Join(dir, "api-token")
	sub := filepath.Join(dir, "keys")
	key := filepath.Join(sub, "k.json")
	for _, p := range []string{cfg, planted} {
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(sub, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(key, []byte("k"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := restrictKeyAccess(key, false); err != nil {
		t.Fatal(err)
	}
	if out := icacls(t, cfg); !strings.Contains(out, `BUILTIN\Users`) {
		t.Fatalf("setup: expected inherited Users entry:\n%s", out)
	}

	// Plant: the file belongs to the Users group and grants Everyone full control.
	if err := enablePrivileges("SeRestorePrivilege", "SeTakeOwnershipPrivilege"); err != nil {
		t.Fatal(err)
	}
	users, _ := windows.CreateWellKnownSid(windows.WinBuiltinUsersSid)
	everyone, _ := windows.CreateWellKnownSid(windows.WinWorldSid)
	acl, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{aclEntry(everyone, fileAllAccess, windows.NO_INHERITANCE)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(planted, windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION, users, nil, acl, nil); err != nil {
		t.Fatalf("planting: %v", err)
	}

	if err := hardenConfigDir(dir); err != nil {
		t.Fatal(err)
	}

	me, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	meName, meDomain, _, _ := me.User.Sid.LookupAccount("")
	for _, p := range []string{dir, cfg} {
		out := icacls(t, p)
		t.Logf("%s", out)
		if strings.Contains(out, `BUILTIN\Users`) || strings.Contains(out, `CREATOR OWNER:(I)`) {
			t.Errorf("%s still grants BUILTIN\\Users:\n%s", p, out)
		}
		for _, want := range []string{`NT AUTHORITY\SYSTEM`, `BUILTIN\Administrators`, meDomain + `\` + meName} {
			if !strings.Contains(strings.ToLower(out), strings.ToLower(want)) {
				t.Errorf("%s lacks %s:\n%s", p, want, out)
			}
		}
	}
	out := icacls(t, planted)
	if strings.Contains(out, "Everyone") {
		t.Errorf("planted file kept its explicit Everyone entry:\n%s", out)
	}
	sd, err := windows.GetNamedSecurityInfo(planted, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	owner, _, _ := sd.Owner()
	admins, _ := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if !owner.Equals(admins) {
		t.Errorf("planted file owner = %s, want Administrators", owner)
	}
	if out := icacls(t, key); strings.Contains(out, "(I)") {
		t.Errorf("key file lost its own protected ACL:\n%s", out)
	}
}

func icacls(t *testing.T, p string) string {
	t.Helper()
	out, err := exec.Command("icacls", p).CombinedOutput()
	if err != nil {
		t.Fatalf("icacls %s: %v %s", p, err, out)
	}
	return string(out)
}
