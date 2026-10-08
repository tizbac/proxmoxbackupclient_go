//go:build windows
// +build windows

package main

import (
	"os"
	"path/filepath"
	"strings"
)

// serviceExecutableBase returns the base name of the running executable
// without its extension ("ProxmoxBackupClientSVC", "EtitechBackupSVC", ...).
func serviceExecutableBase() string {
	exePath, err := os.Executable()
	if err != nil {
		return defaultBrandKey
	}
	base := filepath.Base(exePath)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// windowsServiceName returns the SCM service name the WiX installer registered
// for this build (see serviceIdentityForExeBase for the contract with
// installer/wix).
func windowsServiceName() string {
	name, _ := serviceIdentityForExeBase(serviceExecutableBase())
	return name
}

// windowsServiceDisplayName returns the display name matching the installer's
// DisplayName="$(var.ProductName) Service".
func windowsServiceDisplayName() string {
	_, displayName := serviceIdentityForExeBase(serviceExecutableBase())
	return displayName
}

// serviceIdentity returns the (SCM name, display name) pair the service must
// register under on this platform.
func serviceIdentity() (name, displayName string) {
	return serviceIdentityForExeBase(serviceExecutableBase())
}
