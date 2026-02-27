//go:build linux || darwin || freebsd || openbsd || solaris || netbsd
// +build linux darwin freebsd openbsd solaris netbsd

package clientcommon

type Locking struct {
	mutexid uintptr
}

func (l *Locking) AcquireProcessLock() bool {
	return true
}

func (l *Locking) ReleaseProcessLock() {

}
