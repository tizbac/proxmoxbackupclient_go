package snapshot

import (
	"runtime"
	"testing"
)

// The GUI calls DetectControl on every platform to show which Linux snapshot
// module a machine backup will use. Off Linux there is none.
func TestDetectControlOffLinux(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("on Linux the answer depends on the machine; see TestDetectControlNamesTheModule")
	}
	if c, ok := DetectControl(); ok || c != (SnapControl{}) {
		t.Fatalf("DetectControl() = %+v, %v; want nothing found", c, ok)
	}
}

// When a module is found, the GUI shows its name, so it must be one of the
// two the backup knows how to drive.
func TestDetectControlNamesTheModule(t *testing.T) {
	c, ok := DetectControl()
	if !ok {
		t.Skip("no elastio-snap or dattobd module on this machine")
	}
	if c.Name != "elastio-snap" && c.Name != "dattobd" {
		t.Fatalf("DetectControl().Name = %q, want elastio-snap or dattobd", c.Name)
	}
	if c.CtlDevice == "" || c.DevPrefix == "" {
		t.Fatalf("DetectControl() = %+v: control device and device prefix must be set", c)
	}
}
