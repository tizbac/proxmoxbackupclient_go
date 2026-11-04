package vss

import (
	"errors"
	"fmt"
	"time"
	"unsafe"
)

//NOTE: Microsoft Documentation can be found here: https://docs.microsoft.com/en-us/windows/win32/api/vss/ne-vss-vss_snapshot_context

// The VssContex enumeration enables a requester using `IVssBackupComponents::SetContext` to specify how a shadow copy is to be created, queried, or deleted and the degree of writer involvement.
type VssContext uint

const (
	VSS_CTX_BACKUP                    VssContext = iota // POSSIBLE: Default backup context. Auto-release, nonpersistent shadow copy in which writers are involved in the creation.
	VSS_CTX_FILE_SHARE_BACKUP                           // FAILS: Specifies an auto-release, nonpersistent shadow copy created without writer involvement.
	VSS_CTX_NAS_ROLLBACK                                // POSSIBLE: Specifies a persistent, non-auto-release shadow copy without writer involvement. This context should be used when files are in a consistent state at the time of the shadow copy.
	VSS_CTX_APP_ROLLBACK                                // Fails: Specifies a persistent, non-auto-release shadow copy with writer involvement. This context is designed to be used when writers are needed to ensure that files are in a well-defined state prior to shadow copy.
	VSS_CTX_CLIENT_ACCESSIBLE                           // Fails: Specifies a read-only, client-accessible shadow copy that supports Shadow Copies for Shared Folders and is created without writer involvement. Only the system provider can create this type of shadow copy.
	VSS_CTX_CLIENT_ACCESSIBLE_WRITERS                   // Fails: Specifies a read-only, client-accessible shadow copy that is created with writer involvement. Only the system provider an create this type of shadow copy.
	VSS_CTX_ALL                                         // Fails: All types of currently live shadow copies are available for administrative operations
)

//NOTE: Microsoft Documentation can be found here: https://docs.microsoft.com/en-us/windows/win32/api/vss/ne-vss-vss_backup_type

// The VssBackup enumeration indicates the type of backup to be performed using VSS writer/requester coordination.
type VssBackup uint

const (
	VSS_BT_UNDEFINED    VssBackup = iota // The backup type is not known, indicates an application error.
	VSS_BT_FULL                          // All files, regardless of whether they have been marked as backed up or not, are saved. This is the default backup and all writers support it.
	VSS_BT_INCREMENTAL                   // Files created or changed since the last full or incremental backup are saved.
	VSS_BT_DIFFERENTIAL                  // Files created or changed since the last full backup are saved.
	VSS_BT_LOG                           // The log file of a writer is to participate in backup or restore operations.
	VSS_BT_COPY                          // Files on disk will be copied to a backup medium regardless of the state of each file's backup history, and the backup history will not be updated.
	VSS_BT_OTHER                         // Backup type that is not full, copy, log, incremental, or differential.
)

//NOTE: Microsoft Documentation can be found here: https://docs.microsoft.com/en-us/windows/win32/api/vss/ne-vss-vss_object_type

// The VssObjectType enumeration is used by requesters to identify an object as a shadow copy set, shadow copy, or provider.
type VssObjectType uint

const (
	VSS_OBJECT_UNKNOWN      VssObjectType = iota // The object type is not known. This indicates an application error.
	VSS_OBJECT_NONE                              // If returned as output, indicates application error.
	VSS_OBJECT_SNAPSHOT_SET                      // Shadow copy set.
	VSS_OBJECT_SNAPSHOT                          // Shadow Copy
	VSS_OBJECT_PROVIDER                          // Shadow Copy Provider
	VSS_OBJECT_TYPE_COUNT                        // Reserved Value
)

//NOTE: Microsoft Documentation can be found here: https://docs.microsoft.com/en-us/windows/win32/api/vss/ne-vss-vss_snapshot_state

// The VSS_SNAPSHOT_STATE enumeration is returned by a provider to specify the state of a given shadow copy operation.
type VSS_SNAPSHOT_STATE uint32

const (
	VSS_SS_UNKNOWN VSS_SNAPSHOT_STATE = iota
	VSS_SS_PREPARING
	VSS_SS_PROCESSING_PREPARE
	VSS_SS_PREPARED
	VSS_SS_PROCESSING_PRECOMMIT
	VSS_SS_PRECOMMITTED
	VSS_SS_PROCESSING_COMMIT
	VSS_SS_COMMITTED
	VSS_SS_PROCESSING_POSTCOMMIT
	VSS_SS_PROCESSING_PREFINALCOMMIT
	VSS_SS_PREFINALCOMMITTED
	VSS_SS_PROCESSING_POSTFINALCOMMIT
	VSS_SS_CREATED
	VSS_SS_ABORTED
	VSS_SS_DELETED
	VSS_SS_POSTCOMMITTED
	VSS_SS_COUNT
)

var snapStateToString = map[VSS_SNAPSHOT_STATE]string{
	VSS_SS_UNKNOWN:                    "VSS_SS_UNKNOWN - Unknown shadow copy state (Reserved for system use)",
	VSS_SS_PREPARING:                  "VSS_SS_PREPARING - Shadow copy is being prepared (Reserved for system use)",
	VSS_SS_PROCESSING_PREPARE:         "VSS_SS_PROCESSING_PREPARE - Processing of the shadow copy preparation is in progress (Reserved for system use)",
	VSS_SS_PREPARED:                   "VSS_SS_PREPARED - Shadow copy has been prepared (Reserved for system use)",
	VSS_SS_PROCESSING_PRECOMMIT:       "VSS_SS_PROCESSING_PRECOMMIT - Processing of the shadow copy precommit is in process (Reserved for system use)",
	VSS_SS_PRECOMMITTED:               "VSS_SS_PRECOMMITTED - Shadow copy is precommitted (Reserved for system use)",
	VSS_SS_PROCESSING_COMMIT:          "VSS_SS_PROCESSING_COMMIT - Processing of the shadow copy commit is in process (Reserved for system use)",
	VSS_SS_COMMITTED:                  "VSS_SS_COMMITTED - Shadow copy is committed (Reserved for system use)",
	VSS_SS_PROCESSING_POSTCOMMIT:      "VSS_SS_PROCESSING_POSTCOMMIT - Processing of the shadow copy postcommit is in process (Reserved for system use)",
	VSS_SS_PROCESSING_PREFINALCOMMIT:  "VSS_SS_PROCESSING_PREFINALCOMMIT - Processing of the shadow copy file commit operation is underway (Reserved for system use)",
	VSS_SS_PREFINALCOMMITTED:          "VSS_SS_PREFINALCOMMITTED - Processing of the shadow copy file commit operation is done (Reserved for system use)",
	VSS_SS_PROCESSING_POSTFINALCOMMIT: "VSS_SS_PROCESSING_POSTFINALCOMMIT - Processing of the shadow copy following the final commit and prior to shadow copy create is underway (Reserved for system use)",
	VSS_SS_CREATED:                    "VSS_SS_CREATED - Shadow copy is created.",
	VSS_SS_ABORTED:                    "VSS_SS_ABORTED - Shadow copy creation is aborted (Reserved for system use)",
	VSS_SS_DELETED:                    "VSS_SS_DELETED - Shadow copy has been deleted (Reserved for system use)",
	VSS_SS_POSTCOMMITTED:              "VSS_SS_POSTCOMMITTED",
	VSS_SS_COUNT:                      "VSS_SS_COUNT (Reserved value)",
}

func (state VSS_SNAPSHOT_STATE) String() string {
	if i, ok := snapStateToString[state]; ok {
		return i
	}
	return fmt.Sprintf("UNKNOWN VSS_SNAPSHOT_STATE Error Code: %d", state)
}

type VSS_GUID uint

func (guid VSS_GUID) Value() (uintptr, uintptr, uintptr, uintptr) {
	return uintptr(*(*uint32)(unsafe.Pointer(uintptr(guid)))),
		uintptr(*(*uint32)(unsafe.Pointer(uintptr(guid + 4)))),
		uintptr(*(*uint32)(unsafe.Pointer(uintptr(guid + 8)))),
		uintptr(*(*uint32)(unsafe.Pointer(uintptr(guid + 12))))
}

type Snapshot struct {
	Id               string
	Drive            string
	Details          SnapshotDetails
	DeviceObjectPath string
}

func (s Snapshot) Validate() error {
	if s.Id == "" {
		return fmt.Errorf("snapshot is missing Id property")
	}
	if s.DeviceObjectPath == "" {
		return errors.New("snapshot is missing DeviceObjectPath property")
	}
	return nil
}

type SnapshotDetails struct {
	Id                 string
	ProviderID         string
	Status             string
	DeviceObject       string
	VolumeName         string
	OriginatingMachine string
	ServiceMachine     string
	ExposedName        string
	Attributes         interface{}
	InstallDate        time.Time
}

func (d SnapshotDetails) Validate() error {
	if d.Id == "" {
		return fmt.Errorf("snapshot details are missing ShadowCopyId property")
	}
	if d.ProviderID == "" {
		return fmt.Errorf("snapshot details are missing Provider property")
	}
	if d.Status == "" {
		return fmt.Errorf("snapshot details are missing Status property")
	}
	if d.DeviceObject == "" {
		return fmt.Errorf("snapshot details are missing ShadowCopyVolume property")
	}
	if d.VolumeName == "" {
		return fmt.Errorf("snapshot details are missing OriginalVolume property")
	}
	if d.OriginatingMachine == "" {
		return fmt.Errorf("snapshot details are missing OriginatingMachine property")
	}
	if d.ServiceMachine == "" {
		return fmt.Errorf("snapshot details are missing ServiceMachine property")
	}
	if d.ExposedName == "" {
		return fmt.Errorf("snapshot details are missing ExposedName property")
	}
	if d.Attributes == nil {
		return fmt.Errorf("snapshot details are missing Attributes property")
	}
	if d.InstallDate.IsZero() {
		return fmt.Errorf("snapshot details are missing InstallDate property")
	}
	return nil
}

type ISnapshotter interface {
	CreateSnapshot(drive string, timeout int, force bool) (*Snapshot, error)
	Details(id string) (*Snapshot, error)
	DeleteSnapshot(id string) error
}
