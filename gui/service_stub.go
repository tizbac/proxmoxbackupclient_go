// +build !windows

package main

// RunAsService is a stub for non-Windows platforms
func RunAsService() {
	writeDebugLog("Service mode not supported on this platform")
}

// IsServiceMode always returns false on non-Windows
func IsServiceMode() bool {
	return false
}
