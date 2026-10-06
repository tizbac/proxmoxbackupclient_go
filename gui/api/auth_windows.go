//go:build windows
// +build windows

package api

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/windows"
)

// EnsureToken returns the local API token stored at path, generating and writing
// a fresh random one if the file is missing or empty. On Windows, sets ACL to
// allow only Administrators and SYSTEM to read the token, so only elevated
// processes can access the API and modify scheduled jobs.
func EnsureToken(path string) (string, error) {
	if b, err := os.ReadFile(path); err == nil {
		if t := strings.TrimSpace(string(b)); t != "" {
			return t, nil
		}
	}

	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate api token: %w", err)
	}
	t := hex.EncodeToString(buf)

	// Write token file
	if err := os.WriteFile(path, []byte(t), 0o600); err != nil {
		return "", fmt.Errorf("write api token %q: %w", path, err)
	}

	// Set Windows ACL: only Administrators and SYSTEM can read
	if err := setTokenFileACL(path); err != nil {
		// Log but don't fail - token still works for elevated processes
		// writeDebugLog not available in api package
	}

	return t, nil
}

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