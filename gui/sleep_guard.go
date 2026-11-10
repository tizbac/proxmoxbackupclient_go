package main

import (
	"fmt"
	"sync"
)

// A backup that runs while the computer goes to sleep stalls: the network drops
// (Modern Standby cuts it), the PBS session is lost, and the run starts again
// hours later, or hangs at the snapshot step until it is stopped. So a run asks
// the system not to sleep until it ends, whether it succeeds, fails or is
// stopped. Several runs at once share one request: the system may sleep again
// when the last of them ends.
//
// The platform files (sleep_windows.go, sleep_linux.go, sleep_darwin.go,
// sleep_other.go) make the request.

// acquireSleepLock asks the system not to sleep and returns what undoes it. It
// is a variable so the tests can stand in for the system.
var acquireSleepLock = platformAcquireSleepLock

var sleepGuard struct {
	mu      sync.Mutex
	holders int
	release func()
}

// preventSleep keeps the computer awake until the returned function is called
// (safe to call more than once). When the system will not do it, the backup
// still runs: err says why, for the run's log, and the returned function is
// still safe to call.
func preventSleep(reason string) (end func(), err error) {
	sleepGuard.mu.Lock()
	defer sleepGuard.mu.Unlock()
	if sleepGuard.release == nil {
		rel, aerr := acquireSleepLock(reason)
		if aerr != nil {
			err = fmt.Errorf("could not keep the computer awake: %w", aerr)
		} else {
			sleepGuard.release = rel
		}
	}
	sleepGuard.holders++
	var once sync.Once
	return func() {
		once.Do(func() {
			sleepGuard.mu.Lock()
			defer sleepGuard.mu.Unlock()
			sleepGuard.holders--
			if sleepGuard.holders == 0 && sleepGuard.release != nil {
				sleepGuard.release()
				sleepGuard.release = nil
			}
		})
	}, err
}

// sleepReason is what the system shows for the request (powercfg /requests,
// systemd-inhibit --list).
func sleepReason() string {
	return product().Title + ": a backup is running"
}
