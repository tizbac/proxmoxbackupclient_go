package main

import "testing"

func TestMachineSnapshotType(t *testing.T) {
	for in, want := range map[string]string{
		"machine":    "host",
		"":           "host",
		"directory":  "host",
		"machine-vm": "vm",
	} {
		if got := machineSnapshotType(in); got != want {
			t.Errorf("machineSnapshotType(%q) = %q, want %q", in, got, want)
		}
	}
}
