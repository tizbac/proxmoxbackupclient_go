//go:build windows
// +build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	// seeMaskNoCloseProcess asks ShellExecuteEx to hand back a handle to the
	// started process in HProcess (0 when the call fails or the verb does not
	// start a process).
	seeMaskNoCloseProcess = 0x00000040
	// swShownormal is SW_SHOWNORMAL: show the new process' window normally.
	swShownormal = 1
)

// ErrElevationDeclined is returned when the user dismisses the UAC prompt.
var ErrElevationDeclined = errors.New("UAC elevation prompt was declined")

var (
	shell32DLL         = windows.NewLazySystemDLL("shell32.dll")
	shellExecuteExProc = shell32DLL.NewProc("ShellExecuteExW")
)

// shellExecuteRunas starts exe with the given command line under the UAC
// "runas" verb and returns a handle to the elevated process (0 when the shell
// did not hand one back), or an error. Declining the prompt surfaces as
// ErrElevationDeclined; a missing/failed elevation mechanism (UAC disabled, no
// shell) as a plain error — callers turn both into "run standalone / give up",
// never a hang.
//
// The shellExecInfo layout (and therefore this function) is pinned by
// TestShellExecInfoLayout: a wrong cbSize makes ShellExecuteExW fail with
// ERROR_INVALID_PARAMETER before it even looks at the verb, so no UAC prompt
// ever appears.
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
		if errors.Is(callErr, windows.ERROR_CANCELLED) { // 1223: user declined UAC
			return 0, ErrElevationDeclined
		}
		if callErr != nil {
			return 0, fmt.Errorf("ShellExecuteEx(runas) failed: %w", callErr)
		}
		return 0, errors.New("ShellExecuteEx(runas) failed without an error code")
	}
	return windows.Handle(sei.HProcess), nil
}

// elevatedFetchToken launches this same executable elevated (UAC, verb
// "runas") as the token-fetch child and waits for the handoff file. The GUI
// runs asInvoker now, so this is the normal launch-time path whenever the
// service is up and the token file is not ours to read — the child is the
// same GUI exe, which handles --elevated-token-fetch before starting any UI.
func elevatedFetchToken(handoffFile string) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("cannot resolve own executable: %w", err)
	}

	h, err := shellExecuteRunas(exe, `--elevated-token-fetch "`+handoffFile+`"`)
	if err != nil {
		return "", err
	}
	if h == 0 {
		return waitForHandoff(handoffFile, elevatedChildTimeout)
	}
	defer func() { _ = windows.CloseHandle(h) }()

	// Wait for the child instead of polling for the full timeout. It only
	// reads the token file and exits, so a failure (token unreadable even
	// elevated, handoff not writable from the elevated context, ...) shows up
	// here immediately, WITH its real reason, instead of costing the caller a
	// silent minute and then reporting a bare timeout.
	ev, werr := windows.WaitForSingleObject(h, uint32(elevatedChildTimeout/time.Millisecond))
	if werr != nil || ev != windows.WAIT_OBJECT_0 {
		writeDebugLog(fmt.Sprintf("[ElevatedTokenFetch] child still running after %s (wait error %v, event %d); polling the handoff file",
			elevatedChildTimeout, werr, ev))
		return waitForHandoff(handoffFile, elevatedChildTimeout)
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
		return "", fmt.Errorf("elevated token fetch failed: child exited with code %s — %s",
			elevatedChildExitLabel(code), elevatedChildExitHint(code))
	}
	return "", errors.New("elevated token fetch failed: the child exited without writing the token")
}
