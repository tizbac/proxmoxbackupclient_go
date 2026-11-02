//go:build !windows && !linux

package main

import "errors"

// elevatedFetchToken: no elevation helper on this system; an app run as the
// service's own account reads the token file directly (fetchServiceToken).
func elevatedFetchToken(string) (string, error) {
	return "", errors.New("connecting to the background service needs the app run as administrator on this system")
}
