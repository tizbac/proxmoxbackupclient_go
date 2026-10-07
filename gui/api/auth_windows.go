//go:build windows
// +build windows

package api

import (
	"fmt"
	"os"
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
// Administrators and SYSTEM (the service account) to read it.
// This ensures only elevated processes can read the token and call the API.
func setTokenFileACL(path string) error {
	// Build a fresh DACL from scratch (nil merged ACL): only SYSTEM and
	// Administrators get access. Windows resolves the well-known names to
	// SIDs during SetEntriesInAcl.
	newAcl, err := windows.ACLFromEntries(
		[]windows.EXPLICIT_ACCESS{
			{
				Trustee: windows.TRUSTEE{
					TrusteeForm:  windows.TRUSTEE_IS_NAME,
					TrusteeType:  windows.TRUSTEE_IS_UNKNOWN,
					TrusteeValue: windows.TrusteeValueFromString(`NT AUTHORITY\SYSTEM`),
				},
				AccessPermissions: windows.GENERIC_READ | windows.GENERIC_WRITE,
				AccessMode:        windows.SET_ACCESS,
				Inheritance:       windows.NO_INHERITANCE,
			},
			{
				Trustee: windows.TRUSTEE{
					TrusteeForm:  windows.TRUSTEE_IS_NAME,
					TrusteeType:  windows.TRUSTEE_IS_UNKNOWN,
					TrusteeValue: windows.TrusteeValueFromString(`BUILTIN\Administrators`),
				},
				AccessPermissions: windows.GENERIC_READ | windows.GENERIC_WRITE,
				AccessMode:        windows.SET_ACCESS,
				Inheritance:       windows.NO_INHERITANCE,
			},
		},
		nil)
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
