//go:build windows

package machinebackuplib

import (
	"syscall"
	"testing"
)

// On Windows only disk errors count as bad sectors; anything else (here
// ERROR_INVALID_PARAMETER, what a misaligned raw read gives) must stop the
// backup rather than be written as zeros.
func TestReadResilientOtherErrorStops(t *testing.T) {
	src := &faultyReaderAt{data: pattern(4096), unit: 512, bad: map[uint64]bool{2: true}, err: syscall.Errno(87)}
	_, bad, err := collect(t, src, 0, 4096, 512, 4096)
	if err != syscall.Errno(87) {
		t.Fatalf("err = %v, want ERROR_INVALID_PARAMETER", err)
	}
	if len(bad) != 0 {
		t.Fatalf("nothing should be zero-filled, got %v", bad)
	}
}
