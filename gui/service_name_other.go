//go:build !windows
// +build !windows

package main

// serviceIdentity returns the (SCM/systemd name, display name) pair the service
// registers under on non-Windows platforms. The systemd unit file
// (packaging/systemd/pbsgo.service) names and starts the binary explicitly, so
// this identity is only used by kardianos/service install/uninstall actions.
func serviceIdentity() (name, displayName string) {
	return "ProxmoxBackupClient", "Proxmox Backup Client SVC"
}
