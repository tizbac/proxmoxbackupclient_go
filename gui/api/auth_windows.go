//go:build windows
// +build windows

package api

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"golang.org/x/sys/windows"
)

// EnsureToken returns the local API token stored at path, generating and writing
// a fresh random one if the file is missing or empty. The DACL allows only
// Administrators and SYSTEM to read it: on Windows that IS the elevation gate,
// because a non-elevated process (even one started from an Administrators
// account) holds a filtered token whose Administrators SID is deny-only.
func EnsureToken(path string) (string, error) {
	if b, err := os.ReadFile(path); err == nil {
		if t := strings.TrimSpace(string(b)); t != "" {
			// Re-apply the DACL every start: a file written by an older build
			// may still be readable byUsers.
			_ = setTokenFileACL(path)
			return t, nil
		}
	}

	t, err := GenerateToken()
	if err != nil {
		return "", err
	}
	if err := writeTokenFile(path, t); err != nil {
		return "", err
	}

	// Set Windows ACL: only Administrators and SYSTEM can read. Best-effort —
	// the token stays usable by its owner if icacls/ACL plumbing is missing.
	if err := setTokenFileACL(path); err != nil {
		// Log but don't fail - token still works for elevated processes
		// writeDebugLog not available in api package
	}

	return t, nil
}

// canReadTokenFile reports whether this process may take the token at path
// straight from the file. Always yes here: the file's DACL (SYSTEM +
// Administrators, see EnsureToken) already restricts reads to elevated
// processes, so there is no wheel-group style shortcut to close — unlike on
// Unix, where the owner uid is the gate.
func canReadTokenFile(string) bool { return true }

// setTokenFileACL sets the DACL on the token file to allow only
// Administrators and SYSTEM to read it.
// This ensures only elevated processes can read the token and call the API.
//
// The trustees are built from SIDs, not from account names: on a localized
// (non-English) Windows "NT AUTHORITY\SYSTEM" / "BUILTIN\Administrators" do
// not resolve, SetEntriesInAcl fails and the file keeps whatever DACL it
// already had — the same locale bug that, in the icacls-based
// hardenWindowsACLs, used to leave files with an empty DACL that nobody
// (SYSTEM included) could read. S-1-5-18 = SYSTEM, S-1-5-32-544 =
// BUILTIN\Administrators; both are locale-independent.
func setTokenFileACL(path string) error {
	sysSID, err := windows.StringToSid("S-1-5-18")
	if err != nil {
		return fmt.Errorf("system SID: %w", err)
	}
	admSID, err := windows.StringToSid("S-1-5-32-544")
	if err != nil {
		return fmt.Errorf("administrators SID: %w", err)
	}

	trustee := func(sid *windows.SID) windows.TRUSTEE {
		return windows.TRUSTEE{
			TrusteeForm:  windows.TRUSTEE_IS_SID,
			TrusteeType:  windows.TRUSTEE_IS_UNKNOWN,
			TrusteeValue: windows.TrusteeValueFromSID(sid),
		}
	}

	newAcl, err := windows.ACLFromEntries(
		[]windows.EXPLICIT_ACCESS{
			{
				Trustee:           trustee(sysSID),
				AccessPermissions: windows.FILE_GENERIC_READ | windows.FILE_GENERIC_WRITE,
				AccessMode:        windows.SET_ACCESS,
				Inheritance:       windows.NO_INHERITANCE,
			},
			{
				Trustee:           trustee(admSID),
				AccessPermissions: windows.FILE_GENERIC_READ | windows.FILE_GENERIC_WRITE,
				AccessMode:        windows.SET_ACCESS,
				Inheritance:       windows.NO_INHERITANCE,
			},
		},
		nil)
	// The Trustee values hold raw pointers to the SIDs; SetEntriesInAcl (in
	// ACLFromEntries above) is what dereferences them, so keep them alive
	// until it has returned.
	runtime.KeepAlive(sysSID)
	runtime.KeepAlive(admSID)
	if err != nil {
		return fmt.Errorf("create ACL: %w", err)
	}

	// Set the new DACL
	if err := windows.SetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION,
		nil, nil, newAcl, nil); err != nil {
		return fmt.Errorf("set security info: %w", err)
	}

	return nil
}
