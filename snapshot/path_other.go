//go:build !windows && !linux
// +build !windows,!linux

package snapshot

// PathSupportsSnapshot is a no-op on platforms with no snapshot support of
// their own anyway (see nop_snapshot.go): nothing to gate.
func PathSupportsSnapshot(path string) bool {
	return true
}
