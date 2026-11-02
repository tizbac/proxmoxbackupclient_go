//go:build windows

package main

import (
	"os"
	"path/filepath"
	"testing"
)

// The real values, as the test run found them, for useRealSettings.
var (
	realProgramData = os.Getenv("ProgramData")
	realUserProfile = os.Getenv("USERPROFILE")
)

// isolateSettings: getConfigDir reads %ProgramData% first, and the home
// folder when a test clears ProgramData and SystemDrive.
func isolateSettings(scratch string) {
	_ = os.Setenv("ProgramData", filepath.Join(scratch, "ProgramData"))
	_ = os.Setenv("USERPROFILE", filepath.Join(scratch, "home"))
}

// useRealSettings is for an opt-in live test that must use the computer's
// real folders.
func useRealSettings(t *testing.T) {
	t.Helper()
	t.Setenv("ProgramData", realProgramData)
	t.Setenv("USERPROFILE", realUserProfile)
}
