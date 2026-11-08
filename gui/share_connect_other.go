//go:build !windows

package main

import (
	"errors"
	"os"
	"syscall"
)

// Elsewhere a share is mounted (with its own credentials, by fstab or the
// desktop), and the backups read the mount like any folder: there is no
// \\server\share path to connect to, so a saved login is never used.

func connectShare(root, username, password string) error {
	return errors.New("network share logins are used on Windows only; mount the share instead")
}

func disconnectShare(root string) {}

func isLoginRefusal(err error) bool {
	return errors.Is(err, os.ErrPermission) || errors.Is(err, syscall.EACCES)
}

func resolveMappedDrive(path string) string { return path }
