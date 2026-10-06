//go:build !linux

package main

// getSnapshotModule has nothing to detect off Linux: snapshot.DetectControl and
// the elastio-snap/dattobd kernel modules only exist in the Linux build.
func getSnapshotModule() string { return "" }
