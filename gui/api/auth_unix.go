//go:build !windows

package api

import (
	"fmt"
	"os"
	"syscall"
)

// secureTokenFileDefault makes the token file readable by its owner only (the
// service, root). Earlier builds made it 0640 with the sudo or wheel group,
// which let any member of those groups drive the service without a prompt.
func secureTokenFileDefault(path string) error {
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("chmod api token %q: %w", path, err)
	}
	return nil
}

// canReadTokenFile reports whether this process may take the token at path
// straight from the file: only its owner or root. An app of a user who is
// merely allowed to open the file (a leftover group permission) must still
// ask for elevation.
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
