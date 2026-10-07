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

// ErrElevationDeclined is returned when the user dismisses the UAC prompt.
var ErrElevationDeclined = errors.New("UAC elevation prompt was declined")

var (
	shell32DLL         = windows.NewLazySystemDLL("shell32.dll")
	shellExecuteExProc = shell32DLL.NewProc("ShellExecuteExW")
)

// shellExecInfo mirrors the Windows SHELL_EXECUTE_INFO structure
// (Unicode build, 64-bit layout).
type shellExecInfo struct {
	CbSize       uint32
	FMask        uint32
	LpFile       *uint16
	LpParameters *uint16
	LpDirectory  *uint16
	LpVerb       *uint16
	NShow        int32
	HInstApp     uintptr
	LpClass      *uint16
	HkeyHotKey   uintptr
	HIcon        uintptr
	DwReserved   uint32
	FMaskPoints  uint32
	HProcess     uintptr
}

const (
	seeMaskNoCloseProcess = 0x00000040
	swShownormal          = 1
)

// shellExecuteRunas starts exe with the given command line under the UAC
// "runas" verb and returns as soon as the elevated process exists. Declining
// the prompt surfaces as ErrElevationDeclined; a missing/failed elevation
// mechanism (UAC disabled, no shell) as a plain error — callers turn both into
// "run standalone / give up", never a hang.
func shellExecuteRunas(exe, params string) error {
	file, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		return fmt.Errorf("cannot encode executable path: %w", err)
	}
	verb, err := windows.UTF16PtrFromString("runas")
	if err != nil {
		return fmt.Errorf("cannot encode verb: %w", err)
	}

	var p *uint16
	if params != "" {
		if p, err = windows.UTF16PtrFromString(params); err != nil {
			return fmt.Errorf("cannot encode parameters: %w", err)
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
	if sei.HProcess != 0 {
		_ = windows.CloseHandle(windows.Handle(sei.HProcess))
	}
	if ret == 0 {
		if callErr == windows.ERROR_CANCELLED { // 1223: user declined UAC
			return ErrElevationDeclined
		}
		return fmt.Errorf("ShellExecuteEx(runas) failed: %v", callErr)
	}
	return nil
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

	if err := shellExecuteRunas(exe, `--elevated-token-fetch "`+handoffFile+`"`); err != nil {
		return "", err
	}

	writeDebugLog("[ElevatedTokenFetch] launched elevated child, waiting for handoff")
	return waitForHandoff(handoffFile, 60*time.Second)
}
