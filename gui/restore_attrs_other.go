//go:build !windows

package main

// Windows file attributes have no meaning here.
func applyFileAttributes(path string, attrs uint32) {}

func clearReadOnly(path string) {}
