//go:build windows

package snapshot

import (
	"encoding/hex"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/st-matskevich/go-vss"
)

// TestLiveVSSProperties makes one shadow copy of the volume in PBS_VSS_PROPS
// (for example C:\) the way createShadow does, and shows what
// GetSnapshotProperties returned: the structure as bytes and as go-vss reads
// it, a second call into a larger buffer filled with 0xAA (to see how many
// bytes VSS writes), and Win32_ShadowCopy's device object for the same ID.
// The shadow is deleted at the end. For tizbac/proxmoxbackupclient_go#131:
// on Windows Server 2025 the device object comes back empty.
// Run elevated: PBS_VSS_PROPS=C:\ snapshot.test -test.run LiveVSSProperties -test.v
func TestLiveVSSProperties(t *testing.T) {
	vol := os.Getenv("PBS_VSS_PROPS")
	if vol == "" {
		t.Skip(`set PBS_VSS_PROPS to a volume, for example C:\`)
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	uninit, err := initCOM()
	if err != nil {
		t.Fatal(err)
	}
	defer uninit()

	c, err := newVSSComponents()
	if err != nil {
		t.Fatal(err)
	}
	// Deferred first so it runs last: the shadow goes after BackupComplete.
	var id string
	defer func() {
		if id != "" {
			deleteShadow(id)
		}
	}()
	s := &vssShadow{c: c, timeoutMs: vssTimeoutMs}
	defer func() {
		if err := s.release(); err != nil {
			t.Logf("release: %v", err)
		}
	}()
	check := func(step string, err error) {
		if err != nil {
			c.AbortBackup()
			t.Fatalf("%s: %v", step, err)
		}
	}
	check("SetContext", c.SetContext(vss.VSS_CTX_BACKUP))
	check("SetBackupState", c.SetBackupState(false, false, vss.VSS_BT_COPY, false))
	async, err := c.GatherWriterMetadata()
	check("GatherWriterMetadata", waitVSS("GatherWriterMetadata", async, err, vssTimeoutMs))
	var setID, snapID ole.GUID
	check("StartSnapshotSet", c.StartSnapshotSet(&setID))
	check("AddToSnapshotSet", c.AddToSnapshotSet(vol, &snapID))
	async, err = c.PrepareForBackup()
	check("PrepareForBackup", waitVSS("PrepareForBackup", async, err, vssTimeoutMs))
	async, err = c.DoSnapshotSet()
	check("DoSnapshotSet", waitVSS("DoSnapshotSet", async, err, vssTimeoutMs))
	id = snapID.String()
	t.Logf("snapshot %s in set %s; go-vss VssSnapshotProperties is %d bytes, pointers %d",
		id, setID.String(), unsafe.Sizeof(vss.VssSnapshotProperties{}), unsafe.Sizeof(uintptr(0)))

	var props vss.VssSnapshotProperties
	err = c.GetSnapshotProperties(snapID, &props)
	t.Logf("GetSnapshotProperties: err=%v", err)
	t.Logf("bytes:\n%s", hex.Dump(unsafe.Slice((*byte)(unsafe.Pointer(&props)), unsafe.Sizeof(props))))
	d, _ := vss.ParseProperties(props)
	t.Logf("as go-vss reads it: device=%q volume=%q machine=%q service=%q provider=%s status=%s attributes=%v",
		d.DeviceObject, d.VolumeName, d.OriginatingMachine, d.ServiceMachine, d.ProviderID, d.Status, props.GetSnapshotAttributesVerbose())
	procVssFreeSnapshotProperties.Call(uintptr(unsafe.Pointer(&props)))

	var big [256]byte
	for i := range big {
		big[i] = 0xAA
	}
	err = c.GetSnapshotProperties(snapID, (*vss.VssSnapshotProperties)(unsafe.Pointer(&big[0])))
	written := len(big)
	for written > 0 && big[written-1] == 0xAA {
		written--
	}
	t.Logf("second call into 256 bytes of 0xAA: err=%v, last byte written at %d:\n%s", err, written, hex.Dump(big[:]))
	procVssFreeSnapshotProperties.Call(uintptr(unsafe.Pointer(&big[0])))

	q := "Get-CimInstance Win32_ShadowCopy -Filter \"ID='" + strings.ToUpper(id) + "'\" | Format-List ID, DeviceObject, VolumeName, ClientAccessible, Persistent, State"
	out, err := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", q).CombinedOutput()
	t.Logf("Win32_ShadowCopy (err=%v):\n%s", err, strings.TrimSpace(string(out)))

	// The fallback createShadow uses when VSS gives no device must give the
	// same device as VSS does here.
	wmiDev, err := shadowDeviceFromWMI(id)
	t.Logf("shadowDeviceFromWMI: %q, err=%v", wmiDev, err)
	if err != nil || wmiDev == "" {
		t.Errorf("WMI fallback found no device for %s: %v", id, err)
	} else if d.DeviceObject != "" && !strings.EqualFold(wmiDev, d.DeviceObject) {
		t.Errorf("WMI fallback gives %q, VSS %q", wmiDev, d.DeviceObject)
	}
	if _, err := shadowDeviceFromWMI("{00000000-0000-0000-0000-000000000000}"); err == nil {
		t.Error("WMI fallback found a device for a shadow copy that does not exist")
	}
}
