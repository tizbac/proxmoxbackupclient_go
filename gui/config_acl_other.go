//go:build !windows

package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// hardenConfigDir protects a settings folder.
//
// A user's own folder (~/.proxmox-backup-guardian) is made private to its
// owner; files in it are already written 0600.
//
// The service's folder (/var/lib/proxmoxbackupclient) is root's alone: root
// owns everything, folders 0700 and files 0600, as upstream's service keeps
// its state. Earlier builds let the sudo or wheel group read it (setgid,
// group-readable files) so an administrator's app could read the Backup Sets
// and the API token without a prompt; upstream removed that as a
// vulnerability, and this undoes it on each start of the service. The app
// reaches the settings through the service's API instead. Only root can do
// this: for anyone else it does nothing.
func hardenConfigDir(dir string) error {
	if filepath.Clean(dir) != filepath.Clean(sharedConfigDir) {
		return os.Chmod(dir, 0o700)
	}
	if os.Geteuid() != 0 {
		return nil
	}
	start := time.Now()
	if err := os.Chown(dir, 0, 0); err != nil {
		return fmt.Errorf("%s: %w", dir, err)
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return fmt.Errorf("%s: %w", dir, err)
	}
	items, failed := 0, 0
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || p == dir {
			return nil
		}
		items++
		if lerr := os.Lchown(p, 0, 0); lerr != nil {
			failed++
		}
		if d.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		mode := os.FileMode(0o600)
		if d.IsDir() {
			mode = 0o700
		}
		if cerr := os.Chmod(p, mode); cerr != nil {
			failed++
		}
		return nil
	})
	writeDebugLog(fmt.Sprintf("[ConfigACL] %s: root only, %d item(s), %d failed; %dms",
		dir, items, failed, time.Since(start).Milliseconds()))
	return nil
}
