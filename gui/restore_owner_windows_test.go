//go:build windows

package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

// unknownSID is an account no computer here knows, as on a file backed up
// from a machine in another domain.
const unknownSID = "S-1-5-21-1111111111-2222222222-3333333333-1001"

func currentUserSIDForTest(t *testing.T) string {
	t.Helper()
	u, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	return u.User.Sid.String()
}

// tokenHoldsPrivilegeForTest reports whether this process's token holds the
// named privilege at all (enabled or not).
func tokenHoldsPrivilegeForTest(t *testing.T, name string) bool {
	t.Helper()
	var luid windows.LUID
	if err := windows.LookupPrivilegeValue(nil, windows.StringToUTF16Ptr(name), &luid); err != nil {
		t.Fatal(err)
	}
	token := windows.GetCurrentProcessToken()
	var n uint32
	_ = windows.GetTokenInformation(token, windows.TokenPrivileges, nil, 0, &n)
	buf := make([]byte, n)
	if err := windows.GetTokenInformation(token, windows.TokenPrivileges, &buf[0], n, &n); err != nil {
		t.Fatal(err)
	}
	for _, p := range (*windows.Tokenprivileges)(unsafe.Pointer(&buf[0])).AllPrivileges() {
		if p.Luid == luid {
			return true
		}
	}
	return false
}

func readSDForTest(t *testing.T, path string) *windows.SECURITY_DESCRIPTOR {
	t.Helper()
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	return sd
}

func newFileForTest(t *testing.T) string {
	t.Helper()
	f := filepath.Join(t.TempDir(), "doc.txt")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	return f
}

// A DACL that was protected (not inheriting) at the source stays protected,
// so the restored file does not pick up the destination folder's entries.
// One that inherited at the source still inherits.
func TestApplyNTFSMetadataProtectedDACL(t *testing.T) {
	me := currentUserSIDForTest(t)
	for _, tc := range []struct {
		name      string
		protected bool
	}{
		{"protected", true},
		{"inheriting", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFileForTest(t)
			flags := ""
			if tc.protected {
				flags = "P"
			}
			sddl := "D:" + flags + "(A;;FA;;;SY)(A;;FA;;;BA)(A;;FA;;;" + me + ")"
			if err := applyNTFSMetadata(f, FileMetaEntry{SDDLIdx: 0}, []string{sddl}); err != nil {
				t.Fatalf("applyNTFSMetadata: %v", err)
			}
			sd := readSDForTest(t, f)
			control, _, err := sd.Control()
			if err != nil {
				t.Fatal(err)
			}
			s := sd.String()
			if got := control&windows.SE_DACL_PROTECTED != 0; got != tc.protected {
				t.Errorf("DACL protected = %v, want %v: %s", got, tc.protected, s)
			}
			if inherited := strings.Contains(s, "ID;"); inherited == tc.protected {
				t.Errorf("inherited entries present = %v, want %v: %s", inherited, !tc.protected, s)
			}
		})
	}
}

// An owner that cannot be set does not cost the file its permissions.
// Elevated, the recorded owner is put back even though this computer does
// not know the account; not elevated, the group and DACL are applied, the
// owner stays the restoring account, and the error says only that. Run it
// both ways (an elevated shell, and runas /trustlevel:0x20000).
func TestApplyNTFSMetadataForeignOwner(t *testing.T) {
	me := currentUserSIDForTest(t)
	f := newFileForTest(t)
	sddl := "O:" + unknownSID + "G:BAD:P(A;;FA;;;SY)(A;;FA;;;BA)(A;;FA;;;" + me + ")(A;;FR;;;" + unknownSID + ")"
	err := applyNTFSMetadata(f, FileMetaEntry{SDDLIdx: 0}, []string{sddl})

	sd := readSDForTest(t, f)
	owner, _, oerr := sd.Owner()
	if oerr != nil {
		t.Fatal(oerr)
	}
	s := sd.String()
	// Which of the two happens depends on whether the token holds
	// SeRestorePrivilege, not on IsElevated: runas /trustlevel gives a token
	// that reports elevated but holds no such privilege.
	holds := tokenHoldsPrivilegeForTest(t, "SeRestorePrivilege")
	switch {
	case holds && err != nil:
		t.Fatalf("token holds SeRestorePrivilege, but the owner was not set: %v", err)
	case err == nil:
		if owner.String() != unknownSID {
			t.Errorf("no error, but owner %s, want %s", owner, unknownSID)
		}
	case errors.Is(err, errOwnerKept):
		if owner.String() != me {
			t.Errorf("owner kept, but owner %s, want the restoring account %s", owner, me)
		}
	default:
		t.Fatalf("applyNTFSMetadata: %v", err)
	}
	if !strings.Contains(s, "(A;;FR;;;"+unknownSID+")") {
		t.Errorf("DACL not restored: %s", s)
	}
	if control, _, _ := sd.Control(); control&windows.SE_DACL_PROTECTED == 0 {
		t.Errorf("DACL lost its protection: %s", s)
	}
	t.Logf("holds SeRestorePrivilege %v, error %v; restored: %s", holds, err, s)
}
