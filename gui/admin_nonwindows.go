//go:build !windows
// +build !windows

package main

import (
	"os"
)

// isAdmin returns true if running as root (euid == 0) on non-Windows systems
func isAdmin() bool {
	return os.Geteuid() == 0
}
