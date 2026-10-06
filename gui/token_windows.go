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

// elevatedFetchToken launches this same executable elevated (UAC, verb
// "runas") as the token-fetch child and waits for the handoff file. The
// installed Windows GUI runs with requireAdministrator and reads the token
// file directly; this path covers non-elevated launches (dev builds,
// manually extracted binaries) — the child is the same GUI exe, which
// handles --elevated-token-fetch before starting any UI.
func elevatedFetchToken(handoffFile string) (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("cannot resolve own executable: %w", err)
	}

	file, _ := windows.UTF16PtrFromString(exe)
	params, _ := windows.UTF16PtrFromString(`--elevated-token-fetch "` + handoffFile + `"`)
	verb, _ := windows.UTF16PtrFromString("runas")

	var sei shellExecInfo
	sei.CbSize = uint32(unsafe.Sizeof(sei))
	sei.FMask = seeMaskNoCloseProcess
	sei.LpFile = file
	sei.LpParameters = params
	sei.LpVerb = verb
	sei.NShow = swShownormal

	ret, _, callErr := shellExecuteExProc.Call(uintptr(unsafe.Pointer(&sei)))
	if sei.HProcess != 0 {
		_ = windows.CloseHandle(windows.Handle(sei.HProcess))
	}
	if ret == 0 {
		if callErr == windows.ERROR_CANCELLED { // 1223: user declined UAC
			return "", ErrElevationDeclined
		}
		return "", fmt.Errorf("ShellExecuteEx(runas) failed: %v", callErr)
	}

	writeDebugLog("[ElevatedTokenFetch] launched elevated child, waiting for handoff")
	return waitForHandoff(handoffFile, 60*time.Second)
}
