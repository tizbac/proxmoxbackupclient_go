//go:build !windows && service
// +build !windows,service

package main

// Only the service build (-tags service) needs this identity: the unit file
// names and starts the binary explicitly. Compiled out otherwise so it is not
// dead code in the GUI build.
//
// serviceIdentity returns the (SCM/systemd name, display name) pair the service
// registers under on non-Windows platforms. The systemd unit file
// (packaging/systemd/pbsgo.service) names and starts the binary explicitly, so
// this identity is only used by kardianos/service install/uninstall actions.
func serviceIdentity() (name, displayName string) {
	return "ProxmoxBackupClient", "Proxmox Backup Client SVC"
}
