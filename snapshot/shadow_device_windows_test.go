//go:build windows

package snapshot

import (
	"errors"
	"strings"
	"testing"
)

const testShadowID = "{11111111-2222-3333-4444-555555555555}"

// The device VSS gives is used as it is, without asking WMI.
func TestShadowDeviceFromVSS(t *testing.T) {
	asked := false
	dev, err := shadowDevice(testShadowID, `\\?\GLOBALROOT\Device\HarddiskVolumeShadowCopy7`, func(string) (string, error) {
		asked = true
		return "", nil
	})
	if err != nil || dev != `\\?\GLOBALROOT\Device\HarddiskVolumeShadowCopy7` || asked {
		t.Fatalf("got %q %v (WMI asked: %v)", dev, err, asked)
	}
}

// VSS gives no device (#131): WMI's is used.
func TestShadowDeviceFallsBackToWMI(t *testing.T) {
	dev, err := shadowDevice(testShadowID, "", func(id string) (string, error) {
		if id != testShadowID {
			t.Errorf("WMI asked for %q", id)
		}
		return `\\?\GLOBALROOT\Device\HarddiskVolumeShadowCopy8`, nil
	})
	if err != nil || dev != `\\?\GLOBALROOT\Device\HarddiskVolumeShadowCopy8` {
		t.Fatalf("got %q %v", dev, err)
	}
}

// Neither VSS nor WMI gives a device: an error, never an empty device (which
// would make the path `\` and read the live volume).
func TestShadowDeviceNeitherStops(t *testing.T) {
	for _, wmi := range []func(string) (string, error){
		func(string) (string, error) { return "", errors.New("no Win32_ShadowCopy") },
		func(string) (string, error) { return "", nil },
	} {
		dev, err := shadowDevice(testShadowID, "", wmi)
		if err == nil || dev != "" || !strings.Contains(err.Error(), "live volume") {
			t.Fatalf("got %q %v", dev, err)
		}
	}
}
