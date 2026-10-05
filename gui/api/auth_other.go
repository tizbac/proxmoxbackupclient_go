//go:build !windows && !linux
// +build !windows,!linux

package api

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
)

// EnsureToken returns the local API token stored at path, generating and writing
// a fresh random one if the file is missing or empty. For other platforms (macOS),
// uses basic 0600 permissions.
func EnsureToken(path string) (string, error) {
	if b, err := os.ReadFile(path); err == nil {
		if t := strings.TrimSpace(string(b)); t != "" {
			return t, nil
		}
	}

	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate api token: %w", err)
	}
	t := hex.EncodeToString(buf)

	if err := os.WriteFile(path, []byte(t), 0o600); err != nil {
		return "", fmt.Errorf("write api token %q: %w", path, err)
	}

	return t, nil
}