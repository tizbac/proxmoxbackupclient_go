//go:build !windows && !service

package main

// startAsAdmin: --admin starts the app elevated on Windows. On Linux the
// app's Restart as administrator runs it through salvanza-gui-root
// (pkexec or sudo) instead, so here it does nothing.
func startAsAdmin() bool { return false }
