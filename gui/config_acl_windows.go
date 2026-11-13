//go:build windows

package main

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The settings folder (C:\ProgramData\Salvanza) holds config.json
// with the PBS API token secrets, the Backup Sets, encryption keys, logs, and
// the token that lets a program drive the background service, which runs as
// SYSTEM. It used to inherit ProgramData's defaults, which let every local
// account read all of that and create files in it (so a standard user could
// plant an api-token or a config.json before the service wrote its own).
//
// hardenConfigDir replaces the folder's permissions with:
//   - SYSTEM and Administrators: full control;
//   - nobody else.
//
// A program without elevation holds the Administrators SID as deny-only, so
// only the service and elevated administrators open the folder (upstream's
// strict model); the app reaches the settings through the service's API.
// Earlier builds also let each local administrator account read the folder
// (and create files in it, with CREATOR OWNER rights on them), so an
// administrator's app read the PBS secrets and the API token without a
// prompt; setting the DACL whole removes those entries on the next start.
//
// The folder's DACL is protected (no longer inherited from ProgramData) and
// the new entries flow down to every file and subfolder that inherits. Files
// owned by an account other than SYSTEM or Administrators (planted by a
// standard user, or created by an administrator's app under the old rules)
// get Administrators as owner and lose any explicit entries. Key files and
// the API token keep their own explicit, protected ACL.
//
// Called by the service when it starts.
func hardenConfigDir(dir string) error {
	start := time.Now()
	system, err := windows.CreateWellKnownSid(windows.WinLocalSystemSid)
	if err != nil {
		return err
	}
	admins, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err != nil {
		return err
	}

	acl, err := windows.ACLFromEntries([]windows.EXPLICIT_ACCESS{
		aclEntry(system, fileAllAccess, windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT),
		aclEntry(admins, fileAllAccess, windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT),
	}, nil)
	if err != nil {
		return fmt.Errorf("building ACL: %w", err)
	}
	// SetNamedSecurityInfo also re-computes the inherited entries of every
	// child that inherits, which is what removes BUILTIN\Users from the files.
	if err := windows.SetNamedSecurityInfo(dir, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil, nil, acl, nil); err != nil {
		return fmt.Errorf("setting permissions on %s: %w", dir, err)
	}

	fixed := normaliseOwners(dir, []*windows.SID{system, admins}, admins)
	fixed += inheritUpstreamStateFiles(dir)
	writeDebugLog(fmt.Sprintf("[ConfigACL] %s: SYSTEM + Administrators; %d foreign-owned or locked item(s) reset; %v",
		dir, fixed, time.Since(start).Round(time.Millisecond)))
	return nil
}

// upstreamStateFiles are the settings files the project's 1.2.0-rc service
// locked down one by one (hardenWindowsACLs: SYSTEM only, inheritance
// removed). Its api-token is left out: the service gives it its own
// protected ACL (secureTokenFile).
var upstreamStateFiles = []string{"config.json", "scheduled_jobs.json", "job_history.json"}

// inheritUpstreamStateFiles puts those files back to inheriting the folder's
// permissions after an upgrade from 1.2.0-rc, so an elevated administrator
// can open them as well as the service. Only a protected DACL on one of those
// names is reset: this app protects key files on purpose, never these. The
// service runs as SYSTEM, which those DACLs let in, with one exception:
// Nimbus Backup 0.4.1 could leave a file with a protected DACL without any
// entry, which lets nobody in. The owner may still change it, so the reset
// works there too, and until it has run the service cannot read the file.
// Returns how many were reset.
func inheritUpstreamStateFiles(dir string) int {
	emptyBuf := [8]byte{2, 0, 8, 0, 0, 0, 0, 0} // ACL_REVISION, size 8, 0 ACEs
	empty := (*windows.ACL)(unsafe.Pointer(&emptyBuf[0]))
	n := 0
	for _, name := range upstreamStateFiles {
		path := filepath.Join(dir, name)
		sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
		if err != nil {
			continue // not there
		}
		control, _, err := sd.Control()
		if err != nil || control&windows.SE_DACL_PROTECTED == 0 {
			continue
		}
		if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
			windows.DACL_SECURITY_INFORMATION|windows.UNPROTECTED_DACL_SECURITY_INFORMATION,
			nil, nil, empty, nil); err != nil {
			writeDebugLog(fmt.Sprintf("[ConfigACL] could not unlock %s (locked by an older version): %v", path, err))
			continue
		}
		writeDebugLog(fmt.Sprintf("[ConfigACL] %s locked by an older version: now inherits the folder's permissions", path))
		n++
	}
	return n
}

// unlockTakenOverSettings makes the settings files of a folder just taken
// over from an earlier install readable again where an older version locked
// them (inheritUpstreamStateFiles), so the copy kept for a rollback has them.
// The service locks the whole folder down straight after (hardenConfigDir).
func unlockTakenOverSettings(dir string) {
	inheritUpstreamStateFiles(dir)
}

const (
	fileAllAccess   = windows.STANDARD_RIGHTS_REQUIRED | windows.SYNCHRONIZE | 0x1FF // FILE_ALL_ACCESS
	fileReadExecute = 0x001200A9                                                     // FILE_GENERIC_READ | FILE_GENERIC_EXECUTE
)

func aclEntry(sid *windows.SID, mask windows.ACCESS_MASK, inherit uint32) windows.EXPLICIT_ACCESS {
	return windows.EXPLICIT_ACCESS{
		AccessPermissions: mask,
		AccessMode:        windows.GRANT_ACCESS,
		Inheritance:       inherit,
		Trustee: windows.TRUSTEE{
			TrusteeForm:  windows.TRUSTEE_IS_SID,
			TrusteeType:  windows.TRUSTEE_IS_UNKNOWN,
			TrusteeValue: windows.TrusteeValueFromSID(sid),
		},
	}
}

// normaliseOwners walks dir and gives Administrators ownership of anything
// owned by an account outside trusted, clearing that item's explicit entries so
// it only inherits the folder's. Taking ownership needs SeTakeOwnershipPrivilege
// (SYSTEM and elevated administrators hold it). Returns how many were reset.
func normaliseOwners(dir string, trusted []*windows.SID, newOwner *windows.SID) int {
	_ = enablePrivileges("SeTakeOwnershipPrivilege", "SeRestorePrivilege")
	// An empty DACL (header only, no entries) plus UNPROTECTED leaves the item
	// with just what it inherits. ACLFromEntries cannot build one, and a nil
	// DACL would mean "everyone, full control".
	emptyBuf := [8]byte{2, 0, 8, 0, 0, 0, 0, 0} // ACL_REVISION, size 8, 0 ACEs
	empty := (*windows.ACL)(unsafe.Pointer(&emptyBuf[0]))
	fixed := 0
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if path == dir {
			// The folder itself keeps the permissions just set, but not an
			// outside owner: the owner of a folder can always change its
			// permissions back (a standard user who created the folder
			// before the app was installed).
			fixed += resetForeignOwner(dir, trusted, newOwner)
			return nil
		}
		sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION)
		if err != nil {
			return nil
		}
		owner, _, err := sd.Owner()
		if err != nil || owner == nil {
			return nil
		}
		for _, t := range trusted {
			if owner.Equals(t) {
				return nil
			}
		}
		if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
			windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION|windows.UNPROTECTED_DACL_SECURITY_INFORMATION,
			newOwner, nil, empty, nil); err != nil {
			writeDebugLog(fmt.Sprintf("[ConfigACL] could not reset owner of %s (owned by %s): %v", path, owner.String(), err))
			return nil
		}
		writeDebugLog(fmt.Sprintf("[ConfigACL] reset %s (was owned by %s)", path, owner.String()))
		fixed++
		return nil
	})
	return fixed
}

// resetForeignOwner gives newOwner ownership of path when its owner is not in
// trusted, leaving its permissions as they are. Returns 1 when it did.
func resetForeignOwner(path string, trusted []*windows.SID, newOwner *windows.SID) int {
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION)
	if err != nil {
		return 0
	}
	owner, _, err := sd.Owner()
	if err != nil || owner == nil {
		return 0
	}
	for _, t := range trusted {
		if owner.Equals(t) {
			return 0
		}
	}
	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.OWNER_SECURITY_INFORMATION, newOwner, nil, nil, nil); err != nil {
		writeDebugLog(fmt.Sprintf("[ConfigACL] could not reset owner of %s (owned by %s): %v", path, owner.String(), err))
		return 0
	}
	writeDebugLog(fmt.Sprintf("[ConfigACL] reset owner of %s (was %s)", path, owner.String()))
	return 1
}
