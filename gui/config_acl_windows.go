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

// The settings folder (C:\ProgramData\ProxmoxBackupClient) holds config.json
// with the PBS API token secrets, the Backup Sets, encryption keys, logs, and
// the token that lets a program drive the background service, which runs as
// SYSTEM. It used to inherit ProgramData's defaults, which let every local
// account read all of that and create files in it (so a standard user could
// plant an api-token or a config.json before the service wrote its own).
//
// hardenConfigDir replaces the folder's permissions with:
//   - SYSTEM and Administrators: full control;
//   - each local user account in the Administrators group: read, and create
//     files and folders (what BUILTIN\Users had before), so an administrator's
//     GUI running without elevation still reads the settings, writes its logs
//     and talks to the service;
//   - CREATOR OWNER: full control of what that account creates;
//   - nobody else: standard users get no access at all.
//
// The folder's DACL is protected (no longer inherited from ProgramData) and
// the new entries flow down to every file and subfolder that inherits. Files
// owned by an account outside that list (for example planted by a standard
// user before this ran) get Administrators as owner and lose any explicit
// entries. Key files keep their own explicit, protected ACL.
//
// Called by the service when it starts and by the GUI when it runs elevated.
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
	creatorOwner, err := windows.CreateWellKnownSid(windows.WinCreatorOwnerSid)
	if err != nil {
		return err
	}
	adminUsers, err := localAdminUserSIDs()
	if err != nil {
		// Fail safe: without the list only SYSTEM and Administrators get in,
		// so a non-elevated GUI falls back to asking for elevation.
		writeDebugLog(fmt.Sprintf("[ConfigACL] listing local administrators failed: %v", err))
	}

	entries := []windows.EXPLICIT_ACCESS{
		aclEntry(system, fileAllAccess, windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT),
		aclEntry(admins, fileAllAccess, windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT),
		aclEntry(creatorOwner, fileAllAccess, windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT|windows.INHERIT_ONLY),
	}
	for _, u := range adminUsers {
		entries = append(entries,
			aclEntry(u, fileReadExecute, windows.SUB_CONTAINERS_AND_OBJECTS_INHERIT),
			aclEntry(u, fileCreateChildren, windows.SUB_CONTAINERS_ONLY_INHERIT))
	}
	acl, err := windows.ACLFromEntries(entries, nil)
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

	trusted := append([]*windows.SID{system, admins}, adminUsers...)
	fixed := normaliseOwners(dir, trusted, admins)
	writeDebugLog(fmt.Sprintf("[ConfigACL] %s: SYSTEM + Administrators + %d administrator account(s); %d foreign-owned item(s) reset; %v",
		dir, len(adminUsers), fixed, time.Since(start).Round(time.Millisecond)))
	return nil
}

const (
	fileAllAccess   = windows.STANDARD_RIGHTS_REQUIRED | windows.SYNCHRONIZE | 0x1FF // FILE_ALL_ACCESS
	fileReadExecute = 0x001200A9                                                     // FILE_GENERIC_READ | FILE_GENERIC_EXECUTE
	// FILE_ADD_FILE | FILE_ADD_SUBDIRECTORY | FILE_WRITE_EA | FILE_WRITE_ATTRIBUTES,
	// the "WD,AD,WEA,WA" ProgramData gives BUILTIN\Users on folders.
	fileCreateChildren = 0x0002 | 0x0004 | 0x0010 | 0x0100
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
		if err != nil || path == dir {
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

var (
	procNetLocalGroupGetMembers = windows.NewLazySystemDLL("netapi32.dll").NewProc("NetLocalGroupGetMembers")
)

// localAdminUserSIDs lists the user accounts that are direct members of the
// local Administrators group. Groups are left out: under UAC a non-elevated
// token carries admin groups as deny-only, so a grant to a group would not
// reach a non-elevated GUI anyway.
func localAdminUserSIDs() ([]*windows.SID, error) {
	admins, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err != nil {
		return nil, err
	}
	// The group's name is localised ("Administratoren", ...), so look it up.
	name, _, _, err := admins.LookupAccount("")
	if err != nil {
		return nil, fmt.Errorf("looking up the Administrators group: %w", err)
	}
	var buf *byte
	var read, total uint32
	r, _, _ := procNetLocalGroupGetMembers.Call(0,
		uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(name))),
		0, uintptr(unsafe.Pointer(&buf)), 0xFFFFFFFF,
		uintptr(unsafe.Pointer(&read)), uintptr(unsafe.Pointer(&total)), 0)
	if r != 0 {
		return nil, fmt.Errorf("NetLocalGroupGetMembers: %w", windows.Errno(r))
	}
	defer windows.NetApiBufferFree(buf)

	// LOCALGROUP_MEMBERS_INFO_0 is a single PSID.
	members := unsafe.Slice((**windows.SID)(unsafe.Pointer(buf)), read)
	var users []*windows.SID
	for _, m := range members {
		if m == nil || !m.IsValid() {
			continue
		}
		if _, _, use, err := m.LookupAccount(""); err != nil || use != windows.SidTypeUser {
			continue
		}
		c, err := m.Copy()
		if err != nil {
			continue
		}
		users = append(users, c)
	}
	return users, nil
}
