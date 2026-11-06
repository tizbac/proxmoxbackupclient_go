//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Connecting to a share with a login: what net use \\server\share does,
// through mpr.dll. The connection belongs to this program's logon session
// (the service's, SYSTEM's) and has no drive letter; it is not remembered
// across restarts.

var (
	mpr                       = windows.NewLazySystemDLL("mpr.dll")
	procWNetAddConnection2W   = mpr.NewProc("WNetAddConnection2W")
	procWNetCancelConnection2 = mpr.NewProc("WNetCancelConnection2W")
	procWNetGetConnectionW    = mpr.NewProc("WNetGetConnectionW")
)

// netResource is NETRESOURCEW.
type netResource struct {
	Scope       uint32
	Type        uint32
	DisplayType uint32
	Usage       uint32
	LocalName   *uint16
	RemoteName  *uint16
	Comment     *uint16
	Provider    *uint16
}

const (
	resourcetypeDisk               = 1
	errorSessionCredentialConflict = syscall.Errno(1219)
	errorNotConnected              = syscall.Errno(2250)
)

func connectShare(root, username, password string) error {
	remote, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return err
	}
	user, err := windows.UTF16PtrFromString(username)
	if err != nil {
		return err
	}
	pass, err := windows.UTF16PtrFromString(password)
	if err != nil {
		return err
	}
	nr := netResource{Type: resourcetypeDisk, RemoteName: remote}
	add := func() error {
		r, _, _ := procWNetAddConnection2W.Call(uintptr(unsafe.Pointer(&nr)), uintptr(unsafe.Pointer(pass)), uintptr(unsafe.Pointer(user)), 0)
		if r != 0 {
			return syscall.Errno(r)
		}
		return nil
	}
	err = add()
	// This session is already connected to the server under another user
	// name (a run of an older version that did not let go, or one ended
	// abruptly): Windows takes one per server, so drop that one and connect
	// again.
	if errors.Is(err, errorSessionCredentialConflict) {
		disconnectServer(root)
		err = add()
	}
	if err != nil {
		return describeShareError(err)
	}
	return nil
}

func disconnectShare(root string) {
	remote, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return
	}
	// fForce: a file left open does not keep the connection.
	r, _, _ := procWNetCancelConnection2.Call(uintptr(unsafe.Pointer(remote)), 0, 1)
	if r != 0 && syscall.Errno(r) != errorNotConnected {
		writeDebugLog(fmt.Sprintf("Network share: letting go of %s: %v", root, syscall.Errno(r)))
	}
}

// disconnectServer drops this session's connections to root's server: the
// share itself and the server's IPC$, which hold the user name Windows
// compares a new one with.
func disconnectServer(root string) {
	disconnectShare(root)
	if server, _, ok := splitShare(root); ok {
		disconnectShare(`\\` + server + `\IPC$`)
	}
}

// isLoginRefusal reports a refusal that a (different) login could cure: access
// denied, a wrong user name or password, an account not allowed in.
func isLoginRefusal(err error) bool {
	if errors.Is(err, os.ErrPermission) {
		return true
	}
	for _, e := range []syscall.Errno{
		windows.ERROR_ACCESS_DENIED,
		windows.ERROR_LOGON_FAILURE,
		windows.ERROR_INVALID_PASSWORD,
		windows.ERROR_ACCOUNT_RESTRICTION,
		windows.ERROR_ACCOUNT_DISABLED,
		windows.ERROR_ACCOUNT_EXPIRED,
		windows.ERROR_PASSWORD_EXPIRED,
		windows.ERROR_LOGON_TYPE_NOT_GRANTED,
		windows.ERROR_BAD_USERNAME,
		errorSessionCredentialConflict,
	} {
		if errors.Is(err, e) {
			return true
		}
	}
	return false
}

// describeShareError words the usual failures to connect, keeping the error
// for errors.Is.
func describeShareError(err error) error {
	switch {
	case errors.Is(err, windows.ERROR_LOGON_FAILURE), errors.Is(err, windows.ERROR_INVALID_PASSWORD):
		return fmt.Errorf("the user name or password is wrong: %w", err)
	case errors.Is(err, windows.ERROR_BAD_NETPATH), errors.Is(err, windows.ERROR_BAD_NET_NAME):
		return fmt.Errorf("the server or share cannot be found: %w", err)
	case errors.Is(err, windows.ERROR_ACCESS_DENIED):
		return fmt.Errorf("access is denied to this user: %w", err)
	}
	return err
}

// resolveMappedDrive turns a path on a drive letter mapped to a share (Z:\x)
// into the share's own path (\\server\share\x). The background service does
// not see the drive letters a user maps, so a Backup Set keeps the share's
// path. Anything else is returned unchanged. It must run in the user's
// program (the app), where the letter is mapped.
func resolveMappedDrive(path string) string {
	if len(path) < 2 || path[1] != ':' {
		return path
	}
	drive := strings.ToUpper(path[:2])
	root, err := windows.UTF16PtrFromString(drive + `\`)
	if err != nil || windows.GetDriveType(root) != windows.DRIVE_REMOTE {
		return path
	}
	local, err := windows.UTF16PtrFromString(drive)
	if err != nil {
		return path
	}
	buf := make([]uint16, 1024)
	size := uint32(len(buf))
	r, _, _ := procWNetGetConnectionW.Call(uintptr(unsafe.Pointer(local)), uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
	if r != 0 {
		return path
	}
	remote := strings.TrimRight(windows.UTF16ToString(buf), `\`)
	if _, _, ok := splitShare(remote); !ok {
		return path
	}
	rest := strings.TrimLeft(path[2:], `\/`)
	if rest == "" {
		return remote
	}
	return remote + `\` + rest
}
