package main

import (
	"testing"

	"machinebackuplib"
)

// The Boot badge of the disk picker goes to the disk that has / or a boot
// partition, not to every sd and nvme disk.
func TestHoldsBootOrRoot(t *testing.T) {
	for _, c := range []struct {
		name  string
		parts []machinebackuplib.PartitionInfo
		want  bool
	}{
		{"root partition", []machinebackuplib.PartitionInfo{{Number: 1, Role: "system"}}, true},
		{"EFI partition", []machinebackuplib.PartitionInfo{{Number: 1, Role: "boot"}, {Number: 2}}, true},
		{"data disk", []machinebackuplib.PartitionInfo{{Number: 1, Mount: "/data"}}, false},
		{"empty disk", nil, false},
		{"unmounted partitions", []machinebackuplib.PartitionInfo{{Number: 1}, {Number: 2}}, false},
	} {
		if got := holdsBootOrRoot(c.parts); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}
