//go:build windows

package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"unsafe"

	"github.com/tizbac/proxmoxbackupclient_go/gui/imagefs"
	"golang.org/x/sys/windows"
	"pbscommon"
)

// restoreImageStreams writes the alternate data streams (Zone.Identifier
// and the like) of a file restored from an NTFS volume in a disk image, as
// dest:name. Returns how many were written; a destination that cannot hold
// streams (FAT, a network share without them) is reported once by the error.
func restoreImageStreams(vol imagefs.Volume, dest string, e imagefs.Entry, buf []byte) (int, error) {
	nv, ok := vol.(*imagefs.NTFS)
	if !ok || len(e.Streams) == 0 {
		return 0, nil
	}
	n := 0
	for _, s := range e.Streams {
		f, err := nv.OpenStream(e, s.Name)
		if err != nil {
			return n, fmt.Errorf("stream %s: %w", s.Name, err)
		}
		out, err := os.Create(dest + ":" + s.Name)
		if err != nil {
			return n, fmt.Errorf("stream %s: %w", s.Name, err)
		}
		_, cerr := io.CopyBuffer(out, io.NewSectionReader(f, 0, f.Size()), buf)
		if err := out.Close(); cerr == nil {
			cerr = err
		}
		if cerr != nil {
			return n, fmt.Errorf("stream %s: %w", s.Name, cerr)
		}
		n++
	}
	return n, nil
}

// restoreImageEAs puts the NTFS extended attributes of an entry restored from
// a disk image (WSL's $LXUID, $LXGID and $LXMOD, say) back on dest, as a
// folder restore does. Returns how many were written.
func restoreImageEAs(vol imagefs.Volume, dest string, e imagefs.Entry) (int, error) {
	nv, ok := vol.(*imagefs.NTFS)
	if !ok || !e.HasEAs {
		return 0, nil
	}
	eas, err := nv.ExtendedAttributes(e)
	if err != nil || len(eas) == 0 {
		return 0, err
	}
	out := make([]pbscommon.EAEntry, len(eas))
	for i, ea := range eas {
		out[i] = pbscommon.EAEntry{Name: ea.Name, Flags: ea.Flags, Value: ea.Value}
	}
	if _, _, err := pbscommon.WriteStreamsAndEAs(dest, nil, out); err != nil {
		return 0, err
	}
	return len(out), nil
}

// imageSDDL returns the SDDL form of an NTFS entry's security descriptor
// from $Secure, caching by security id.
func imageSDDL(vol imagefs.Volume, e imagefs.Entry, cache map[uint32]string) (string, bool) {
	nv, ok := vol.(*imagefs.NTFS)
	if !ok || e.SecurityID == 0 {
		return "", false
	}
	if s, ok := cache[e.SecurityID]; ok {
		return s, s != ""
	}
	sd, ok := nv.SecurityDescriptor(e.SecurityID)
	if !ok || len(sd) < 20 {
		cache[e.SecurityID] = ""
		return "", false
	}
	// A copy, so the descriptor sits in memory Windows may align as it likes.
	buf := make([]byte, len(sd))
	copy(buf, sd)
	s := (*windows.SECURITY_DESCRIPTOR)(unsafe.Pointer(&buf[0])).String()
	cache[e.SecurityID] = s
	return s, s != ""
}

// applyImagePermissions puts an NTFS entry's recorded permissions (owner,
// group, DACL) on the restored dest, as a folder restore does; see
// applyNTFSMetadata for the owner fallback.
func applyImagePermissions(vol imagefs.Volume, dest string, e imagefs.Entry, cache map[uint32]string) (applied, ownerKept bool, err error) {
	sddl, ok := imageSDDL(vol, e, cache)
	if !ok {
		return false, false, nil
	}
	err = applyNTFSMetadata(dest, FileMetaEntry{SDDLIdx: 0}, []string{sddl})
	if errors.Is(err, errOwnerKept) {
		return true, true, nil
	}
	return err == nil, false, err
}
