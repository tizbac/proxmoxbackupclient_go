//go:build !windows && !linux

package main

import "fmt"

// There is no request to make here: the backup runs, and the run's log says
// the computer was not kept awake.
func platformAcquireSleepLock(reason string) (func(), error) {
	return nil, fmt.Errorf("not supported on this system")
}
