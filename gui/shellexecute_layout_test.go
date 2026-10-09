package main

import (
	"testing"
	"unsafe"
)

// The struct is passed straight to ShellExecuteExW, so its layout must match
// SHELLEXECUTEINFOW exactly: cbSize is validated first, so a wrong size makes
// the call fail with ERROR_INVALID_PARAMETER and no UAC prompt — the failure
// mode reported as "elevation asks nothing at all".
func TestShellExecInfoLayout(t *testing.T) {
	if unsafe.Sizeof(uintptr(0)) != 8 {
		t.Skip("layout assertions cover the 64-bit build")
	}

	type field struct {
		name   string
		offset uintptr
	}
	want := []field{
		{"CbSize", 0},
		{"FMask", 4},
		{"Hwnd", 8},
		{"LpVerb", 16},
		{"LpFile", 24},
		{"LpParameters", 32},
		{"LpDirectory", 40},
		{"NShow", 48},
		{"HInstApp", 56},
		{"LpIDList", 64},
		{"LpClass", 72},
		{"HkeyClass", 80},
		{"DwHotKey", 88},
		{"HMonitor", 96},
		{"HProcess", 104},
	}
	got := map[string]uintptr{
		"CbSize":       unsafe.Offsetof(shellExecInfo{}.CbSize),
		"FMask":        unsafe.Offsetof(shellExecInfo{}.FMask),
		"Hwnd":         unsafe.Offsetof(shellExecInfo{}.Hwnd),
		"LpVerb":       unsafe.Offsetof(shellExecInfo{}.LpVerb),
		"LpFile":       unsafe.Offsetof(shellExecInfo{}.LpFile),
		"LpParameters": unsafe.Offsetof(shellExecInfo{}.LpParameters),
		"LpDirectory":  unsafe.Offsetof(shellExecInfo{}.LpDirectory),
		"NShow":        unsafe.Offsetof(shellExecInfo{}.NShow),
		"HInstApp":     unsafe.Offsetof(shellExecInfo{}.HInstApp),
		"LpIDList":     unsafe.Offsetof(shellExecInfo{}.LpIDList),
		"LpClass":      unsafe.Offsetof(shellExecInfo{}.LpClass),
		"HkeyClass":    unsafe.Offsetof(shellExecInfo{}.HkeyClass),
		"DwHotKey":     unsafe.Offsetof(shellExecInfo{}.DwHotKey),
		"HMonitor":     unsafe.Offsetof(shellExecInfo{}.HMonitor),
		"HProcess":     unsafe.Offsetof(shellExecInfo{}.HProcess),
	}
	for _, f := range want {
		if got[f.name] != f.offset {
			t.Errorf("shellExecInfo.%s at offset %d, SHELLEXECUTEINFOW has it at %d",
				f.name, got[f.name], f.offset)
		}
	}

	// SHELLEXECUTEINFOW is 112 bytes on x64; ShellExecuteExW rejects anything
	// else before looking at a single field.
	if size := unsafe.Sizeof(shellExecInfo{}); size != 112 {
		t.Errorf("sizeof(shellExecInfo) = %d, want 112 (SHELLEXECUTEINFOW on x64)", size)
	}
}
