//go:build windows && !service

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/windows"
)

// startAsAdmin handles --admin, which the installer's "(Administrator)"
// shortcuts pass: Windows shortcuts made by the MSI cannot carry "Run as
// administrator" themselves. It reports whether this process should exit.
//
//   - Already elevated: nothing to do, start as usual.
//   - Another copy is running (usually the one in the tray, started at
//     logon): bring it to the front instead; its "Restart as administrator"
//     does the rest. Starting an elevated copy beside it would only find
//     the single-instance lock taken.
//   - Otherwise relaunch through the UAC prompt and exit. If the prompt is
//     declined, carry on without elevation rather than vanish.
func startAsAdmin() bool {
	if isAdmin() {
		return false
	}
	if anotherInstanceRunning() {
		writeDebugLog("--admin: another copy is running; activating it")
		activateExistingWindow()
		return true
	}
	exe, err := os.Executable()
	if err != nil {
		writeDebugLog(fmt.Sprintf("--admin: %v; starting without elevation", err))
		return false
	}
	verb, _ := syscall.UTF16PtrFromString("runas")
	file, _ := syscall.UTF16PtrFromString(exe)
	dir, _ := syscall.UTF16PtrFromString(filepath.Dir(exe))
	if err := windows.ShellExecute(0, verb, file, nil, dir, windows.SW_NORMAL); err != nil {
		if errors.Is(err, windows.ERROR_CANCELLED) {
			writeDebugLog("--admin: elevation declined; starting without it")
		} else {
			writeDebugLog(fmt.Sprintf("--admin: relaunch failed (%v); starting without elevation", err))
		}
		return false
	}
	return true
}

// anotherInstanceRunning reports whether the single-instance lock is held,
// without taking it.
func anotherInstanceRunning() bool {
	name, err := syscall.UTF16PtrFromString(mutexName)
	if err != nil {
		return false
	}
	h, err := windows.CreateMutex(nil, false, name)
	if h != 0 {
		windows.CloseHandle(h)
	}
	return mutexHeldByAnotherInstance(err)
}
