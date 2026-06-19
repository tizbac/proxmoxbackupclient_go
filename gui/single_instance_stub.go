//go:build !windows

package main

// CheckSingleInstance is a no-op on non-Windows platforms
// Returns true to allow the instance to start
func CheckSingleInstance() bool {
	return true
}
