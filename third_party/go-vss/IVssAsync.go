//go:build windows
// +build windows

package vss

import (
	"syscall"
	"unsafe"

	ole "github.com/go-ole/go-ole"
)

// NOTE: Microsoft Documentation can be found here: https://docs.microsoft.com/en-us/windows/win32/api/vss/nn-vss-ivssasync
// Limited Implementation of IVssAsync Interface. Allows to wait for an asychronous VSS method and query its status to either cancel or keep waiting.
type IVssAsync struct {
	ole.IUnknown
}

// VTable for IVssAsync
type IVssAsyncVTable struct {
	ole.IUnknownVtbl
	cancel      uintptr
	wait        uintptr
	queryStatus uintptr
}

// Returns pointer to IVssAsyncVTable
func (vssAsync *IVssAsync) getVTable() *IVssAsyncVTable {
	return (*IVssAsyncVTable)(unsafe.Pointer(vssAsync.RawVTable))
}

// Will wait for a method the given amount of seconds before throwing an timeout error.
// If the method completes before the timeout nil will be returned.
func (async *IVssAsync) Wait(miliseconds int) error {
	code, _, _ := syscall.Syscall(async.getVTable().wait, 2, uintptr(unsafe.Pointer(async)), uintptr(miliseconds), 0)
	if err := CreateVSSError("IVssAsync.Wait", code); err != nil {
		return err
	}

	return nil
}

func (async *IVssAsync) QueryStatus() (HRESULT, error) {
	var status int32
	code, _, _ := syscall.Syscall(async.getVTable().queryStatus, 3, uintptr(unsafe.Pointer(async)), uintptr(unsafe.Pointer(&status)), 0)
	if err := CreateVSSError("IVssAsync.QueryStatus", code); err != nil {
		return 0, err
	}

	return HRESULT(status), nil
}

func (async *IVssAsync) Cancel() error {
	code, _, _ := syscall.Syscall(async.getVTable().cancel, 1, uintptr(unsafe.Pointer(async)), 0, 0)
	return CreateVSSError("IVssAsync.cancel", code)
}
