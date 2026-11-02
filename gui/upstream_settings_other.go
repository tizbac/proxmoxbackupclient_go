//go:build !windows

package main

// importUpstreamUserSettings: on Linux the project's 1.2.0-rc app and this
// one use the same per-user folder (~/.proxmox-backup-guardian), so there is
// nothing to bring across. See upstream_settings_windows.go.
func importUpstreamUserSettings() {}
