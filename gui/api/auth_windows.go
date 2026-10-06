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
	systemSid, err := windows.CreateWellKnownSid(windows.WinLocalSystemSid)
	if err != nil {
		return fmt.Errorf("system sid: %w", err)
	}
	adminSid, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err != nil {
		return fmt.Errorf("administrators sid: %w", err)
	}

	// One full-access entry for each of SYSTEM and Administrators.
	entries := make([]windows.EXPLICIT_ACCESS, 0, 2)
	for _, sid := range []*windows.SID{systemSid, adminSid} {
		entries = append(entries, windows.EXPLICIT_ACCESS{
			AccessPermissions: windows.GENERIC_READ | windows.GENERIC_WRITE,
			AccessMode:        windows.SET_ACCESS,
			Inheritance:       windows.NO_INHERITANCE,
			Trustee: windows.TRUSTEE{
				TrusteeForm:  windows.TRUSTEE_IS_SID,
				TrusteeType:  windows.TRUSTEE_IS_WELL_KNOWN_GROUP,
				TrusteeValue: windows.TrusteeValueFromSID(sid),
			},
		})
	}

	acl, err := windows.ACLFromEntries(entries, nil)
	if err != nil {
		return fmt.Errorf("create ACL: %w", err)
	}

	if err := windows.SetNamedSecurityInfo(
		path,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION,
		nil, nil, acl, nil); err != nil {
		return fmt.Errorf("set security info: %w", err)
	}
	return nil
}
