//go:build linux || darwin || freebsd || openbsd
// +build linux darwin freebsd openbsd

package main

func createVSSSnapshot(path string) string {
	return path
}

func VSSCleanup() {
}
