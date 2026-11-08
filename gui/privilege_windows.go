//go:build windows

package main

import (
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

var procAdjustTokenPrivileges = windows.NewLazySystemDLL("advapi32.dll").NewProc("AdjustTokenPrivileges")

// enablePrivileges switches on the named privileges in this process's token,
// for example SeRestorePrivilege (set any owner on a restored file) and
// SeTakeOwnershipPrivilege. An elevated administrator and SYSTEM hold both, but
// Windows keeps them disabled until asked. Returns an error naming any privilege
// the token does not hold; the others are still enabled.
func enablePrivileges(names ...string) error {
	var token windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_ADJUST_PRIVILEGES|windows.TOKEN_QUERY, &token); err != nil {
		return fmt.Errorf("OpenProcessToken: %w", err)
	}
	defer token.Close()

	var missing []string
	for _, name := range names {
		var luid windows.LUID
		if err := windows.LookupPrivilegeValue(nil, windows.StringToUTF16Ptr(name), &luid); err != nil {
			missing = append(missing, name)
			continue
		}
		tp := windows.Tokenprivileges{PrivilegeCount: 1}
		tp.Privileges[0] = windows.LUIDAndAttributes{Luid: luid, Attributes: windows.SE_PRIVILEGE_ENABLED}
		// Called directly, not through windows.AdjustTokenPrivileges: the call
		// succeeds with ERROR_NOT_ALL_ASSIGNED when the token does not hold the
		// privilege, and the x/sys wrapper drops the error code on success.
		r1, _, e1 := procAdjustTokenPrivileges.Call(uintptr(token), 0, uintptr(unsafe.Pointer(&tp)), uintptr(unsafe.Sizeof(tp)), 0, 0)
		if r1 == 0 || e1 == windows.ERROR_NOT_ALL_ASSIGNED {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("privileges not held: %v", missing)
	}
	return nil
}
