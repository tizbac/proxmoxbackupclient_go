//go:build !windows && !linux
// +build !windows,!linux

package api

import (
	"fmt"
	"os"
	"strings"
	"syscall"
)

// EnsureToken returns the local API token stored at path, generating and writing
// a fresh random one if the file is missing or empty. The file is owner-only
// (0600): see auth_linux.go for why nobody but the privileged account may read
// it without an elevation prompt.
func EnsureToken(path string) (string, error) {
	if b, err := os.ReadFile(path); err == nil {
		if t := strings.TrimSpace(string(b)); t != "" {
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
// we are root. See auth_linux.go.
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
