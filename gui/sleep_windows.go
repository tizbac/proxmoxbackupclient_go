//go:build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// A power request belongs to the process, not to a thread (SetThreadExecutionState
// would need a thread of its own for the whole run), and shows up in
// "powercfg /requests" with its reason. SystemRequired keeps the system from
// sleeping; ExecutionRequired asks that the process keeps running, which is what
// Modern Standby needs.
var (
	kernel32Power          = windows.NewLazySystemDLL("kernel32.dll")
	procPowerCreateRequest = kernel32Power.NewProc("PowerCreateRequest")
	procPowerSetRequest    = kernel32Power.NewProc("PowerSetRequest")
	procPowerClearRequest  = kernel32Power.NewProc("PowerClearRequest")
)

const (
	powerRequestSystemRequired    = 1
	powerRequestExecutionRequired = 3
	powerRequestContextSimple     = 0x1 // POWER_REQUEST_CONTEXT_SIMPLE_STRING
)

// reasonContext is REASON_CONTEXT with the simple reason string; the union's
// remaining bytes are padding.
type reasonContext struct {
	Version            uint32
	Flags              uint32
	SimpleReasonString *uint16
	_                  [24]byte
}

func platformAcquireSleepLock(reason string) (func(), error) {
	text, err := syscall.UTF16PtrFromString(reason)
	if err != nil {
		return nil, err
	}
	ctx := reasonContext{Flags: powerRequestContextSimple, SimpleReasonString: text}
	h, _, callErr := procPowerCreateRequest.Call(uintptr(unsafe.Pointer(&ctx)))
	if h == 0 || h == uintptr(windows.InvalidHandle) {
		return nil, fmt.Errorf("PowerCreateRequest: %v", callErr)
	}
	handle := windows.Handle(h)
	set := []uintptr{}
	for _, kind := range []uintptr{powerRequestSystemRequired, powerRequestExecutionRequired} {
		ok, _, setErr := procPowerSetRequest.Call(h, kind)
		if ok == 0 {
			if kind == powerRequestSystemRequired { // the one that matters
				_ = windows.CloseHandle(handle)
				return nil, fmt.Errorf("PowerSetRequest: %v", setErr)
			}
			continue // ExecutionRequired is not offered by every Windows version
		}
		set = append(set, kind)
	}
	return func() {
		for _, kind := range set {
			_, _, _ = procPowerClearRequest.Call(h, kind)
		}
		_ = windows.CloseHandle(handle)
	}, nil
}
