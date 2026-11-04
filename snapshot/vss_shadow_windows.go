//go:build windows

package snapshot

import (
	"errors"
	"fmt"
	"sync"
	"time"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/st-matskevich/go-vss"
	"golang.org/x/sys/windows"
)

// The shadow copy calls of go-vss v0.3.3 (MIT, Jerome Hadorn and Stanislau
// Matskevich), with its object lifetimes fixed. Found 2026-10-21: every
// snapshot left about 33 handles behind in the process (threads that had
// ended, events, semaphores), so the GUI and the service, which run for
// weeks, kept growing. The fixes below brought it to about 21; the last of
// it was found 2026-11-04 (TestLiveVSSRefCounts, TestLiveVSSLeakHunt).
//
//   - vss.LoadAndInitVSS takes a QueryInterface reference on the object
//     CreateVssBackupComponents returned and never releases the first one, so
//     one Release never freed it (newVSSComponents).
//   - Its five methods that start an asynchronous step do the same with the
//     IVssAsync they return, and every async object holds a reference to
//     the VSS object, so neither was ever freed (waitVSS).
//   - Snapshotter.Release returned early, without releasing the object, when
//     BackupComplete or GatherWriterStatus failed (vssShadow.release always
//     releases).
//   - The snapshot properties VSS allocates were never freed
//     (VssFreeSnapshotProperties).
//
// COM must be initialised on the calling thread (initCOM), which must not
// change until release (CreateVSSSnapshot locks it).

var procVssFreeSnapshotProperties = windows.NewLazySystemDLL("VssApi.dll").NewProc("VssFreeSnapshotProperties")

// What CoInitializeEx returns when COM is already initialised on the thread:
// in the same apartment (S_FALSE), or in the other one (RPC_E_CHANGED_MODE).
const (
	comSFalse          = 0x00000001
	comRPCEChangedMode = 0x80010106
)

// initCOM initialises COM on the calling thread in the multithreaded
// apartment and returns what undoes it; the caller must have locked the
// thread. A VSS requestor must be multithreaded. go-vss and this package used
// CoInitialize, a single-threaded apartment, where the writers' calls back
// arrive through a message queue nothing here pumps: after the February 2026
// Windows update GatherWriterMetadata took about 182 s on Server 2022
// (tizbac/proxmoxbackupclient_go#67).
//
// S_FALSE (already multithreaded) still counts as a call to undo, as for any
// successful CoInitializeEx. RPC_E_CHANGED_MODE (already single-threaded, by
// someone else) is not ours to undo; VSS still works there, only slowly.
func initCOM() (func(), error) {
	err := ole.CoInitializeEx(0, ole.COINIT_MULTITHREADED)
	if err == nil {
		return ole.CoUninitialize, nil
	}
	var oe *ole.OleError
	if errors.As(err, &oe) {
		switch oe.Code() {
		case comSFalse:
			return ole.CoUninitialize, nil
		case comRPCEChangedMode:
			fmt.Println("VSS: COM is already single-threaded on this thread; using it as it is")
			return func() {}, nil
		}
	}
	return nil, fmt.Errorf("COM initialisation: %w", err)
}

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

// vssPollEvery is how often waitVSS asks again whether a step is done once
// IVssAsync::Wait has returned with it still running.
var vssPollEvery = 500 * time.Millisecond

// waitVSS waits for one asynchronous VSS step (as go-vss's doAsyncOperation)
// and releases it. The step may take timeoutMs in all.
func waitVSS(step string, async *vss.IVssAsync, err error, timeoutMs int) error {
	if err != nil {
		return fmt.Errorf("%s: %w", step, err)
	}
	if async == nil {
		return fmt.Errorf("%s: VSS returned no IVssAsync object", step)
	}
	// go-vss hands each IVssAsync out with two references: the one VSS
	// returned and one more from the QueryInterface it adds (as
	// LoadAndInitVSS does; newVSSComponents). Drop the extra one here, or the
	// release below never frees the object. Each async object holds a
	// reference to the VSS object, which then outlived its own release too,
	// with the thread VSS starts for it: every snapshot left 5 references,
	// a live thread and about 22 handles behind (found 2026-11-04 by
	// counting the VSS object's references after each step).
	async.Release()
	defer func() {
		async.Cancel()
		async.Release()
	}()
	timeout := time.Duration(timeoutMs) * time.Millisecond
	deadline := time.Now().Add(timeout)
	if err := async.Wait(timeoutMs); err != nil {
		return fmt.Errorf("%s: %w", step, err)
	}
	return pollVSS(step, async.QueryStatus, deadline, timeout, vssPollEvery)
}

// pollVSS reads a VSS step's status (IVssAsync::QueryStatus) after
// IVssAsync::Wait has returned, and while it is still pending asks again every
// "every" until it ends or deadline passes. Wait can return with the step
// still running: go-vss failed the step then ("async operation is pending"),
// though it would have finished seconds later
// (tizbac/proxmoxbackupclient_go#67).
func pollVSS(step string, status func() (vss.HRESULT, error), deadline time.Time, timeout, every time.Duration) error {
	told := false
	for {
		st, err := status()
		if err != nil {
			return fmt.Errorf("%s: %w", step, err)
		}
		switch st {
		case vss.VSS_S_ASYNC_FINISHED:
			return nil
		case vss.VSS_S_ASYNC_CANCELLED:
			return fmt.Errorf("%s didn't finish properly: async operation was cancelled", step)
		case vss.VSS_S_ASYNC_PENDING:
		default:
			return fmt.Errorf("%s didn't finish properly: async operation returned bad status - 0x%x", step, uint32(st))
		}
		if !time.Now().Before(deadline) {
			return fmt.Errorf("%s didn't finish properly: async operation is still pending after %v", step, timeout)
		}
		if !told {
			fmt.Printf("VSS: %s still running after Wait returned; waiting on\n", step)
			told = true
		}
		time.Sleep(every)
	}
}

// shadowMu lets one shadow copy at a time be made or released in this
// process. Several backups can run at once now; Windows makes one shadow copy
// set at a time and answers a second with "another shadow copy creation is
// already in progress", which retryWhileBusy then waited out 30 s at a time.
// Waiting here instead takes only as long as the other creation (seconds).
// The shadow copies themselves are held side by side for as long as each
// backup needs; only making and releasing them take turns.
var shadowMu sync.Mutex

// vssShadow is a shadow copy made by createShadow, held until release.
type vssShadow struct {
	vss.Snapshot
	c         *vss.IVssBackupComponents
	timeoutMs int
}

// createShadow makes a shadow copy of drive (for example `C:\`) in the backup
// context, as a copy backup, the way go-vss's Snapshotter.CreateSnapshot does.
// Each step may take timeoutMs.
func createShadow(drive string, timeoutMs int) (s *vssShadow, err error) {
	shadowMu.Lock()
	defer shadowMu.Unlock()
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
	// The shadow copy's device. Windows Server 2025 build 26100.1742 was
	// reported giving none (tizbac/proxmoxbackupclient_go#131): the path
	// became just `\`, which passes Validate, so a folder backup read the
	// live volume without saying so, and a machine backup failed. WMI still
	// knows the device then. With neither, the backup must stop rather than
	// read the live volume.
	device := details.DeviceObject
	if device == "" {
		d, werr := shadowDeviceFromWMI(snapshotID.String())
		if werr != nil || d == "" {
			deleteShadow(snapshotID.String())
			return nil, fmt.Errorf("VSS_PROPERTIES - VSS gave no device for shadow copy %s, and WMI none either (%v): stopping rather than reading the live volume", snapshotID.String(), werr)
		}
		fmt.Printf("VSS gave no device for shadow copy %s; WMI gives %s\n", snapshotID.String(), d)
		device = d
		details.DeviceObject = d
	}
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

// release tells VSS the backup is done and frees the VSS object, whatever
// fails on the way. The shadow copy itself is deleted by deleteShadow.
func (s *vssShadow) release() error {
	if s == nil || s.c == nil {
		return nil
	}
	shadowMu.Lock()
	defer shadowMu.Unlock()
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
