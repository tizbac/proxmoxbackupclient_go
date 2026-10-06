//go:build linux

package main

import "snapshot"

// getSnapshotModule detects which Linux block snapshot kernel module is available.
// Returns "elastio-snap", "dattobd", or empty string if neither is loaded.
func getSnapshotModule() string {
	if control, ok := snapshot.DetectControl(); ok {
		return control.Name
	}
	return ""
}
