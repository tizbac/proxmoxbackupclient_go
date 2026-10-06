//go:build !windows && !linux
// +build !windows,!linux

package main

import "errors"

// elevatedFetchToken is unavailable on unsupported platforms: the GUI falls
// back to standalone mode instead of prompting.
func elevatedFetchToken(handoffFile string) (string, error) {
	return "", errors.New("elevated token fetch is not supported on this platform")
}
