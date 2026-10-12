//go:build !windows

package main

import (
	"fmt"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"time"
)

// hardenConfigDir protects the settings folder.
//
// A user's own folder (~/.proxmox-backup-guardian) is made private to its
// owner. Files in it are already written 0600; the folder itself was created
// 0755, which let other local accounts list it.
//
// The service's shared folder (/var/lib/proxmoxbackupclient) mirrors what
// C:\ProgramData\ProxmoxBackupClient gets on Windows: root owns everything and
// may change it; the computer's administrators (group sudo, or wheel) may read
// it, so their app, run without sudo, reads the Backup Sets and the API token
// and hands backups to the service; nobody else gets in. The folder is setgid,
// so files written later keep that group, and they are written group-readable
// (sharedFileMode). Only root can do this: for anyone else it does nothing.
func hardenConfigDir(dir string) error {
	if !inSharedConfigDir(dir) {
		return os.Chmod(dir, 0o700)
	}
	if os.Geteuid() != 0 {
		return nil
	}
	start := time.Now()
	gid, group := adminGroup()
	if err := os.Chown(dir, 0, gid); err != nil {
		return fmt.Errorf("%s: %w", dir, err)
	}
	if err := os.Chmod(dir, os.ModeSetgid|0o750); err != nil {
		return fmt.Errorf("%s: %w", dir, err)
	}
	items, failed := 0, 0
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || p == dir {
			return nil
		}
		items++
		if lerr := os.Lchown(p, 0, gid); lerr != nil {
			failed++
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		info, ierr := d.Info()
		if ierr != nil {
			return nil
		}
		mode := sharedFileMode(p, info.Mode().Perm())
		if d.IsDir() {
			mode = os.ModeSetgid | 0o750
		}
		if cerr := os.Chmod(p, mode); cerr != nil {
			failed++
		}
		return nil
	})
	writeDebugLog(fmt.Sprintf("[ConfigACL] %s: root + group %s (read), %d item(s), %d failed; %dms",
		dir, group, items, failed, time.Since(start).Milliseconds()))
	return nil
}

// shareConfigFile gives a file the service wrote outside atomicWriteFile (the
// API token) the shared folder's group-readable mode.
func shareConfigFile(path string) {
	if !inSharedConfigDir(path) {
		return
	}
	if info, err := os.Stat(path); err == nil {
		_ = os.Chmod(path, sharedFileMode(path, info.Mode().Perm()))
	}
}

// adminGroup is the group whose members administer this computer: sudo
// (Debian, Ubuntu, Mint), wheel (Fedora, RHEL, Arch) or admin. Without one
// only root gets in (gid 0).
func adminGroup() (int, string) {
	for _, name := range []string{"sudo", "wheel", "admin"} {
		if g, err := user.LookupGroup(name); err == nil {
			if gid, err := strconv.Atoi(g.Gid); err == nil {
				return gid, name
			}
		}
	}
	return 0, "root"
}
