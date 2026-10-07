//go:build !windows

package main

// CheckSingleInstance is a no-op on non-Windows platforms
// Returns true to allow the instance to start
func CheckSingleInstance() bool {
	return true
}

// ReleaseSingleInstance is a no-op on non-Windows platforms (no instance lock
// is taken there; see single_instance_windows.go for why it exists).
func ReleaseSingleInstance() {}
