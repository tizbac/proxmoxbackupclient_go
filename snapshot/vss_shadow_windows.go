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

// The shadow copy calls of go-vss's Snapshotter.CreateSnapshot (MIT, Jerome
// Hadorn and Stanislau Matskevich), made here step by step for what its
// Snapshotter doesn't do: a multithreaded apartment (initCOM), a step that is
// still running when Wait returns (pollVSS), and one shadow copy creation at a
// time in the process (shadowMu).
//
// Found 2026-10-21: every snapshot left about 33 handles behind in the process
// (threads that had ended, events, semaphores), so the GUI and the service,
// which run for weeks, kept growing. go-vss up to v0.3.x took a QueryInterface
// reference on each object VSS returned and never released the first; v0.4.2
// returns the objects as VSS gives them (st-matskevich/go-vss#16), so one
// Release frees each. TestLiveVSSRefCounts and TestLiveVSSLeakHunt check it.
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

// newVSSComponents is vss.LoadAndInitVSS: the object it returns is freed by
// one Release.
func newVSSComponents() (*vss.IVssBackupComponents, error) {
	return vss.LoadAndInitVSS()
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
	// Nothing here reads the writers' metadata, so it is freed straight away,
	// as go-vss does.
	if err := c.FreeWriterMetadata(); err != nil {
		return nil, fmt.Errorf("VSS_GATHER - FreeWriterMetadata: %w", err)
	}
	// GUID_NULL as the provider: VSS picks it.
	var provider ole.GUID
	if ok, err := c.IsVolumeSupported(provider, drive); err != nil || !ok {
		return nil, fmt.Errorf("VSS_VOLUME_SUPPORT - snapshots are not supported for drive %s, err: %v", drive, err)
	}
	var setID, snapshotID ole.GUID
	if err := c.StartSnapshotSet(&setID); err != nil {
		return nil, fmt.Errorf("VSS_START - StartSnapshotSet: %w", err)
	}
	if err := c.AddToSnapshotSet(drive, provider, &snapshotID); err != nil {
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

	// The shadow copy's device. With GetSnapshotProperties, Windows Server
	// 2025 was reported giving none (tizbac/proxmoxbackupclient_go#131): the
	// QEMU guest agent's VSS provider answers it for any shadow copy, with
	// empty properties (st-matskevich/go-vss#15). The path became just `\`,
	// which passes Validate, so a folder backup read the live volume without
	// saying so, and a machine backup failed. Query lists only what each
	// provider made, as go-vss now reads it; WMI is asked if that has no
	// device either. With neither, the backup must stop rather than read the
	// live volume.
	details, qerr := queriedSnapshotDetails(c, snapshotID)
	device := details.DeviceObject
	if device == "" {
		d, werr := shadowDeviceFromWMI(snapshotID.String())
		if werr != nil || d == "" {
			deleteShadow(snapshotID.String())
			return nil, fmt.Errorf("VSS_PROPERTIES - VSS gave no device for shadow copy %s (%v), and WMI none either (%v): stopping rather than reading the live volume", snapshotID.String(), qerr, werr)
		}
		fmt.Printf("VSS gave no device for shadow copy %s (%v); WMI gives %s\n", snapshotID.String(), qerr, d)
		device = d
		details.Id = snapshotID.String()
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

// vssObjectProps is go-vss's VssObjectProperties, one entry of
// IVssBackupComponents::Query, with its fields readable here: go-vss reads the
// entries itself only inside Snapshotter.CreateSnapshot. The layout is go-vss's
// (TestVSSObjectPropsLayout).
type vssObjectProps struct {
	objectType uint32
	_          uint32
	snapshot   vss.VssSnapshotProperties
	_          uint32
}

// queriedSnapshotDetails returns the properties of shadow copy id from
// IVssBackupComponents::Query, as go-vss's own CreateSnapshot reads them.
func queriedSnapshotDetails(c *vss.IVssBackupComponents, id ole.GUID) (vss.SnapshotDetails, error) {
	list, err := c.Query()
	if err != nil {
		return vss.SnapshotDetails{}, fmt.Errorf("Query: %w", err)
	}
	if list == nil {
		return vss.SnapshotDetails{}, errors.New("Query returned no list")
	}
	defer list.Release()
	want := id.String()
	for {
		var o vssObjectProps
		fetched, err := list.Next((*vss.VssObjectProperties)(unsafe.Pointer(&o)))
		if err != nil {
			return vss.SnapshotDetails{}, fmt.Errorf("Query: %w", err)
		}
		if !fetched {
			return vss.SnapshotDetails{}, fmt.Errorf("Query does not list shadow copy %s", want)
		}
		details, err := vss.ParseProperties(o.snapshot) // copies the strings
		procVssFreeSnapshotProperties.Call(uintptr(unsafe.Pointer(&o.snapshot)))
		if details.Id == want {
			return details, err
		}
	}
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
