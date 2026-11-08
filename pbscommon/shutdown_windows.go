//go:build windows

package pbscommon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// ShutdownComputer turns the computer off after a backup that asks for it (a
// Backup Set's "Turn off the computer after the backup", the command line
// tools' -shutdown-after): through the Windows API, as asked for in
// tizbac/proxmoxbackupclient_go#43, with shutdown.exe as the fallback should
// the API call fail. comment goes in the System event log. It returns how the
// shutdown was requested, and, when it fails, why each way failed.
//
// InitiateSystemShutdownEx rather than ExitWindowsEx: a Backup Set usually
// runs in the background service, as SYSTEM in session 0, where ExitWindowsEx
// only ends the caller's own logon session ("if you are not the interactive
// user, the function can succeed without actually shutting down the
// computer"); InitiateSystemShutdownEx turns the computer off from any
// session. Like shutdown /s /t 0: no warning period, and programs are asked
// to close rather than forced. The reason code marks it as a planned shutdown
// by an application.
func ShutdownComputer(comment string) (method string, err error) {
	apiErr := apiShutdown(comment)
	if apiErr == nil {
		return "the Windows API", nil
	}
	if cmdErr := commandShutdown(); cmdErr != nil {
		return "", fmt.Errorf("the Windows API: %v; shutdown.exe: %w", apiErr, cmdErr)
	}
	return fmt.Sprintf("shutdown.exe (the Windows API failed: %v)", apiErr), nil
}

// The two ways ShutdownComputer turns the computer off; tests replace them.
var (
	apiShutdown     = shutdownViaAPI
	commandShutdown = shutdownViaCommand
)

// shutdownReason is a planned shutdown for application maintenance.
const shutdownReason = windows.SHTDN_REASON_MAJOR_APPLICATION | windows.SHTDN_REASON_MINOR_MAINTENANCE | windows.SHTDN_REASON_FLAG_PLANNED

var procAdjustTokenPrivileges = windows.NewLazySystemDLL("advapi32.dll").NewProc("AdjustTokenPrivileges")

// enableShutdownPrivilege switches on SeShutdownPrivilege, which SYSTEM and an
// interactive user hold but Windows keeps disabled until asked.
func enableShutdownPrivilege() error {
	var token windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_ADJUST_PRIVILEGES|windows.TOKEN_QUERY, &token); err != nil {
		return fmt.Errorf("OpenProcessToken: %w", err)
	}
	defer func() { _ = token.Close() }()
	var luid windows.LUID
	if err := windows.LookupPrivilegeValue(nil, windows.StringToUTF16Ptr("SeShutdownPrivilege"), &luid); err != nil {
		return fmt.Errorf("LookupPrivilegeValue: %w", err)
	}
	tp := windows.Tokenprivileges{PrivilegeCount: 1}
	tp.Privileges[0] = windows.LUIDAndAttributes{Luid: luid, Attributes: windows.SE_PRIVILEGE_ENABLED}
	// Called directly, not through windows.AdjustTokenPrivileges: the call
	// succeeds with ERROR_NOT_ALL_ASSIGNED when the token does not hold the
	// privilege, and the x/sys wrapper drops the error code on success.
	r1, _, e1 := procAdjustTokenPrivileges.Call(uintptr(token), 0, uintptr(unsafe.Pointer(&tp)), uintptr(unsafe.Sizeof(tp)), 0, 0)
	if r1 == 0 || e1 == windows.ERROR_NOT_ALL_ASSIGNED {
		return fmt.Errorf("this account does not hold SeShutdownPrivilege")
	}
	return nil
}

func shutdownViaAPI(comment string) error {
	if err := enableShutdownPrivilege(); err != nil {
		return err
	}
	msg, err := windows.UTF16PtrFromString(comment)
	if err != nil {
		return err
	}
	if err := windows.InitiateSystemShutdownEx(nil, msg, 0, false, false, shutdownReason); err != nil {
		return fmt.Errorf("InitiateSystemShutdownEx: %w", err)
	}
	return nil
}

// shutdownViaCommand runs shutdown /s /t 0, from System32 by its full path.
func shutdownViaCommand() error {
	exe := "shutdown"
	if root := os.Getenv("SystemRoot"); root != "" {
		exe = filepath.Join(root, "System32", "shutdown.exe")
	}
	if out, err := exec.Command(exe, "/s", "/t", "0").CombinedOutput(); err != nil {
		return fmt.Errorf("%v: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
