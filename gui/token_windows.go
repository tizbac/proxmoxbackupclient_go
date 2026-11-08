//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// From upstream (token_windows.go).

const (
	// seeMaskNoCloseProcess makes ShellExecuteEx hand back the started
	// process in HProcess.
	seeMaskNoCloseProcess = 0x00000040
	swShownormal          = 1
)

var (
	shell32DLL         = windows.NewLazySystemDLL("shell32.dll")
	shellExecuteExProc = shell32DLL.NewProc("ShellExecuteExW")
)

// shellExecuteRunas starts exe with params under the UAC "runas" verb and
// returns the elevated process (0 when the shell hands none back). A
// declined prompt is ErrElevationDeclined. shellExecInfo's layout is pinned
// by TestShellExecInfoLayout: a wrong size fails the call before any prompt.
func shellExecuteRunas(exe, params string) (windows.Handle, error) {
	file, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		return 0, fmt.Errorf("cannot encode executable path: %w", err)
	}
	verb, err := windows.UTF16PtrFromString("runas")
	if err != nil {
		return 0, fmt.Errorf("cannot encode verb: %w", err)
	}
	var p *uint16
	if params != "" {
		if p, err = windows.UTF16PtrFromString(params); err != nil {
			return 0, fmt.Errorf("cannot encode parameters: %w", err)
		}
	}

	var sei shellExecInfo
	sei.CbSize = uint32(unsafe.Sizeof(sei))
	sei.FMask = seeMaskNoCloseProcess
	sei.LpFile = file
	sei.LpParameters = p
	sei.LpVerb = verb
	sei.NShow = swShownormal

	ret, _, callErr := shellExecuteExProc.Call(uintptr(unsafe.Pointer(&sei)))
	if ret == 0 {
		if errors.Is(callErr, windows.ERROR_CANCELLED) { // 1223: UAC declined
			return 0, ErrElevationDeclined
		}
		if callErr != nil {
			return 0, fmt.Errorf("ShellExecuteEx(runas) failed: %w", callErr)
		}
		return 0, errors.New("ShellExecuteEx(runas) failed without an error code")
	}
	return windows.Handle(sei.HProcess), nil
}

// runElevatedHandoff starts this program elevated (UAC) with flag and the
// handoff file, waits for it (timeout, then as long again polling the file),
// and returns the token it wrote into the file: the token-fetch copy
// (elevatedFetchToken), or the copy that starts the service first
// (service_start.go).
func runElevatedHandoff(flag, handoffFile string, timeout time.Duration) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("cannot resolve own executable: %w", err)
	}

	h, err := shellExecuteRunas(exe, flag+` "`+handoffFile+`"`)
	if err != nil {
		return "", err
	}
	if h == 0 {
		return waitForHandoff(handoffFile, timeout)
	}
	defer func() { _ = windows.CloseHandle(h) }()

	// Wait for the copy rather than polling the whole time: a failure shows
	// at once, with its reason.
	ev, werr := windows.WaitForSingleObject(h, uint32(timeout/time.Millisecond))
	if werr != nil || ev != windows.WAIT_OBJECT_0 {
		writeDebugLog(fmt.Sprintf("[ElevatedTokenFetch] %s: child still running after %s (wait error %v, event %d); polling the handoff file",
			flag, timeout, werr, ev))
		return waitForHandoff(handoffFile, timeout)
	}

	var code uint32
	haveCode := false
	if err := windows.GetExitCodeProcess(h, &code); err == nil {
		haveCode = true
		writeDebugLog(fmt.Sprintf("[ElevatedTokenFetch] child exited with code %d", code))
	}
	if token, err := readHandoff(handoffFile); err == nil {
		return token, nil
	}
	if haveCode && code != 0 {
		return "", &elevatedChildError{code: code}
	}
	return "", errors.New("the elevated process exited without writing the token")
}
