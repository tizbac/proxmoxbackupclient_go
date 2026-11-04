//go:build windows
// +build windows

package vss

import (
	"fmt"

	"github.com/go-ole/go-ole"
)

const _MIN_VSS_TIMEOUT = 180 * 1000

type Snapshotter struct {
	components *IVssBackupComponents
	timeout    int
}

func doAsyncOperation(async *IVssAsync, timeout int) error {
	defer func() {
		async.Cancel()
		async.Release()
	}()

	err := async.Wait(timeout)
	if err != nil {
		return err
	}

	status, err := async.QueryStatus()
	if err != nil {
		return err
	}

	if status == VSS_S_ASYNC_CANCELLED {
		return fmt.Errorf("async operation was cancelled")
	}

	if status == VSS_S_ASYNC_PENDING {
		return fmt.Errorf("async operation is pending")
	}

	if status != VSS_S_ASYNC_FINISHED {
		return fmt.Errorf("async operation returned bad status - 0x%x", status)
	}

	return nil
}

func (v *Snapshotter) CreateSnapshot(drive string, bootable bool, timeout int) (s *Snapshot, rerr error) {
	if v.components != nil {
		return nil, fmt.Errorf("snapshotter is already in use")
	}

	if timeout < _MIN_VSS_TIMEOUT {
		timeout = _MIN_VSS_TIMEOUT
	}

	// Initalize COM Library
	ole.CoInitialize(0)
	defer ole.CoUninitialize()
	vssBackupComponent, err := LoadAndInitVSS()
	if err != nil {
		return nil, err
	}

	v.timeout = timeout
	v.components = vssBackupComponent
	defer func() {
		if rerr != nil {
			v.components.AbortBackup()
			v.components.Release()
		}
	}()

	if err := v.components.SetContext(VSS_CTX_BACKUP); err != nil {
		return nil, err
	}

	if err := v.components.SetBackupState(false, bootable, VSS_BT_COPY, false); err != nil {
		return nil, err
	}

	var async *IVssAsync

	// TODO: GatherWriterMetadata should request check writers status and fail execution if any writer is in a failed state
	if async, err = v.components.GatherWriterMetadata(); err != nil {
		return nil, fmt.Errorf("VSS_GATHER - Shadow copy creation failed: GatherWriterMetadata, err: %s", err)
	} else if async == nil {
		return nil, fmt.Errorf("VSS_GATHER - Shadow copy creation failed: GatherWriterMetadata failed to return a valid IVssAsync object")
	}

	if err := doAsyncOperation(async, timeout); err != nil {
		return nil, fmt.Errorf("VSS_GATHER - Shadow copy creation failed: GatherWriterMetadata didn't finish properly, err: %s", err)
	}

	if isSupported, err := v.components.IsVolumeSupported(drive); err != nil {
		return nil, fmt.Errorf("VSS_VOLUME_SUPPORT - snapshots are not supported for drive %s, err: %s", drive, err)
	} else if !isSupported {
		return nil, fmt.Errorf("VSS_VOLUME_SUPPORT - snapshots are not supported for drive %s, err: %s", drive, err)
	}

	var snapshotSetID ole.GUID
	var snapshotID ole.GUID

	if err = v.components.StartSnapshotSet(&snapshotSetID); err != nil {
		return nil, fmt.Errorf("VSS_START - Shadow copy creation failed: StartSnapshotSet, err %s", err)
	}

	if err = v.components.AddToSnapshotSet(drive, &snapshotID); err != nil {
		return nil, fmt.Errorf("VSS_ADD - Shadow copy creation failed: AddToSnapshotSet, err: %s", err)
	}

	if async, err = v.components.PrepareForBackup(); err != nil {
		return nil, fmt.Errorf("VSS_PREPARE - Shadow copy creation failed: PrepareForBackup returned, err: %s", err)
	}
	if async == nil {
		return nil, fmt.Errorf("VSS_PREPARE - Shadow copy creation failed: PrepareForBackup failed to return a valid IVssAsync object")
	}

	if err := doAsyncOperation(async, timeout); err != nil {
		return nil, fmt.Errorf("VSS_PREPARE - Shadow copy creation failed: PrepareForBackup didn't finish properly, err %s", err)
	}

	if async, err = v.components.DoSnapshotSet(); err != nil {
		return nil, fmt.Errorf("VSS_SNAPSHOT - Shadow copy creation failed: DoSnapshotSet, err: %s", err)
	}
	if async == nil {
		return nil, fmt.Errorf("VSS_SNAPSHOT - Shadow copy creation failed: DoSnapshotSet failed to return a valid IVssAsync object")
	}

	if err := doAsyncOperation(async, timeout); err != nil {
		return nil, fmt.Errorf("VSS_SNAPSHOT - Shadow copy creation failed: DoSnapshotSet didn't finish properly, err: %s", err)
	}

	// Gather Properties
	properties := VssSnapshotProperties{}

	if err = vssBackupComponent.GetSnapshotProperties(snapshotID, &properties); err != nil {
		return nil, fmt.Errorf("VSS_PROPERTIES - GetSnapshotProperties, err: %s", err)
	}
	details := SnapshotDetails{}
	details, err = ParseProperties(properties)
	if err != nil {
		return nil, fmt.Errorf("VSS_PROPERTIES - ParseProperties, err: %s", err)
	}

	deviceObjectPath := details.DeviceObject + `\`
	snapshot := Snapshot{
		Id:               snapshotID.String(),
		Details:          details,
		Drive:            drive,
		DeviceObjectPath: deviceObjectPath,
	}

	// Check Snapshot is Complete
	if err := snapshot.Validate(); err != nil {
		return nil, err
	}

	return &snapshot, nil
}

func (v *Snapshotter) Release() error {
	if v.components == nil {
		return nil
	}

	var async *IVssAsync
	var err error

	if async, err = v.components.BackupComplete(); err != nil {
		return fmt.Errorf("VSS_COMPLETE - Shadow copy creation failed: BackupComplete, err: %s", err)
	} else if async == nil {
		return fmt.Errorf("VSS_COMPLETE - Shadow copy creation failed: BackupComplete failed to return a valid IVssAsync object")
	}

	if err = doAsyncOperation(async, v.timeout); err != nil {
		return fmt.Errorf("VSS_COMPLETE - Shadow copy creation failed: BackupComplete didn't finish properly, err: %s", err)
	}

	// TODO: GatherWriterStatus should request check writers status and fail execution if any writer is in a failed state
	// After calling BackupComplete, requesters must call GatherWriterStatus to cause the writer session to be set to a completed state.
	// This is only necessary on Windows Server 2008 with Service Pack 2 (SP2) and earlier.
	if async, err = v.components.GatherWriterStatus(); err != nil {
		return fmt.Errorf("VSS_GATHER - Shadow copy creation failed: GatherWriterStatus, err: %s", err)
	} else if async == nil {
		return fmt.Errorf("VSS_GATHER - Shadow copy creation failed: GatherWriterStatus failed to return a valid IVssAsync object")
	}

	if err = doAsyncOperation(async, v.timeout); err != nil {
		return fmt.Errorf("VSS_GATHER - Shadow copy creation failed: GatherWriterStatus didn't finish properly, err: %s", err)
	}

	// The caller of GatherWriterStatus should also call FreeWriterStatus after receiving the status of each writer.
	if err = v.components.FreeWriterStatus(); err != nil {
		return fmt.Errorf("VSS_GATHER - Shadow copy creation failed: FreeWriterStatus, err: %s", err)
	}

	v.components.Release()
	v.components = nil

	return nil
}
