//go:build !windows && !linux
// +build !windows,!linux

package main

// pathSupportsSnapshot is a no-op on platforms with no snapshot support of
// their own anyway (see snapshot/nop_snapshot.go) — nothing to gate.
func pathSupportsSnapshot(path string) bool {
	return true
}
