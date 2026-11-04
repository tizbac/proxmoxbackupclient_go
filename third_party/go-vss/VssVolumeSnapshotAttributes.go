package vss

//NOTE: Microsoft Documentation can be found here: https://docs.microsoft.com/en-us/windows/win32/api/vss/ne-vss-vss_volume_snapshot_attributes

type VSS_VOLUME_SNAPSHOT_ATTRIBUTES struct {
	Persistent          bool // The shadow copy is persistent across reboots.
	NoAutoRecovery      bool // Auto-recovery is disabled for the shadow copy.
	ClientAccessible    bool // The specified shadow copy is a client-accessible shadow copy that supports Shadow Copies for Shared Folders, and should not be exposed.
	NoAutoRelease       bool // The shadow copy is not automatically deleted when the shadow copy requester process ends.
	NoWriters           bool // No writers are involved in creating the shadow copy.
	Transportable       bool // The shadow copy is to be transported and therefore should not be surfaced locally.
	NotSurfaced         bool // The shadow copy is not currently exposed.
	NotTransacted       bool // The shadow copy is not transacted.
	HardwareAssisted    bool // Indicates that a given provider is a hardware provider.
	Differential        bool // Indicates that a given provider uses differential data or a copy-on-write mechanism to implement shadow copies.
	Plex                bool // Indicates that a given provider uses a PLEX or mirrored split mechanism to implement shadow copies.
	Imported            bool // The shadow copy of the volume was imported onto this machine using the IVssBackupComponents::ImportSnapshots method rather than created using the IVssBackupComponents::DoSnapshotSet method.
	ExposedLocally      bool // The shadow copy is locally exposed. If this and ExposedRemotely isn't set the shadow copy is hidden
	ExposedRemotely     bool // The shadow copy is remotely exposed.
	AutoRecover         bool // Indicates that the writer will need to auto-recover the component in CVssWriter::OnPostSnapshot.
	RollbackRecovery    bool // Indicates that the writer will need to auto-recover the component in CVssWriter::OnPostSnapshot if the shadow copy is being used for rollback.
	DelayedPostSnapshot bool // Reserved for system use.
	TXFRecovery         bool // Indicates that TxF recovery should be enforced during shadow copy creation.
	FileShare           bool // -
}

const (
	VSS_VOLSNAP_ATTR_PERSISTENT int32 = 1 << iota
	VSS_VOLSNAP_ATTR_NO_AUTORECOVERY
	VSS_VOLSNAP_ATTR_CLIENT_ACCESSIBLE
	VSS_VOLSNAP_ATTR_NO_AUTO_RELEASE
	VSS_VOLSNAP_ATTR_NO_WRITERS
	VSS_VOLSNAP_ATTR_TRANSPORTABLE
	VSS_VOLSNAP_ATTR_NOT_SURFACED
	VSS_VOLSNAP_ATTR_NOT_TRANSACTED
	VSS_VOLSNAP_ATTR_HARDWARE_ASSISTED
	VSS_VOLSNAP_ATTR_DIFFERENTIAL
	VSS_VOLSNAP_ATTR_PLEX
	VSS_VOLSNAP_ATTR_IMPORTED
	VSS_VOLSNAP_ATTR_EXPOSED_LOCALLY
	VSS_VOLSNAP_ATTR_EXPOSED_REMOTELY
	VSS_VOLSNAP_ATTR_AUTORECOVER
	VSS_VOLSNAP_ATTR_ROLLBACK_RECOVERY
	VSS_VOLSNAP_ATTR_DELAYED_POSTSNAPSHOT
	VSS_VOLSNAP_ATTR_TXF_RECOVERY
	VSS_VOLSNAP_ATTR_FILE_SHARE
)

func parseAttributesBitmask(code int32) VSS_VOLUME_SNAPSHOT_ATTRIBUTES {
	return VSS_VOLUME_SNAPSHOT_ATTRIBUTES{
		Persistent:          code&VSS_VOLSNAP_ATTR_PERSISTENT != 0,
		NoAutoRecovery:      code&VSS_VOLSNAP_ATTR_NO_AUTORECOVERY != 0,
		ClientAccessible:    code&VSS_VOLSNAP_ATTR_CLIENT_ACCESSIBLE != 0,
		NoAutoRelease:       code&VSS_VOLSNAP_ATTR_NO_AUTO_RELEASE != 0,
		NoWriters:           code&VSS_VOLSNAP_ATTR_NO_WRITERS != 0,
		Transportable:       code&VSS_VOLSNAP_ATTR_TRANSPORTABLE != 0,
		NotSurfaced:         code&VSS_VOLSNAP_ATTR_NOT_SURFACED != 0,
		NotTransacted:       code&VSS_VOLSNAP_ATTR_NOT_TRANSACTED != 0,
		HardwareAssisted:    code&VSS_VOLSNAP_ATTR_HARDWARE_ASSISTED != 0,
		Differential:        code&VSS_VOLSNAP_ATTR_DIFFERENTIAL != 0,
		Plex:                code&VSS_VOLSNAP_ATTR_PLEX != 0,
		Imported:            code&VSS_VOLSNAP_ATTR_IMPORTED != 0,
		ExposedLocally:      code&VSS_VOLSNAP_ATTR_EXPOSED_LOCALLY != 0,
		ExposedRemotely:     code&VSS_VOLSNAP_ATTR_EXPOSED_REMOTELY != 0,
		AutoRecover:         code&VSS_VOLSNAP_ATTR_AUTORECOVER != 0,
		RollbackRecovery:    code&VSS_VOLSNAP_ATTR_ROLLBACK_RECOVERY != 0,
		DelayedPostSnapshot: code&VSS_VOLSNAP_ATTR_DELAYED_POSTSNAPSHOT != 0,
		TXFRecovery:         code&VSS_VOLSNAP_ATTR_TXF_RECOVERY != 0,
		FileShare:           code&VSS_VOLSNAP_ATTR_FILE_SHARE != 0,
	}
}

// TODO: Possibility to add more attributes
func (a VSS_VOLUME_SNAPSHOT_ATTRIBUTES) Verbose() []string {
	var attributes []string
	if a.Persistent {
		attributes = append(attributes, "Persistent")
	} else {
		attributes = append(attributes, "Non-Persistent")
	}
	if a.NoAutoRecovery {
		attributes = append(attributes, "No Autorecovery")
	} else {
		attributes = append(attributes, "Auto recovered")
	}
	if a.TXFRecovery {
		attributes = append(attributes, "TxF recovery enforced")
	}
	return attributes
}
