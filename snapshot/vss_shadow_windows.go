//go:build windows

package snapshot

import (
	"fmt"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/st-matskevich/go-vss"
	"golang.org/x/sys/windows"
)

// The shadow copy calls of go-vss v0.3.3 (MIT, Jerome Hadorn and Stanislau
// Matskevich), with its object lifetimes fixed. Every snapshot left about 33
// handles behind in the process (threads that had ended, events,
// semaphores), so the GUI and the service, which run for weeks, kept
// growing. The fixes below bring it to about 21 (measured by
// TestLiveVSSHandles); the rest is inside VSS's own create and release.
//
//   - vss.LoadAndInitVSS takes a QueryInterface reference on the object
//     CreateVssBackupComponents returned and never releases the first one, so
//     one Release never freed it (newVSSComponents).
//   - Snapshotter.Release returned early, without releasing the object, when
//     BackupComplete or GatherWriterStatus failed (vssShadow.release always
//     releases).
//   - The snapshot properties VSS allocates were never freed
//     (VssFreeSnapshotProperties).
//
// COM must be initialised on the calling thread, which must not change until
// release (CreateVSSSnapshot locks it).

var procVssFreeSnapshotProperties = windows.NewLazySystemDLL("VssApi.dll").NewProc("VssFreeSnapshotProperties")

// newVSSComponents is vss.LoadAndInitVSS without its extra reference: the
// object it returns is freed by one Release.
func newVSSComponents() (*vss.IVssBackupComponents, error) {
	c, err := vss.LoadAndInitVSS()
	if err != nil {
		return nil, err
	}
	// LoadAndInitVSS asked the object for the interface it already is
	// (IVssBackupComponents), which returns the same pointer with one more
	// reference, and kept both. Drop one.
	c.Release()
	return c, nil
}

// waitVSS waits for one asynchronous VSS step (as go-vss's doAsyncOperation)
// and releases it.
func waitVSS(step string, async *vss.IVssAsync, err error, timeoutMs int) error {
	if err != nil {
		return fmt.Errorf("%s: %w", step, err)
	}
	if async == nil {
		return fmt.Errorf("%s: VSS returned no IVssAsync object", step)
	}
	defer func() {
		async.Cancel()
		async.Release()
	}()
	if err := async.Wait(timeoutMs); err != nil {
		return fmt.Errorf("%s: %w", step, err)
	}
	status, err := async.QueryStatus()
	if err != nil {
		return fmt.Errorf("%s: %w", step, err)
	}
	switch status {
	case vss.VSS_S_ASYNC_FINISHED:
		return nil
	case vss.VSS_S_ASYNC_CANCELLED:
		return fmt.Errorf("%s didn't finish properly: async operation was cancelled", step)
	case vss.VSS_S_ASYNC_PENDING:
		return fmt.Errorf("%s didn't finish properly: async operation is pending", step)
	}
	return fmt.Errorf("%s didn't finish properly: async operation returned bad status - 0x%x", step, status)
}

// vssShadow is a shadow copy made by createShadow, held until release.
type vssShadow struct {
	vss.Snapshot
	c         *vss.IVssBackupComponents
	timeoutMs int
}

// createShadow makes a shadow copy of drive (for example `C:\`) in the backup
// context, as a copy backup, the way go-vss's Snapshotter.CreateSnapshot does.
// Each step may take timeoutMs. On failure the VSS object is aborted and
// freed here, so the caller has nothing to release.
func createShadow(drive string, timeoutMs int) (s *vssShadow, err error) {
	c, err := newVSSComponents()
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			c.AbortBackup()
			c.Release()
		}
	}()
	if err := c.SetContext(vss.VSS_CTX_BACKUP); err != nil {
		return nil, err
	}
	if err := c.SetBackupState(false, false, vss.VSS_BT_COPY, false); err != nil {
		return nil, err
	}
	async, err := c.GatherWriterMetadata()
	if err := waitVSS("VSS_GATHER - GatherWriterMetadata", async, err, timeoutMs); err != nil {
		return nil, err
	}
	if ok, err := c.IsVolumeSupported(drive); err != nil || !ok {
		return nil, fmt.Errorf("VSS_VOLUME_SUPPORT - snapshots are not supported for drive %s, err: %v", drive, err)
	}
	var setID, snapshotID ole.GUID
	if err := c.StartSnapshotSet(&setID); err != nil {
		return nil, fmt.Errorf("VSS_START - StartSnapshotSet: %w", err)
	}
	if err := c.AddToSnapshotSet(drive, &snapshotID); err != nil {
		return nil, fmt.Errorf("VSS_ADD - AddToSnapshotSet: %w", err)
	}
	async, err = c.PrepareForBackup()
	if err := waitVSS("VSS_PREPARE - PrepareForBackup", async, err, timeoutMs); err != nil {
		return nil, err
	}
	async, err = c.DoSnapshotSet()
	if err := waitVSS("VSS_SNAPSHOT - DoSnapshotSet", async, err, timeoutMs); err != nil {
		return nil, err
	}

	var props vss.VssSnapshotProperties
	if err := c.GetSnapshotProperties(snapshotID, &props); err != nil {
		return nil, fmt.Errorf("VSS_PROPERTIES - GetSnapshotProperties: %w", err)
	}
	details, err := vss.ParseProperties(props) // copies the strings
	procVssFreeSnapshotProperties.Call(uintptr(unsafe.Pointer(&props)))
	if err != nil {
		return nil, fmt.Errorf("VSS_PROPERTIES - ParseProperties: %w", err)
	}
	// The shadow copy's device. On Windows Server 2025 build 26100.1742, run
	// as SYSTEM, VSS gave none (#131): the path became just `\`, which passes
	// Validate, so a folder backup read the live volume without saying so and
	// a machine backup failed. WMI still knows the device then. With neither,
	// delete the shadow copy and stop rather than read the live volume.
	device, err := shadowDevice(snapshotID.String(), details.DeviceObject, shadowDeviceFromWMI)
	if err != nil {
		deleteShadowsBestEffort([]string{snapshotID.String()})
		return nil, err
	}
	details.DeviceObject = device
	shadow := &vssShadow{
		Snapshot: vss.Snapshot{
			Id:               snapshotID.String(),
			Details:          details,
			Drive:            drive,
			DeviceObjectPath: device + `\`,
		},
		c:         c,
		timeoutMs: timeoutMs,
	}
	if err := shadow.Validate(); err != nil {
		return nil, err
	}
	return shadow, nil
}

// shadowDevice returns the device of shadow copy id: the one VSS gave
// (vssDevice) or, when that is empty, the one fromWMI finds. With neither it
// returns an error, so the backup stops instead of reading the live volume.
func shadowDevice(id, vssDevice string, fromWMI func(string) (string, error)) (string, error) {
	if vssDevice != "" {
		return vssDevice, nil
	}
	d, err := fromWMI(id)
	if err != nil || d == "" {
		return "", fmt.Errorf("VSS_PROPERTIES - VSS gave no device for shadow copy %s, and WMI none either (%v): stopping rather than reading the live volume", id, err)
	}
	fmt.Printf("VSS gave no device for shadow copy %s; WMI gives %s\n", id, d)
	return d, nil
}

// release tells VSS the backup is done and frees the VSS object, whatever
// fails on the way. The shadow copy itself is deleted afterwards by
// deleteShadowsBestEffort.
func (s *vssShadow) release() error {
	if s == nil || s.c == nil {
		return nil
	}
	defer func() {
		s.c.Release()
		s.c = nil
	}()
	async, err := s.c.BackupComplete()
	if err := waitVSS("VSS_COMPLETE - BackupComplete", async, err, s.timeoutMs); err != nil {
		return err
	}
	async, err = s.c.GatherWriterStatus()
	if err := waitVSS("VSS_GATHER - GatherWriterStatus", async, err, s.timeoutMs); err != nil {
		return err
	}
	return s.c.FreeWriterStatus()
}
