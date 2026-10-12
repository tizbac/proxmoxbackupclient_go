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
	user32            = windows.NewLazySystemDLL("user32.dll")
	procFindWindow    = user32.NewProc("FindWindowW")
	procSetForeground = user32.NewProc("SetForegroundWindow")
	procShowWindow    = user32.NewProc("ShowWindow")
	procIsIconic      = user32.NewProc("IsIconic")
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

	mutex, err := windows.CreateMutex(nil, false, mutexNamePtr)

	// Does the named mutex already exist (i.e. is another instance running)?
	// Three cases end up here:
	//
	//   - success + ERROR_ALREADY_EXISTS: same integrity level, the normal case.
	//   - ERROR_ALREADY_EXISTS as the error itself: belt and braces for the
	//     same situation.
	//   - ERROR_ACCESS_DENIED: the mutex exists but was created at a HIGHER
	//     integrity level — an elevated relaunch leaves exactly that behind
	//     (requestElevationWindows hands the lock to the UAC-elevated copy).
	//     Mandatory integrity control refuses a medium-IL process any access,
	//     so CreateMutex fails with ACCESS_DENIED instead of reporting
	//     ALREADY_EXISTS. The old code treated any other error as "no other
	//     instance, go ahead" — which is why a second GUI window kept
	//     appearing next to the elevated one. Being locked out of the name
	//     means someone else holds it; that IS a duplicate.
	duplicate := false
	switch {
	case err == nil:
		duplicate = windows.GetLastError() == windows.ERROR_ALREADY_EXISTS
	case errors.Is(err, windows.ERROR_ALREADY_EXISTS):
		duplicate = true
	case errors.Is(err, windows.ERROR_ACCESS_DENIED):
		writeDebugLog(fmt.Sprintf("Mutex %s exists at a higher integrity level (elevated instance): %v", mutexName, err))
		duplicate = true
	default:
		writeDebugLog(fmt.Sprintf("Failed to create mutex: %v", err))
		return true // Allow launch on error
	}

	if duplicate {
		writeDebugLog("Another instance is already running - attempting to bring it to foreground")

		// Try to find and activate the existing window
		if activateExistingWindow() {
			writeDebugLog("Successfully activated existing window")
		} else {
			writeDebugLog("Could not find existing window to activate")
		}

		// Close our mutex handle and exit (the handle is 0 when CreateMutex
		// failed outright, e.g. the ACCESS_DENIED case above).
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

// activateExistingWindow finds the existing client window and brings it to
// foreground.
func activateExistingWindow() bool {
	// The Wails window title is "<Brand> v<version>" (main.go's options), and
	// the brand is derived from the executable name — "Proxmox Backup Client",
	// "Nimbus Backup", "Acme Backup", "Etitech Backup". Searching only for the
	// hardcoded windowName meant a branded build never matched its own window:
	// the second launch exited without showing anything. Try the brand first,
	// then the default brand as a fallback.
	brand := BrandFromExecutable().Title
	titles := []string{
		brand + " v" + appVersion,
		brand,
	}
	if windowName != brand {
		titles = append(titles, windowName+" v"+appVersion, windowName)
	}

	var hwnd uintptr
	for _, title := range titles {
		titlePtr, err := syscall.UTF16PtrFromString(title)
		if err != nil {
			continue
		}
		hwnd, _, _ = procFindWindow.Call(
			0, // lpClassName - null to search all classes
			uintptr(unsafe.Pointer(titlePtr)),
		)
		if hwnd != 0 {
			break
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
