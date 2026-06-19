//go:build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	mutexName  = "Global\\NimbusBackupGUIMutex"
	windowName = "Nimbus Backup"
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
	mutexNamePtr, err := syscall.UTF16PtrFromString(mutexName)
	if err != nil {
		writeDebugLog(fmt.Sprintf("Failed to create mutex name: %v", err))
		return true // Allow launch on error
	}

	// Try to create or open the mutex
	mutex, err := windows.CreateMutex(nil, false, mutexNamePtr)
	if err != nil && err != windows.ERROR_ALREADY_EXISTS {
		writeDebugLog(fmt.Sprintf("Failed to create mutex: %v", err))
		return true // Allow launch on error
	}

	// Check if mutex already existed (GetLastError returns ERROR_ALREADY_EXISTS even when CreateMutex succeeds)
	lastErr := windows.GetLastError()
	if lastErr == windows.ERROR_ALREADY_EXISTS {
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
	writeDebugLog("No other instance detected - continuing startup")
	return true
}

// activateExistingWindow finds the existing Nimbus Backup window and brings it to foreground
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
		// "Nimbus Backup v<appVersion>"). Derived from appVersion so it never
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
