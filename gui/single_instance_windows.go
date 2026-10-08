//go:build windows

package main

import (
	"errors"
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	mutexName  = "Global\\ProxmoxBackupClientGUIMutex"
	windowName = "Proxmox Backup Client"
)

var (
	user32           = windows.NewLazySystemDLL("user32.dll")
	procFindWindow   = user32.NewProc("FindWindowW")
	procSetForeground = user32.NewProc("SetForegroundWindow")
	procShowWindow   = user32.NewProc("ShowWindow")
	procIsIconic     = user32.NewProc("IsIconic")
)

const (
	SW_RESTORE = 9
	SW_SHOW    = 5
)

// CheckSingleInstance checks if another instance is already running.
// Returns true if this is the only instance, false if another exists.
// If another exists, it attempts to bring that window to the foreground.
func CheckSingleInstance() bool {
	return checkSingleInstanceNamed(mutexName)
}

// mutexHeldByAnotherInstance reports whether a CreateMutex error means another
// instance already owns the lock. ACCESS_DENIED counts too: an instance running
// elevated creates the mutex with an administrators-only ACL, which a standard
// copy cannot open.
func mutexHeldByAnotherInstance(err error) bool {
	return errors.Is(err, windows.ERROR_ALREADY_EXISTS) || errors.Is(err, windows.ERROR_ACCESS_DENIED)
}

// checkSingleInstanceNamed is CheckSingleInstance for an arbitrary mutex name,
// so it can be tested without colliding with a running copy of the app.
func checkSingleInstanceNamed(name string) bool {
	mutexNamePtr, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		writeDebugLog(fmt.Sprintf("Failed to create mutex name: %v", err))
		return true // Allow launch on error
	}

	// Try to create or open the mutex
	mutex, err := windows.CreateMutex(nil, false, mutexNamePtr)
	if err != nil && !mutexHeldByAnotherInstance(err) {
		writeDebugLog(fmt.Sprintf("Failed to create mutex: %v", err))
		return true // Allow launch on error
	}

	// Use the error CreateMutex itself returned. Calling windows.GetLastError()
	// afterwards does not work: Go's syscall layer does not preserve the value
	// (it reads back 0), so every launch used to believe it was the first
	// instance and a second copy simply opened alongside the first.
	if mutexHeldByAnotherInstance(err) {
		writeDebugLog("Another instance is already running - attempting to bring it to foreground")

		// Try to find and activate the existing window
		if activateExistingWindow() {
			writeDebugLog("Successfully activated existing window")
		} else {
			writeDebugLog("Could not find existing window to activate")
		}

		// Close our mutex handle and exit
		if mutex != 0 {
			windows.CloseHandle(mutex)
		}
		return false
	}

	// We are the first instance - keep the mutex open
	// Don't close it - it will be released when the process exits
	instanceMutex = mutex
	writeDebugLog("No other instance detected - continuing startup")
	return true
}

// instanceMutex is the handle keeping this process the single instance;
// ReleaseSingleInstance closes it (used before an elevated relaunch).
var instanceMutex windows.Handle

// ReleaseSingleInstance closes the single-instance mutex so an elevated
// relaunch can acquire it instead of being seen as a duplicate. Safe to call
// when no instance lock is held.
func ReleaseSingleInstance() {
	if instanceMutex != 0 {
		windows.CloseHandle(instanceMutex)
		instanceMutex = 0
		writeDebugLog("Single-instance lock released for elevated relaunch")
	}
}

// activateExistingWindow finds the existing Proxmox Backup Client window and brings it to foreground
func activateExistingWindow() bool {
	windowNamePtr, err := syscall.UTF16PtrFromString(windowName)
	if err != nil {
		return false
	}

	// Find window by title (Wails uses the app title as window title)
	hwnd, _, _ := procFindWindow.Call(
		0, // lpClassName - null to search all classes
		uintptr(unsafe.Pointer(windowNamePtr)),
	)

	if hwnd == 0 {
		// Try with the current version suffix (Wails titles the window
		// "Proxmox Backup Client v<appVersion>"). Derived from appVersion so it never
		// goes stale across releases.
		for _, suffix := range []string{" v" + appVersion} {
			titleWithVersion := windowName + suffix
			titlePtr, _ := syscall.UTF16PtrFromString(titleWithVersion)
			hwnd, _, _ = procFindWindow.Call(
				0,
				uintptr(unsafe.Pointer(titlePtr)),
			)
			if hwnd != 0 {
				break
			}
		}
	}

	if hwnd == 0 {
		return false
	}

	// Check if window is minimized
	isMinimized, _, _ := procIsIconic.Call(hwnd)
	if isMinimized != 0 {
		// Restore the window if minimized
		procShowWindow.Call(hwnd, SW_RESTORE)
	} else {
		// Just show it if hidden
		procShowWindow.Call(hwnd, SW_SHOW)
	}

	// Bring window to foreground
	procSetForeground.Call(hwnd)

	return true
}
