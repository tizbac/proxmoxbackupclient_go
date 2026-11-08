//go:build !windows

package main

import "errors"

// Off Windows nothing is sealed: the settings folder's 0600/0640 permissions
// are the protection, as before.
const canSealSecrets = false

func sealPlatform(s string) (string, error) { return s, nil }

func openPlatform(s string) (string, error) {
	return "", errors.New("this value was sealed on Windows and cannot be read here")
}
