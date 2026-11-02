package machinebackuplib

import (
	"math"
	"testing"
)

// The job-wide progress fraction must always be a number between 0 and 1:
// the GUI and the log print it as a percentage, and NaN showed as "NaN%".
func TestJobProgressFraction(t *testing.T) {
	cases := []struct {
		name      string
		base, dev uint64
		fraction  float64
		total     uint64
		want      float64
	}{
		{"half of the only disk", 0, 100, 0.5, 100, 0.5},
		{"second disk starts", 100, 300, 0, 400, 0.25},
		{"second disk done", 100, 300, 1, 400, 1},
		{"no disk size known", 0, 0, 0.5, 0, 0},
		{"reader reported 0/0", 100, 300, math.NaN(), 400, 0.25},
		{"reader reported x/0", 100, 300, math.Inf(1), 400, 0.25},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := jobProgressFraction(tc.base, tc.dev, tc.fraction, tc.total); got != tc.want {
				t.Errorf("jobProgressFraction(%d, %d, %v, %d) = %v, want %v", tc.base, tc.dev, tc.fraction, tc.total, got, tc.want)
			}
		})
	}
}

// A malformed \\.\PhysicalDriveN used to panic with an index out of range.
func TestPhysicalDriveIndex(t *testing.T) {
	for dev, want := range map[string]int{`\\.\PhysicalDrive0`: 0, `\\.\PhysicalDrive12`: 12} {
		if got, err := physicalDriveIndex(dev); err != nil || got != want {
			t.Errorf("physicalDriveIndex(%q) = %d, %v; want %d", dev, got, err, want)
		}
	}
	for _, dev := range []string{`\\.\PhysicalDrive`, `\\.\PhysicalDriveX`, `\\.\PhysicalDrive1x`, `\\.\PhysicalDrive99999999999`} {
		if _, err := physicalDriveIndex(dev); err == nil {
			t.Errorf("physicalDriveIndex(%q): want an error", dev)
		}
	}
}
