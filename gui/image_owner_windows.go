//go:build windows

package main

import "github.com/tizbac/proxmoxbackupclient_go/gui/imagefs"

// applyImageOwnerMode: Linux owners and modes have no meaning on a Windows
// restore.
func applyImageOwnerMode(dest string, e imagefs.Entry) {}
