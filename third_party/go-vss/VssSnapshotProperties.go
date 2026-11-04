package vss

import (
	"time"

	ole "github.com/go-ole/go-ole"
)

//NOTE: Microsoft Documentation can be found here: https://docs.microsoft.com/en-us/windows/win32/api/vss/ns-vss-vss_snapshot_prop

// The VssSnapshotProperties structure contains the properties of a shadow copy or shadow copy set.
type VssSnapshotProperties struct {
	snapshotID           ole.GUID // GUID uniquely identifying the shadow copy identifier.
	snapshotSetID        ole.GUID // GUID uniquely identifying the shadow copy set containing the shadow copy.
	snapshotsCount       uint32   // Number of volumes included with the shadow copy in the shadow copy set when it was created.
	snapshotDeviceObject *uint16  // Requesters will use this device name when accessing files on a shadow-copied volume that it needs to work with.
	originalVolumeName   *uint16  // Name of the volume that had been shadow copied
	originatingMachine   *uint16  // Name of the machine containing the original volume
	serviceMachine       *uint16  // Name of the machine running the Volume Shadow Copy Service that created the shadow copy
	exposedName          *uint16  // Name of the shadow copy when it is exposed.
	exposedPath          *uint16  // Indicates the portion of the shadow copy of a volume made available if it is exposed as a share.
	providerID           ole.GUID // GUID uniquely identifying the provider used to create this shadow copy.
	snapshotAttributes   uint32   // Attributes of the shadow copy expressed as a bit mask
	creationTimestamp    uint64   // Time stamp indicating when the shadow copy was created.
	status               uint     // Current shadow copy creation status.
}

// Returns the SnapshotId
func (p *VssSnapshotProperties) GetSnapshotId() string {
	return p.snapshotID.String()
}

// Returns the SnapshotSetId
func (p *VssSnapshotProperties) GetSnapshotSetId() string {
	return p.snapshotSetID.String()
}

// Returns the SnapshotsCount
func (p *VssSnapshotProperties) GetSnapshotsCount() int {
	return int(p.snapshotsCount)
}

// Returns the SnapshotDeviceObject
func (p *VssSnapshotProperties) GetSnapshotDeviceObject() string {
	return ole.UTF16PtrToString(p.snapshotDeviceObject)
}

// Returns the OriginalVolumeName
func (p *VssSnapshotProperties) GetOriginalVolume() string {
	return ole.UTF16PtrToString(p.originalVolumeName)
}

// Returns the OriginatingMachine
func (p *VssSnapshotProperties) GetOriginatingMachine() string {
	return ole.UTF16PtrToString(p.originatingMachine)
}

// Returns the ServiceMachine
func (p *VssSnapshotProperties) GetServiceMachine() string {
	return ole.UTF16PtrToString(p.serviceMachine)
}

// Returns the ExposedName
func (p *VssSnapshotProperties) GetExposedName() string {
	return ole.UTF16PtrToString(p.exposedName)
}

// Returns the ExposedPath
func (p *VssSnapshotProperties) GetExposedPath() string {
	return ole.UTF16PtrToString(p.exposedPath)
}

// Returns the ProviderId
func (p *VssSnapshotProperties) GetProviderId() string {
	return p.providerID.String()
}

// Returns the SnapshotAttributes
func (p *VssSnapshotProperties) GetSnapshotAttributes() VSS_VOLUME_SNAPSHOT_ATTRIBUTES {
	return parseAttributesBitmask(int32(p.snapshotAttributes))
}

// Returns the SnapshotAttributes as a descriptive list
func (p *VssSnapshotProperties) GetSnapshotAttributesVerbose() []string {
	return parseAttributesBitmask(int32(p.snapshotAttributes)).Verbose()
}

// Returns the CreationTimeStamp
func (p *VssSnapshotProperties) GetCreationTimeStamp() time.Time {
	return convertWindowsTimeToUnixTime(p.creationTimestamp)
}

// Returns the SnapshotStatus
func (p *VssSnapshotProperties) GetSnapshotStatus() string {
	return VSS_SNAPSHOT_STATE(p.status).String()
}

// convertWindowsTimeToUnixTime converts Windows FileTime to time.Time
func convertWindowsTimeToUnixTime(winTime uint64) time.Time {
	const TICKS_PER_SECOND = int64(10000000)    // factor from seconds to 100ns
	const EPOCH_DIFFERENCE = int64(11644473600) // Difference between January 1, 1601 (UTC) and unix start time

	t := int64(winTime)
	temp := t / TICKS_PER_SECOND   //convert from 100ns intervals to seconds;
	temp = temp - EPOCH_DIFFERENCE //subtract number of seconds between epochs
	return time.Unix(temp, 0)
}

func ParseProperties(p VssSnapshotProperties) (SnapshotDetails, error) {
	return SnapshotDetails{
		Id:                 p.GetSnapshotId(),
		ProviderID:         p.GetProviderId(),
		Status:             p.GetSnapshotStatus(),
		DeviceObject:       p.GetSnapshotDeviceObject(),
		VolumeName:         p.GetOriginalVolume(),
		OriginatingMachine: p.GetOriginatingMachine(),
		ServiceMachine:     p.GetServiceMachine(),
		ExposedName:        p.GetExposedName(),
		Attributes:         p.GetSnapshotAttributes(),
		InstallDate:        p.GetCreationTimeStamp(),
	}, nil
}
