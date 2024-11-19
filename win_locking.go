//go:build windows
// +build windows
package main

import "github.com/rodolfoag/gow32"
import "syscall"

const MutexName = "proxmoxbackupclient_go"


type Locking struct {
	mutexid uintptr
}

func (l *Locking) AcquireProcessLock() bool {
	mutexid , err := gow32.CreateMutex(MutexName)
	if err != nil {
		if exitcode := int(err.(syscall.Errno)); exitcode == gow32.ERROR_ALREADY_EXISTS {
			return false 
		}
		panic(err)
	}
	l.mutexid = mutexid
	return true
}

func (l * Locking) ReleaseProcessLock() {
	gow32.ReleaseMutex(l.mutexid)
}