//go:build windows

package main

import "golang.org/x/sys/windows"

// fileManagerLacksOurRights: would File Explorer, started from here, be
// without this process's administrator rights?
//
// With User Account Control an administrator's logon has two tokens, and
// Explorer runs with the limited one. A folder opened from an elevated app is
// shown by that same Explorer, which may not read a folder only administrators
// may read; it offers "Continue", which gives the user permanent full access
// to the folder. That is so exactly when this process is elevated and has a
// linked token (the limited half). The built-in Administrator, and an account
// with User Account Control off, have one token and Explorer shares it.
//
// A variable so a test can set it.
var fileManagerLacksOurRights = func() bool {
	token := windows.GetCurrentProcessToken()
	if !token.IsElevated() {
		return false
	}
	linked, err := token.GetLinkedToken()
	if err != nil {
		return false
	}
	_ = linked.Close()
	return true
}
