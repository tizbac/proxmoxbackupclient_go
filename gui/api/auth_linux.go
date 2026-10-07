//go:build linux
// +build linux

package api

import (
	"fmt"
	"os"
	"strings"
	"syscall"
)

// EnsureToken returns the local API token stored at path, generating and writing
// a fresh random one if the file is missing or empty.
//
// The file is root-only (0600): the service runs as root and the secret it
// carries authenticates callers to a privileged API, so nobody else may read it
// — not even members of wheel/sudo. A normal user's GUI gets the token through
// an elevated fetch (see token_elevated.go) instead of from this file.
func EnsureToken(path string) (string, error) {
	if b, err := os.ReadFile(path); err == nil {
		if t := strings.TrimSpace(string(b)); t != "" {
			// Migration: builds up to and including the wheel-group scheme wrote
			// 0640 with the group set to wheel/sudo, which let any member of
			// those groups authenticate without an elevation prompt.
			if err := os.Chmod(path, 0o600); err != nil && !os.IsNotExist(err) {
				return "", fmt.Errorf("chmod api token %q: %w", path, err)
			}
			return t, nil
		}
	}

	t, err := GenerateToken()
	if err != nil {
		return "", err
	}
	if err := writeTokenFile(path, t); err != nil {
		return "", err
	}
	return t, nil
}

// canReadTokenFile reports whether this process may take the token at path
// straight from the file: only when the file is ours (the process owns it) or
// we are root. The service's token is root-owned, so an unprivileged GUI never
// reads it directly and always authenticates through an elevation prompt.
func canReadTokenFile(path string) bool {
	fi, err := os.Stat(path)
	if err != nil {
		return false
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return false
	}
	return tokenFileOwnedByProcess(int(st.Uid), os.Geteuid())
}
