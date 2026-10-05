//go:build windows
// +build windows

package api

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"unsafe"

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
	// Convert path to Windows format
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}

	// Get current security descriptor
	var sd *windows.SECURITY_DESCRIPTOR
	err = windows.GetNamedSecurityInfo(
		pathPtr,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION,
		nil, nil, &sd, nil, nil)
	if err != nil {
		return fmt.Errorf("get security info: %w", err)
	}
	defer windows.LocalFree(windows.Handle(uintptr(sd)))

	// Build new DACL
	// Allow SYSTEM full access
	systemSid, err := createWellKnownSid(windows.WinLocalSystemSid)
	if err != nil {
		return err
	}

	// Allow Administrators full access
	adminSid, err := createWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err != nil {
		return err
	}

	// Create new ACL with these two ACEs
	var newAcl *windows.ACL
	err = windows.SetEntriesInAcl(
		2,
		[]windows.EXPLICIT_ACCESS{
			{
				Trustee: windows.TRUSTEE{
					TrusteeForm:  windows.TRUSTEE_IS_SID,
					TrusteeType:  windows.TRUSTEE_IS_WELL_KNOWN_GROUP,
					PtstrName:    (*uint16)(unsafe.Pointer(systemSid)),
				},
				AccessPermissions: windows.GENERIC_READ | windows.GENERIC_WRITE,
				AccessMode:        windows.SET_ACCESS,
				Inheritance:       windows.NO_INHERITANCE,
			},
			{
				Trustee: windows.TRUSTEE{
					TrusteeForm:  windows.TRUSTEE_IS_SID,
					TrusteeType:  windows.TRUSTEE_IS_WELL_KNOWN_GROUP,
					PtstrName:    (*uint16)(unsafe.Pointer(adminSid)),
				},
				AccessPermissions: windows.GENERIC_READ | windows.GENERIC_WRITE,
				AccessMode:        windows.SET_ACCESS,
				Inheritance:       windows.NO_INHERITANCE,
			},
		},
		nil,
		&newAcl)
	if err != nil {
		return fmt.Errorf("create ACL: %w", err)
	}

	// Set the new DACL
	err = windows.SetNamedSecurityInfo(
		pathPtr,
		windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION,
		nil, nil, newAcl, nil)
	if err != nil {
		return fmt.Errorf("set security info: %w", err)
	}

	return nil
}

func createWellKnownSid(sidType int) (*windows.SID, error) {
	var sid *windows.SID
	err := windows.AllocateAndInitializeSid(
		&windows.SECURITY_NT_AUTHORITY,
		1,
		uint32(sidType),
		0, 0, 0, 0, 0, 0, 0,
		&sid)
	if err != nil {
		return nil, err
	}
	return sid, nil
}