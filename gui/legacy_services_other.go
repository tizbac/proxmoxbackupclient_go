//go:build !(windows && service)

package main

// removeOldServices has nothing to do outside the Windows service: the Linux
// packages replace the earlier ones themselves (Conflicts, Replaces).
func removeOldServices() {}
