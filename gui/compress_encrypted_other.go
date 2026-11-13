//go:build !windows

package main

// replacedCompressingClient: only the Windows installer replaces a client
// that already wrote compressed encrypted backups (compress_encrypted_windows.go);
// there is no such package for Linux or macOS.
func replacedCompressingClient() bool { return false }
