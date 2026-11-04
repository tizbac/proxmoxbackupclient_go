package vss

import "fmt"

type HRESULT uint

const (
	S_OK                                            HRESULT = 0x00000000
	E_ACCESSDENIED                                  HRESULT = 0x80070005
	E_OUTOFMEMORY                                   HRESULT = 0x8007000E
	E_INVALIDARG                                    HRESULT = 0x80070057
	VSS_E_BAD_STATE                                 HRESULT = 0x80042301
	VSS_E_UNEXPECTED                                HRESULT = 0x80042302
	VSS_E_PROVIDER_ALREADY_REGISTERED               HRESULT = 0x80042303
	VSS_E_PROVIDER_NOT_REGISTERED                   HRESULT = 0x80042304
	VSS_E_PROVIDER_VETO                             HRESULT = 0x80042306
	VSS_E_PROVIDER_IN_USE                           HRESULT = 0x80042307
	VSS_E_OBJECT_NOT_FOUND                          HRESULT = 0x80042308
	VSS_S_ASYNC_PENDING                             HRESULT = 0x00042309
	VSS_S_ASYNC_FINISHED                            HRESULT = 0x0004230A
	VSS_S_ASYNC_CANCELLED                           HRESULT = 0x0004230B
	VSS_E_VOLUME_NOT_SUPPORTED                      HRESULT = 0x8004230C
	VSS_E_VOLUME_NOT_SUPPORTED_BY_PROVIDER          HRESULT = 0x8004230E
	VSS_E_OBJECT_ALREADY_EXISTS                     HRESULT = 0x8004230D
	VSS_E_UNEXPECTED_PROVIDER_ERROR                 HRESULT = 0x8004230F
	VSS_E_CORRUPT_XML_DOCUMENT                      HRESULT = 0x80042310
	VSS_E_INVALID_XML_DOCUMENT                      HRESULT = 0x80042311
	VSS_E_MAXIMUM_NUMBER_OF_VOLUMES_REACHED         HRESULT = 0x80042312
	VSS_E_FLUSH_WRITES_TIMEOUT                      HRESULT = 0x80042313
	VSS_E_HOLD_WRITES_TIMEOUT                       HRESULT = 0x80042314
	VSS_E_UNEXPECTED_WRITER_ERROR                   HRESULT = 0x80042315
	VSS_E_SNAPSHOT_SET_IN_PROGRESS                  HRESULT = 0x80042316
	VSS_E_MAXIMUM_NUMBER_OF_SNAPSHOTS_REACHED       HRESULT = 0x80042317
	VSS_E_WRITER_INFRASTRUCTURE                     HRESULT = 0x80042318
	VSS_E_WRITER_NOT_RESPONDING                     HRESULT = 0x80042319
	VSS_E_WRITER_ALREADY_SUBSCRIBED                 HRESULT = 0x8004231A
	VSS_E_UNSUPPORTED_CONTEXT                       HRESULT = 0x8004231B
	VSS_E_VOLUME_IN_USE                             HRESULT = 0x8004231D
	VSS_E_MAXIMUM_DIFFAREA_ASSOCIATIONS_REACHED     HRESULT = 0x8004231E
	VSS_E_INSUFFICIENT_STORAGE                      HRESULT = 0x8004231F
	VSS_E_NO_SNAPSHOTS_IMPORTED                     HRESULT = 0x80042320
	VSS_E_SOME_SNAPSHOTS_NOT_IMPORTED               HRESULT = 0x80042321
	VSS_E_MAXIMUM_NUMBER_OF_REMOTE_MACHINES_REACHED HRESULT = 0x80042322
	VSS_E_REMOTE_SERVER_UNAVAILABLE                 HRESULT = 0x80042323
	VSS_E_REMOTE_SERVER_UNSUPPORTED                 HRESULT = 0x80042324
	VSS_E_REVERT_IN_PROGRESS                        HRESULT = 0x80042325
	VSS_E_REVERT_VOLUME_LOST                        HRESULT = 0x80042326
	VSS_E_REBOOT_REQUIRED                           HRESULT = 0x80042327
	VSS_E_TRANSACTION_FREEZE_TIMEOUT                HRESULT = 0x80042328
	VSS_E_TRANSACTION_THAW_TIMEOUT                  HRESULT = 0x80042329
	VSS_E_VOLUME_NOT_LOCAL                          HRESULT = 0x8004232D
	VSS_E_CLUSTER_TIMEOUT                           HRESULT = 0x8004232E
	VSS_E_WRITERERROR_INCONSISTENTSNAPSHOT          HRESULT = 0x800423F0
	VSS_E_WRITERERROR_OUTOFRESOURCES                HRESULT = 0x800423F1
	VSS_E_WRITERERROR_TIMEOUT                       HRESULT = 0x800423F2
	VSS_E_WRITERERROR_RETRYABLE                     HRESULT = 0x800423F3
	VSS_E_WRITERERROR_NONRETRYABLE                  HRESULT = 0x800423F4
	VSS_E_WRITERERROR_RECOVERY_FAILED               HRESULT = 0x800423F5
	VSS_E_BREAK_REVERT_ID_FAILED                    HRESULT = 0x800423F6
	VSS_E_LEGACY_PROVIDER                           HRESULT = 0x800423F7
	VSS_E_MISSING_DISK                              HRESULT = 0x800423F8
	VSS_E_MISSING_HIDDEN_VOLUME                     HRESULT = 0x800423F9
	VSS_E_MISSING_VOLUME                            HRESULT = 0x800423FA
	VSS_E_AUTORECOVERY_FAILED                       HRESULT = 0x800423FB
	VSS_E_DYNAMIC_DISK_ERROR                        HRESULT = 0x800423FC
	VSS_E_NONTRANSPORTABLE_BCD                      HRESULT = 0x800423FD
	VSS_E_CANNOT_REVERT_DISKID                      HRESULT = 0x800423FE
	VSS_E_RESYNC_IN_PROGRESS                        HRESULT = 0x800423FF
	VSS_E_CLUSTER_ERROR                             HRESULT = 0x80042400
	VSS_E_UNSELECTED_VOLUME                         HRESULT = 0x8004232A
	VSS_E_SNAPSHOT_NOT_IN_SET                       HRESULT = 0x8004232B
	VSS_E_NESTED_VOLUME_LIMIT                       HRESULT = 0x8004232C
	VSS_E_NOT_SUPPORTED                             HRESULT = 0x8004232F
	VSS_E_WRITERERROR_PARTIAL_FAILURE               HRESULT = 0x80042336
	VSS_E_WRITER_STATUS_NOT_AVAILABLE               HRESULT = 0x80042409
)

var errorToString = map[HRESULT]string{
	S_OK:                                            "S_OK",
	E_ACCESSDENIED:                                  "E_ACCESSDENIED",
	E_OUTOFMEMORY:                                   "E_OUTOFMEMORY",
	E_INVALIDARG:                                    "E_INVALIDARG",
	VSS_S_ASYNC_PENDING:                             "The asynchronous operation is pending.",
	VSS_S_ASYNC_FINISHED:                            "The asynchronous operation has completed.",
	VSS_S_ASYNC_CANCELLED:                           "The asynchronous operation has been cancelled.",
	VSS_E_BAD_STATE:                                 "VSS_E_BAD_STATE - A function call was made when the object was in an incorrect state. This indicates that Microsoft's VSS framework and/or perhaps some of the VSS writers are in a bad state.",
	VSS_E_UNEXPECTED:                                "VSS_E_UNEXPECTED - A volume shadow copy service (VSS) component encountered an unexpected error.",
	VSS_E_PROVIDER_ALREADY_REGISTERED:               "VSS_E_PROVIDER_ALREADY_REGISTERED - The volume shadow copy provider is already registered in the system.",
	VSS_E_PROVIDER_NOT_REGISTERED:                   "VSS_E_PROVIDER_NOT_REGISTERED - The volume shadow copy provider is not registered in the system.",
	VSS_E_PROVIDER_VETO:                             "VSS_E_PROVIDER_VETO - The shadow copy provider had an error. The provider was unable to perform the request at this time. This can be a transient problem. It is recommended to wait ten minutes and try again, up to three times.",
	VSS_E_PROVIDER_IN_USE:                           "VSS_E_PROVIDER_IN_USE - The shadow copy provider is currently in use and cannot be unregistered.",
	VSS_E_OBJECT_NOT_FOUND:                          "VSS_E_OBJECT_NOT_FOUND - The specified object was not found. This error means that Microsoft VSS failed to take a snapshot of your file systems and that the backup job will be unable to backup any files that are opened exclusively by other applications. The most common cause of this error is that VSS has been disabled on one or more of the volumes that are part of the backup.",
	VSS_E_VOLUME_NOT_SUPPORTED:                      "VSS_E_VOLUME_NOT_SUPPORTED - Shadow copying the specified volume is not supported.",
	VSS_E_VOLUME_NOT_SUPPORTED_BY_PROVIDER:          "VSS_E_VOLUME_NOT_SUPPORTED_BY_PROVIDER - The given shadow copy provider does not support shadow copying the specified volume.",
	VSS_E_OBJECT_ALREADY_EXISTS:                     "VSS_E_OBJECT_ALREADY_EXISTS - The object already exists.",
	VSS_E_UNEXPECTED_PROVIDER_ERROR:                 "VSS_E_UNEXPECTED_PROVIDER_ERROR - The shadow copy provider had an unexpected error while trying to process the specified operation.",
	VSS_E_CORRUPT_XML_DOCUMENT:                      "VSS_E_CORRUPT_XML_DOCUMENT - The given XML document is invalid. It is either incorrectly-formed XML or it does not match the schema.",
	VSS_E_INVALID_XML_DOCUMENT:                      "VSS_E_INVALID_XML_DOCUMENT - The given XML document is invalid. It is either incorrectly-formed XML or it does not match the schema.",
	VSS_E_MAXIMUM_NUMBER_OF_VOLUMES_REACHED:         "VSS_E_MAXIMUM_NUMBER_OF_VOLUMES_REACHED - The maximum number of volumes for this operation has been reached.",
	VSS_E_FLUSH_WRITES_TIMEOUT:                      "VSS_E_FLUSH_WRITES_TIMEOUT - The shadow copy provider timed out while flushing data to the volume being shadow copied. This is probably due to excessive activity on the volume. Try again later when the volume is not being used so heavily.",
	VSS_E_HOLD_WRITES_TIMEOUT:                       "VSS_E_HOLD_WRITES_TIMEOUT - The shadow copy provider timed out while holding writes to the volume being shadow copied. This is probably due to excessive activity on the volume by an application or a system service. Try again later when activity on the volume is reduced.",
	VSS_E_UNEXPECTED_WRITER_ERROR:                   "VSS_E_UNEXPECTED_WRITER_ERROR - VSS encountered problems while sending events to writers.",
	VSS_E_SNAPSHOT_SET_IN_PROGRESS:                  "VSS_E_SNAPSHOT_SET_IN_PROGRESS - Another shadow copy creation is already in progress. Wait a few moments and try again.",
	VSS_E_MAXIMUM_NUMBER_OF_SNAPSHOTS_REACHED:       "VSS_E_MAXIMUM_NUMBER_OF_SNAPSHOTS_REACHED - The specified volume has already reached its maximum number of shadow copies. The volume has been added to the maximum number of shadow copy sets. The specified volume was not added to the shadow copy set. Other possible reasons: There is not enough free disk space on the drive where the locked file is located. Delete temporary files, empty recycle bin, etc. Fow WinXP, VSS has the limitation that only one shadow volume can be created per drive at a time. There could be another software that is already using the shadow volume for the drive. Restart your computer and try again.",
	VSS_E_WRITER_INFRASTRUCTURE:                     "VSS_E_WRITER_INFRASTRUCTURE - An error was detected in the VSS. The problem occurred while trying to contact VSS writers.",
	VSS_E_WRITER_NOT_RESPONDING:                     "VSS_E_WRITER_NOT_RESPONDING - A writer did not respond to a GatherWriterStatus call. The writer might have terminated, or it might be stuck.",
	VSS_E_WRITER_ALREADY_SUBSCRIBED:                 "VSS_E_WRITER_ALREADY_SUBSCRIBED - The writer has already successfully called the Subscribe function. It cannot call Subscribe multiple times.",
	VSS_E_UNSUPPORTED_CONTEXT:                       "VSS_E_UNSUPPORTED_CONTEXT - The shadow copy provider does not support the specified shadow copy type.",
	VSS_E_VOLUME_IN_USE:                             "VSS_E_VOLUME_IN_USE - The specified shadow copy storage association is in use and so can't be deleted.",
	VSS_E_MAXIMUM_DIFFAREA_ASSOCIATIONS_REACHED:     "VSS_E_MAXIMUM_DIFFAREA_ASSOCIATIONS_REACHED - Maximum number of shadow copy storage associations already reached.",
	VSS_E_INSUFFICIENT_STORAGE:                      "VSS_E_INSUFFICIENT_STORAGE - Insufficient storage available to create either the shadow copy storage file or other shadow copy data.",
	VSS_E_NO_SNAPSHOTS_IMPORTED:                     "VSS_E_NO_SNAPSHOTS_IMPORTED - No shadow copies were successfully imported.",
	VSS_E_SOME_SNAPSHOTS_NOT_IMPORTED:               "VSS_E_SOME_SNAPSHOTS_NOT_IMPORTED - Some shadow copies were not successfully imported.",
	VSS_E_MAXIMUM_NUMBER_OF_REMOTE_MACHINES_REACHED: "VSS_E_MAXIMUM_NUMBER_OF_REMOTE_MACHINES_REACHED - The maximum number of remote machines for this operation has been reached.",
	VSS_E_REMOTE_SERVER_UNAVAILABLE:                 "VSS_E_REMOTE_SERVER_UNAVAILABLE - The remote server is unavailable.",
	VSS_E_REMOTE_SERVER_UNSUPPORTED:                 "VSS_E_REMOTE_SERVER_UNSUPPORTED - The remote server is running a version of the Volume Shadow Copy Service that does not support remote shadow-copy creation.",
	VSS_E_REVERT_IN_PROGRESS:                        "VSS_E_REVERT_IN_PROGRESS - A revert is currently in progress for the specified volume.  Another revert cannot be initiated until the current revert completes.",
	VSS_E_REVERT_VOLUME_LOST:                        "VSS_E_REVERT_VOLUME_LOST - The volume being reverted was lost during revert.",
	VSS_E_REBOOT_REQUIRED:                           "VSS_E_REBOOT_REQUIRED - A reboot is required after completing this operation.",
	VSS_E_TRANSACTION_FREEZE_TIMEOUT:                "VSS_E_TRANSACTION_FREEZE_TIMEOUT - A timeout occurred while freezing a transaction manager.",
	VSS_E_TRANSACTION_THAW_TIMEOUT:                  "VSS_E_TRANSACTION_THAW_TIMEOUT - Too much time elapsed between freezing a transaction manager and thawing the transaction manager.",
	VSS_E_VOLUME_NOT_LOCAL:                          "VSS_E_VOLUME_NOT_LOCAL - The volume being backed up is not mounted on the local host.",
	VSS_E_CLUSTER_TIMEOUT:                           "VSS_E_CLUSTER_TIMEOUT - A timeout occurred while preparing a cluster shared volume for backup.",
	VSS_E_WRITERERROR_INCONSISTENTSNAPSHOT:          "VSS_E_WRITERERROR_INCONSISTENTSNAPSHOT - The shadow copy set only contains only a subset of the volumes needed to correctly backup the selected components of the writer.",
	VSS_E_WRITERERROR_OUTOFRESOURCES:                "VSS_E_WRITERERROR_OUTOFRESOURCES - A resource allocation failed while processing this operation.",
	VSS_E_WRITERERROR_TIMEOUT:                       "VSS_E_WRITERERROR_TIMEOUT - The writer's timeout expired between the Freeze and Thaw events.",
	VSS_E_WRITERERROR_RETRYABLE:                     "VSS_E_WRITERERROR_RETRYABLE - The writer experienced a transient error. If the backup process is retried, the error might not reoccur.",
	VSS_E_WRITERERROR_NONRETRYABLE:                  "VSS_E_WRITERERROR_NONRETRYABLE - The writer experienced a non-transient error. If the backup process is retried, the error is likely to reoccur.",
	VSS_E_WRITERERROR_RECOVERY_FAILED:               "VSS_E_WRITERERROR_RECOVERY_FAILED - The writer experienced an error while trying to recover the shadow copy volume.",
	VSS_E_BREAK_REVERT_ID_FAILED:                    "VSS_E_BREAK_REVERT_ID_FAILED - The shadow copy set break operation failed because the disk/partition identities could not be reverted. The target identity already exists on the machine or cluster and must be masked before this operation can succeed.",
	VSS_E_LEGACY_PROVIDER:                           "VSS_E_LEGACY_PROVIDER - This version of the hardware provider does not support this operation.",
	VSS_E_MISSING_DISK:                              "VSS_E_MISSING_DISK - An expected disk did not arrive in the system.",
	VSS_E_MISSING_HIDDEN_VOLUME:                     "VSS_E_MISSING_HIDDEN_VOLUME - An expected hidden volume did not arrive in the system. Check the Application event log for more information.",
	VSS_E_MISSING_VOLUME:                            "VSS_E_MISSING_VOLUME - An expected volume did not arrive in the system. Check the Application event log for more information.",
	VSS_E_AUTORECOVERY_FAILED:                       "VSS_E_AUTORECOVERY_FAILED - The autorecovery operation failed to complete on the shadow copy.",
	VSS_E_DYNAMIC_DISK_ERROR:                        "VSS_E_DYNAMIC_DISK_ERROR - An error occurred in processing the dynamic disks involved in the operation.",
	VSS_E_NONTRANSPORTABLE_BCD:                      "VSS_E_NONTRANSPORTABLE_BCD - The given Backup Components Document is for a non-transportable shadow copy. This operation can only be done on transportable shadow copies.",
	VSS_E_CANNOT_REVERT_DISKID:                      "VSS_E_CANNOT_REVERT_DISKID - The MBR signature or GPT ID for one or more disks could not be set to the intended value. Check the Application event log for more information.",
	VSS_E_RESYNC_IN_PROGRESS:                        "VSS_E_RESYNC_IN_PROGRESS - The LUN resynchronization operation could not be started because another resynchronization operation is already in progress.",
	VSS_E_CLUSTER_ERROR:                             "VSS_E_CLUSTER_ERROR - The clustered disks could not be enumerated or could not be put into cluster maintenance mode. Check the System event log for cluster related events and the Application event log for VSS related events.",
	VSS_E_UNSELECTED_VOLUME:                         "VSS_E_UNSELECTED_VOLUME - The requested operation would overwrite a volume that is not explicitly selected. For more information, check the Application event log.",
	VSS_E_SNAPSHOT_NOT_IN_SET:                       "VSS_E_SNAPSHOT_NOT_IN_SET - The shadow copy ID was not found in the backup components document for the shadow copy set.",
	VSS_E_NESTED_VOLUME_LIMIT:                       "VSS_E_NESTED_VOLUME_LIMIT - The specified volume is nested too deeply to participate in the VSS operation.",
	VSS_E_NOT_SUPPORTED:                             "VSS_E_NOT_SUPPORTED - The requested operation is not supported.",
	VSS_E_WRITERERROR_PARTIAL_FAILURE:               "VSS_E_WRITERERROR_PARTIAL_FAILURE - The writer experienced a partial failure. Check the component level error state for more information.",
	VSS_E_WRITER_STATUS_NOT_AVAILABLE:               "VSS_E_WRITER_STATUS_NOT_AVAILABLE - Writer status is not available for one or more writers. A writer might have reached the limit to the number of available backup-restore session states.",
}

// Str converts a HRESULT to a human readable string.
func (h HRESULT) String() string {
	if i, ok := errorToString[h]; ok {
		return i
	}
	return fmt.Sprintf("UNKNOWN error with code: %d", h)
}

// VssError encapsulates errors retruned from calling VSS api.
type VssError struct {
	text    string
	hresult HRESULT
}

func (err *VssError) Error() string {
	return fmt.Sprintf("VSS error: %s: %s (%#x)", err.text, err.hresult.String(), err.hresult)
}

func createVssError(text string, hresult HRESULT) error {
	return &VssError{text: text, hresult: hresult}
}

func CreateVSSError(text string, code uintptr) error {
	hresult := HRESULT(code)
	if hresult != S_OK {
		return createVssError(text, hresult)
	}
	return nil
}
