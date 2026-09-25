//go:build linux
// +build linux

package main

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// applyNTFSMetadata restores every extended attribute captured for one
// file/dir, the Linux counterpart to the Windows implementation of the same
// name (see backup_meta_linux.go's doc comment on the shared naming). This
// includes POSIX ACLs, which travel as ordinary xattrs under their reserved
// kernel names (system.posix_acl_access, system.posix_acl_default) and are
// applied via the exact same Setxattr call as any other attribute — nothing
// ACL-specific needed here at all.
//
// sddls is unused on Linux (kept so the call site in restore_inline.go stays
// platform-agnostic, matching applyNTFSMetadata's Windows signature).
//
// Best-effort per attribute: one failing Setxattr (e.g. a security.* xattr
// requiring a capability the restoring process doesn't hold) is reported to
// the caller as part of a combined error but does not stop the rest of this
// file's attributes from being applied, or abort the restore.
func applyNTFSMetadata(destPath string, entry FileMetaEntry, sddls []string) error {
	if len(entry.Xattrs) == 0 {
		return nil
	}
	var errs []string
	for name, val := range entry.Xattrs {
		if err := unix.Setxattr(destPath, name, val, 0); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", name, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("%d of %d xattrs failed: %v", len(errs), len(entry.Xattrs), errs)
	}
	return nil
}
