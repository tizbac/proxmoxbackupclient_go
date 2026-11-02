//go:build !windows && !service

package main

// startAsAdmin: --admin is for the Windows installer's "(Administrator)"
// shortcuts. On Linux the administrator menu entry runs the app through
// proxmoxbackupclient-root (pkexec or sudo) instead, so here it does nothing.
func startAsAdmin() bool { return false }
