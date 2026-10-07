//go:build !windows

package main

import "fmt"

// requestElevationWindows is unreachable off Windows — RequestElevation
// dispatches on GOOS (Linux uses pkexec/sudo, macOS osascript). It exists so
// main.go compiles on every platform, and is itself compiled only where the
// Windows implementation would be (see elevation_windows.go).
func requestElevationWindows() error {
	return fmt.Errorf("windows elevation is only available on Windows")
}
