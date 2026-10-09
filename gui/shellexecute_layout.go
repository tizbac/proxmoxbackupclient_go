package main

// shellExecInfo mirrors the Win32 SHELLEXECUTEINFOW structure (64-bit layout).
//
// It lives in a non-build-tagged file with an offset test next to it because
// getting this layout wrong is invisible: ShellExecuteExW simply fails with
// ERROR_INVALID_PARAMETER (87) and no UAC prompt ever appears, which reads like
// "elevation does nothing" rather than like a broken struct. Field ORDER is the
// Win32 order — cbSize, fMask, hwnd, lpVerb, lpFile, lpParameters, lpDirectory,
// nShow, hInstApp, lpIDList, lpClass, hkeyClass, dwHotKey, hMonitor, hProcess
// (SHELLEXECUTEINFOW is 112 bytes on x64).
type shellExecInfo struct {
	CbSize       uint32
	FMask        uint32
	Hwnd         uintptr
	LpVerb       *uint16
	LpFile       *uint16
	LpParameters *uint16
	LpDirectory  *uint16
	NShow        int32
	_            uint32 // C pads nShow so hInstApp stays 8-byte aligned
	HInstApp     uintptr
	LpIDList     uintptr
	LpClass      *uint16
	HkeyClass    uintptr
	DwHotKey     uint32
	_            uint32 // padding before the hIcon/hMonitor union
	HMonitor     uintptr
	HProcess     uintptr
}
