//go:build windows
// +build windows

package main

import (
	"fmt"

	"golang.org/x/sys/windows"
)

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

	if err := windows.SetNamedSecurityInfo(destPath, windows.SE_FILE_OBJECT, secInfo, owner, group, dacl, nil); err != nil {
		return fmt.Errorf("SetNamedSecurityInfo: %w", err)
	}
	return nil
}
