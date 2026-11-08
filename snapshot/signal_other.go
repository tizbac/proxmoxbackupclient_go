//go:build !linux

package snapshot

// CleanupOnSignal does nothing here: a Windows shadow copy of a backup
// context goes with the process that made it, and other systems take no
// snapshots.
func CleanupOnSignal() {}
