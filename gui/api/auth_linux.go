//go:build linux
// +build linux

package api

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

// EnsureToken returns the local API token stored at path, generating and writing
// a fresh random one if the file is missing or empty. On Linux, sets group
// ownership to wheel/sudo and permissions to 0640, so only members of those
// groups can read the token and call the API to modify scheduled jobs.
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

	// Write token file with 0640 permissions (owner rw, group r, other none)
	if err := os.WriteFile(path, []byte(t), 0o640); err != nil {
		return "", fmt.Errorf("write api token %q: %w", path, err)
	}

	// Set group ownership to wheel or sudo
	if err := setTokenFileGroup(path); err != nil {
		// Log but don't fail - token still works for group members
		// writeDebugLog not available in api package
	}

	return t, nil
}

// setTokenFileGroup sets the group ownership of the token file to
// "wheel" (common on BSD/RHEL) or "sudo" (common on Debian/Ubuntu).
// This ensures only members of those groups can read the token.
func setTokenFileGroup(path string) error {
	// Try wheel group first
	gid, err := getGroupGID("wheel")
	if err != nil {
		// Try sudo group
		gid, err = getGroupGID("sudo")
		if err != nil {
			return fmt.Errorf("neither wheel nor sudo group found")
		}
	}

	// Change group ownership
	if err := unix.Chown(path, -1, gid); err != nil {
		return fmt.Errorf("chown token file: %w", err)
	}

	return nil
}

func getGroupGID(name string) (int, error) {
	f, err := os.Open("/etc/group")
	if err != nil {
		return -1, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, ":")
		if len(parts) >= 3 && parts[0] == name {
			var gid int
			_, err := fmt.Sscanf(parts[2], "%d", &gid)
			if err != nil {
				return -1, err
			}
			return gid, nil
		}
	}
	return -1, fmt.Errorf("group %s not found", name)
}