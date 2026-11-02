//go:build windows

package main

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// keyFileOpenToOthers reports whether other local accounts can read the key
// file at path: its DACL lets Everyone, Users, Authenticated Users or
// Interactive read it, or it has no DACL at all. A key on a drive without
// file permissions (a FAT or exFAT USB stick) is reported as not open: there
// is nothing to restrict, and a stick kept in a drawer is a fair place for it.
func keyFileOpenToOthers(path string) (bool, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return false, err
	}
	vol := make([]uint16, windows.MAX_PATH+1)
	if err := windows.GetVolumePathName(p, &vol[0], uint32(len(vol))); err == nil {
		var flags uint32
		if err := windows.GetVolumeInformation(&vol[0], nil, 0, nil, nil, &flags, nil, 0); err == nil &&
			flags&windows.FILE_PERSISTENT_ACLS == 0 {
			return false, nil
		}
	}

	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return false, fmt.Errorf("reading the permissions of %s: %w", path, err)
	}
	dacl, _, err := sd.DACL()
	if err != nil || dacl == nil {
		return true, nil // no DACL: everyone has full access
	}

	var broad []*windows.SID
	for _, t := range []windows.WELL_KNOWN_SID_TYPE{windows.WinWorldSid, windows.WinBuiltinUsersSid,
		windows.WinAuthenticatedUserSid, windows.WinInteractiveSid} {
		if s, err := windows.CreateWellKnownSid(t); err == nil {
			broad = append(broad, s)
		}
	}
	const readBits = windows.FILE_READ_DATA | windows.GENERIC_READ | windows.GENERIC_ALL
	for i := uint32(0); i < uint32(dacl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, i, &ace); err != nil {
			continue
		}
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE || ace.Header.AceFlags&windows.INHERIT_ONLY_ACE != 0 {
			continue
		}
		if uint32(ace.Mask)&readBits == 0 {
			continue
		}
		sid := (*windows.SID)(unsafe.Pointer(&ace.SidStart))
		for _, b := range broad {
			if sid.Equals(b) {
				return true, nil
			}
		}
	}
	return false, nil
}
