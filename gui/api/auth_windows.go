//go:build windows

package api

import "errors"

// secureTokenFileDefault fails: the service must set SecureTokenFile to the
// ACL code in the gui package before it writes a token.
func secureTokenFileDefault(string) error {
	return errors.New("api token permissions are not set up (SecureTokenFile)")
}

// canReadTokenFile is always true on Windows: the file's ACL (SYSTEM and
// Administrators) is the gate, and a process without elevation simply fails
// to open it.
func canReadTokenFile(string) bool { return true }
