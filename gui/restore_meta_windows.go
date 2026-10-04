//go:build windows
// +build windows

package main

import (
	"fmt"

	"golang.org/x/sys/windows"
)

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
