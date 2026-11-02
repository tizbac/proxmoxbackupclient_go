//go:build windows && !service

package main

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	procIsWindowVisible          = user32.NewProc("IsWindowVisible")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
)

func init() { mainWindowVisible = ownMainWindowVisible }

// ownMainWindowVisible reports whether this process's main window is on
// screen (a minimised window counts: it is in the taskbar, not the tray). A
// second launch shows it with ShowWindow (single_instance_windows.go), which
// the app is not told of; before it drops the token, token_drop.go asks
// here. The window is found by its title, as a second launch finds it.
func ownMainWindowVisible(*App) bool {
	brand := BrandFromExecutable().Title
	pid := uint32(os.Getpid()) // #nosec G115 -- a Windows process ID is a DWORD
	for _, title := range []string{brand + " " + versionLabel(), brand} {
		titlePtr, err := syscall.UTF16PtrFromString(title)
		if err != nil {
			continue
		}
		hwnd, _, _ := procFindWindow.Call(0, uintptr(unsafe.Pointer(titlePtr)))
		if hwnd == 0 {
			continue
		}
		var owner uint32
		procGetWindowThreadProcessId.Call(hwnd, uintptr(unsafe.Pointer(&owner)))
		if owner != pid {
			continue
		}
		visible, _, _ := procIsWindowVisible.Call(hwnd)
		return visible != 0
	}
	return false
}
