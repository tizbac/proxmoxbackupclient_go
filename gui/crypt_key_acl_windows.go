//go:build windows

package main

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// restrictKeyAccess replaces the DACL of path with one that grants full control
// only to the current user, SYSTEM and Administrators, and stops it inheriting
// from the parent. Without this a key written under %ProgramData% inherits
// BUILTIN\Users read access, so any local account could copy it. SYSTEM stays
// in because the background service reads the key.
func restrictKeyAccess(path string, isDir bool) error {
	tokenUser, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return fmt.Errorf("reading current user: %w", err)
	}
	system, err := windows.CreateWellKnownSid(windows.WinLocalSystemSid)
	if err != nil {
		return err
	}
	admins, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err != nil {
		return err
	}

	inherit := uint32(windows.NO_INHERITANCE)
	if isDir {
		inherit = windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT
	}
	var entries []windows.EXPLICIT_ACCESS
	var seen []*windows.SID
	for _, sid := range []*windows.SID{tokenUser.User.Sid, system, admins} {
		dup := false
		for _, s := range seen {
			if s.Equals(sid) {
				dup = true
			}
		}
		if dup {
			continue
		}
		seen = append(seen, sid)
		entries = append(entries, windows.EXPLICIT_ACCESS{
			AccessPermissions: windows.GENERIC_ALL,
			AccessMode:        windows.GRANT_ACCESS,
			Inheritance:       inherit,
			Trustee: windows.TRUSTEE{
				TrusteeForm:  windows.TRUSTEE_IS_SID,
				TrusteeType:  windows.TRUSTEE_IS_UNKNOWN,
				TrusteeValue: windows.TrusteeValueFromSID(sid),
			},
		})
	}
	acl, err := windows.ACLFromEntries(entries, nil)
	if err != nil {
		return fmt.Errorf("building ACL: %w", err)
	}
	return windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil, nil, acl, nil)
}
