// +build !windows,!linux

package main

// RunAsService is a stub for non-Windows, non-Linux platforms
func RunAsService() {
	writeDebugLog("Service mode not supported on this platform")
}

// IsServiceMode always returns false on unsupported platforms
func IsServiceMode() bool {
	return false
}
