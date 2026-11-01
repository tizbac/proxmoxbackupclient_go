//go:build windows

package snapshot

import (
	"os"
	"runtime"
	"strings"
	"testing"

	"github.com/go-ole/go-ole"
)

// TestLiveWMIDeviceMatchesVSS makes one shadow copy of the volume in
// PBS_VSS_WMI (for example C:\) as CreateVSSSnapshot does, and checks that
// the WMI fallback (shadowDeviceFromWMI) gives the same device as VSS, and
// none for a shadow copy that does not exist. The shadow is deleted at the
// end. Run elevated: PBS_VSS_WMI=C:\ snapshot.test -test.run LiveWMIDevice -test.v
func TestLiveWMIDeviceMatchesVSS(t *testing.T) {
	vol := os.Getenv("PBS_VSS_WMI")
	if vol == "" {
		t.Skip(`set PBS_VSS_WMI to a volume, for example C:\`)
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ole.CoInitialize(0)
	defer ole.CoUninitialize()

	s, err := createShadow(vol, vssTimeoutMs)
	if err != nil {
		t.Fatalf("createShadow: %v", err)
	}
	defer func() {
		if err := s.release(); err != nil {
			t.Logf("release: %v", err)
		}
		deleteShadowsBestEffort([]string{s.Id})
	}()
	t.Logf("shadow %s, VSS device %q", s.Id, s.Details.DeviceObject)

	dev, err := shadowDeviceFromWMI(s.Id)
	if err != nil || dev == "" {
		t.Fatalf("WMI gave no device for %s: %v", s.Id, err)
	}
	t.Logf("WMI device %q", dev)
	if !strings.EqualFold(dev, s.Details.DeviceObject) {
		t.Errorf("WMI gives %q, VSS %q", dev, s.Details.DeviceObject)
	}
	if _, err := shadowDeviceFromWMI("{00000000-0000-0000-0000-000000000000}"); err == nil {
		t.Error("WMI fallback found a device for a shadow copy that does not exist")
	}
}
