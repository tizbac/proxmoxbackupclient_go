//go:build windows
// +build windows

package snapshot

import (
	"os"
	"testing"
)

// A network path, here this computer's own administrative share, cannot be
// shadow-copied. Skipped where the share is not there (it needs an
// administrator and the Server service).
func TestPathSupportsSnapshotNetworkShare(t *testing.T) {
	share := `\\localhost\C$\Windows`
	if _, err := os.Stat(share); err != nil {
		t.Skipf("%s not reachable: %v", share, err)
	}
	if PathSupportsSnapshot(share) {
		t.Errorf("%s: a network path should not be snapshotted", share)
	}
	if !PathSupportsSnapshot(`C:\Windows`) {
		t.Error(`C:\Windows: a local path should be snapshotted`)
	}
}
