//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"testing"
)

// The real values, as the test run found them, for useRealSettings.
var (
	realHome            = os.Getenv("HOME")
	realSharedConfigDir = sharedConfigDir
)

// isolateSettings: getConfigDir uses the service's shared folder once it
// exists, else the home folder. ProgramData would win over both; a Linux run
// has none.
func isolateSettings(scratch string) {
	_ = os.Unsetenv("ProgramData")
	_ = os.Unsetenv("SystemDrive")
	_ = os.Setenv("HOME", filepath.Join(scratch, "home"))
	sharedConfigDir = filepath.Join(scratch, "shared")
	// The service program (-tags service) takes over an earlier install's
	// settings folder by moving it (adoptLinuxState): run as root on a
	// computer that has one, a test would have moved the real folder into
	// the scratch one, which is deleted at the end.
	legacyStateDir = filepath.Join(scratch, "proxmoxbackupclient")
	pbsgoStateDir = filepath.Join(scratch, "pbsgo")
}

// useRealSettings is for an opt-in live test that must use the computer's
// real folders.
func useRealSettings(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", realHome)
	old := sharedConfigDir
	sharedConfigDir = realSharedConfigDir
	t.Cleanup(func() { sharedConfigDir = old })
}
