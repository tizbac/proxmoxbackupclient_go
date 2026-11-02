//go:build windows

package main

// unixServiceStateDir: only reached on Windows when neither ProgramData nor
// SystemDrive is set (tests do this); the home folder, as before.
func unixServiceStateDir() (string, error) {
	return userConfigDir()
}
