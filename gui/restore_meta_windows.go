//go:build windows
// +build windows

package main

import (
	"errors"
	"fmt"
	"sync"

	"golang.org/x/sys/windows"
)

// restorePrivileges switches on, once per process, the privileges that let
// an elevated restore put back any recorded owner (SeRestorePrivilege) and
// take over files it does not own (SeTakeOwnershipPrivilege). Windows holds
// them disabled even for an administrator; without them SetNamedSecurityInfo
// refuses an owner other than the restoring account or its groups ("This
// security ID may not be assigned as the owner of this object").
var restorePrivileges = sync.OnceValue(func() error {
	return enablePrivileges("SeRestorePrivilege", "SeTakeOwnershipPrivilege")
})

// applyCreationTime sets a restored file's or folder's creation time to the
// captured FILETIME (0 = not captured, left alone). Only the creation time is
// written; the modification time the extraction set stays as it is.
func applyCreationTime(destPath string, created int64) error {
	if created <= 0 {
		return nil
	}
	p, err := windows.UTF16PtrFromString(destPath)
	if err != nil {
		return err
	}
	// BACKUP_SEMANTICS opens folders too; OPEN_REPARSE_POINT sets the link's
	// own time, not its target's.
	h, err := windows.CreateFile(p, windows.FILE_WRITE_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil,
		windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return fmt.Errorf("open for creation time: %w", err)
	}
	defer windows.CloseHandle(h)
	ft := windows.Filetime{LowDateTime: uint32(created), HighDateTime: uint32(created >> 32)}
	if err := windows.SetFileTime(h, &ft, nil, nil); err != nil {
		return fmt.Errorf("SetFileTime: %w", err)
	}
	return nil
}

// applyNTFSMetadata restores a single file/dir's captured Security
// Descriptor (owner/group/DACL) and DOS attributes, the counterpart to
// NTFSMetaCollector.Collect on the backup side. Best-effort by design,
// matching the collector's own convention: a missing/invalid SDDL or a
// SetNamedSecurityInfo failure is reported to the caller as an error (for
// logging) but must never abort or fail the restore itself — the file's
// actual content already restored successfully regardless.
func applyNTFSMetadata(destPath string, entry FileMetaEntry, sddls []string) error {
	if entry.Attrs != 0 {
		if pathW, err := windows.UTF16PtrFromString(destPath); err == nil {
			_ = windows.SetFileAttributes(pathW, entry.Attrs)
		}
	}

	if entry.SDDLIdx < 0 || entry.SDDLIdx >= len(sddls) {
		return nil
	}
	sddl := sddls[entry.SDDLIdx]
	if sddl == "" {
		return nil
	}

	sd, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		return fmt.Errorf("parse SDDL: %w", err)
	}

	var secInfo windows.SECURITY_INFORMATION
	owner, _, err := sd.Owner()
	if err != nil {
		owner = nil
	} else if owner != nil {
		secInfo |= windows.OWNER_SECURITY_INFORMATION
	}
	group, _, err := sd.Group()
	if err != nil {
		group = nil
	} else if group != nil {
		secInfo |= windows.GROUP_SECURITY_INFORMATION
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		dacl = nil
	} else if dacl != nil {
		secInfo |= windows.DACL_SECURITY_INFORMATION
	}
	if secInfo == 0 {
		return nil
	}
	// A DACL that did not inherit at the source must not start inheriting
	// here (the restored file would gain the destination folder's entries).
	if control, _, err := sd.Control(); err == nil && control&windows.SE_DACL_PROTECTED != 0 && secInfo&windows.DACL_SECURITY_INFORMATION != 0 {
		secInfo |= windows.PROTECTED_DACL_SECURITY_INFORMATION
	}

	_ = restorePrivileges()
	err = windows.SetNamedSecurityInfo(destPath, windows.SE_FILE_OBJECT, secInfo, owner, group, dacl, nil)
	if err != nil && secInfo&windows.OWNER_SECURITY_INFORMATION != 0 &&
		(errors.Is(err, windows.ERROR_INVALID_OWNER) || errors.Is(err, windows.ERROR_ACCESS_DENIED) || errors.Is(err, windows.ERROR_PRIVILEGE_NOT_HELD)) {
		// The recorded owner cannot be set here (not elevated, or an account
		// this computer does not know). Put the permissions back anyway and
		// leave the owner as the restoring account.
		if err2 := windows.SetNamedSecurityInfo(destPath, windows.SE_FILE_OBJECT, secInfo&^windows.OWNER_SECURITY_INFORMATION, nil, group, dacl, nil); err2 == nil {
			return fmt.Errorf("%w (%s): %v", errOwnerKept, owner.String(), err)
		}
	}
	if err != nil {
		return fmt.Errorf("SetNamedSecurityInfo: %w", err)
	}
	return nil
}
