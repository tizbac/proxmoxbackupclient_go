//go:build windows

package main

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// secureTokenFile gives the API token file SYSTEM full control and the
// Administrators group read, and nothing inherited (upstream's
// setTokenFileACL, with the inheritance cut that upstream's icacls step did
// separately). A program without elevation holds the Administrators SID as
// deny-only, so it cannot read the file even under an administrator account:
// it must ask through UAC. Trustees are SIDs, so a localised Windows works.
func secureTokenFile(path string) error {
	system, err := windows.CreateWellKnownSid(windows.WinLocalSystemSid)
	if err != nil {
		return err
	}
	admins, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err != nil {
		return err
	}
	acl, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{
		aclEntry(system, fileAllAccess, windows.NO_INHERITANCE),
		aclEntry(admins, fileReadExecute, windows.NO_INHERITANCE),
	}, nil)
	if err != nil {
		return fmt.Errorf("building the API token's ACL: %w", err)
	}
	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil, nil, acl, nil); err != nil {
		return fmt.Errorf("setting the API token's permissions on %s: %w", path, err)
	}
	return nil
}
