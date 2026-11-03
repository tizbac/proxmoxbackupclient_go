//go:build linux || darwin || freebsd || openbsd || solaris || netbsd
// +build linux darwin freebsd openbsd solaris netbsd

package clientcommon

// Locking takes no process lock outside Windows; see win_locking.go.
type Locking struct{}

func (l *Locking) AcquireProcessLock() bool {
	return true
}

func (l *Locking) ReleaseProcessLock() {

}
