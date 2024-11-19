// +build linux darwin freebsd openbsd
package main

type Locking struct {
	mutexid uintptr
}

func (l *Locking) AcquireProcessLock() bool {
	return true
}

func (l * Locking) ReleaseProcessLock() {
	
}