// +build !windows

package main

// CleanupLegacyAutoStart is a no-op on non-Windows platforms
func CleanupLegacyAutoStart() {
	// Nothing to do on non-Windows
}
