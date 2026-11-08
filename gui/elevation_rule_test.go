package main

import (
	"testing"

	"github.com/salvanza/salvanza_client/gui/api"
)

func TestMachineNeedsElevation(t *testing.T) {
	cases := []struct {
		goos           string
		admin, service bool
		mode           api.ExecutionMode
		want           bool
	}{
		{"linux", false, false, api.ModeStandalone, true}, // the case the box is for
		{"linux", false, false, api.ModeService, false},   // the service reads the disks
		{"linux", true, false, api.ModeStandalone, false}, // already root
		{"linux", false, true, api.ModeStandalone, false}, // this is the service
		{"windows", false, false, api.ModeStandalone, false},
		{"darwin", false, false, api.ModeStandalone, false},
	}
	for _, c := range cases {
		if got := machineNeedsElevation(c.goos, c.admin, c.service, c.mode); got != c.want {
			t.Errorf("machineNeedsElevation(%s, admin=%v, service=%v, %s) = %v, want %v",
				c.goos, c.admin, c.service, c.mode.String(), got, c.want)
		}
	}
}
