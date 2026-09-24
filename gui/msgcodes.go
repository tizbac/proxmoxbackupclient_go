package main

// MessageKey identifies a translatable backend-generated message shown in the
// Job History and Message Log tabs. Those two lists used to store only a
// pre-rendered English string (JobHistory.Message / MessageLogEntry.Message),
// so every other language in gui/frontend/src/i18n/translations.js covered
// the rest of the UI but not these — the backend was baking English text
// straight into persisted, displayed data instead of going through the
// frontend's own t(key, params) system every other user-facing string uses.
//
// Fix: every place that used to build one of these messages now ALSO records
// a MessageKey + MessageParams alongside the original plain-English text
// (kept as Message/Text — never removed). The frontend renders
// t(messageKey, messageParams) when a key is present, falling back to the
// plain text otherwise — which covers both an unrecognized/future key and
// every entry already persisted in job_history.json/message_log.json before
// this change (they simply have no key, exactly like a real user's existing
// data will after this ships).
//
// Params are always plain, already-formatted strings/numbers (duration
// strings, byte counts, error text) — the same values the old fmt.Sprintf
// calls used — never Go error values or other types translations.js's dumb
// string-replace t() can't stringify sensibly.
type MessageKey string

const (
	// Directory backup (gui/backup_inline.go, runBackupInlineInternal).
	MsgBackupDirNotExist         MessageKey = "logBackupDirNotExist"         // {dir}
	MsgBackupCancelled           MessageKey = "logBackupCancelled"           // (none)
	MsgCatalogCreateFailed       MessageKey = "logCatalogCreateFailed"       // {error}
	MsgSessionLost               MessageKey = "logSessionLost"               // {error}
	MsgCatalogFinalizeFailed     MessageKey = "logCatalogFinalizeFailed"     // {error}
	MsgCatalogCloseFailed        MessageKey = "logCatalogCloseFailed"        // {error}
	MsgAllDirsFailed             MessageKey = "logAllDirsFailed"             // {total}, {errors}
	MsgManifestUploadFailed      MessageKey = "logManifestUploadFailed"      // {error}
	MsgSessionFinalizeFailed     MessageKey = "logSessionFinalizeFailed"     // {error}
	MsgBackupPartial             MessageKey = "logBackupPartial"             // {duration}, {ok}, {total}, {mb}, {new}, {reused}, {errors}, {skippedNote}
	MsgBackupCompletedWithErrors MessageKey = "logBackupCompletedWithErrors" // {duration}, {mb}, {new}, {reused}, {failed}, {skippedNote}
	MsgBackupCompleted           MessageKey = "logBackupCompleted"           // {duration}, {mb}, {new}, {reused}, {skippedNote}
	// logSkippedFilesNote is not a message in its own right — it's a small
	// reusable clause the frontend renders first (via {count}) and splices
	// into the {skippedNote} placeholder of the three keys above, so the
	// "N files skipped" detail survives translation instead of being
	// silently dropped when a message_key is present. See App.jsx's
	// renderLocalizedMessage.
	MsgSkippedFilesNote MessageKey = "logSkippedFilesNote" // {count}

	// Machine (raw-disk) backup (gui/backup_inline.go, runMachineBackupInline).
	MsgMachineBackupFailed    MessageKey = "logMachineBackupFailed"    // {error}
	MsgMachineBackupCompleted MessageKey = "logMachineBackupCompleted" // {duration}

	// Scheduler (gui/scheduler.go).
	MsgScheduledJobError      MessageKey = "logScheduledJobError"      // {error}
	MsgJobAbandoned           MessageKey = "logJobAbandoned"           // (none)
	MsgBackupCompletedGeneric MessageKey = "logBackupCompletedGeneric" // (none) — scheduler's own placeholder before the detailed OnComplete entry lands (standalone mode)

	// Restore (gui/main.go, RestoreSnapshot's goroutine).
	MsgRestoreCompleted MessageKey = "logRestoreCompleted" // (none)
	MsgRestoreFailed    MessageKey = "logRestoreFailed"    // {error}
)

// msgParams is a small typed alias so call sites read as `msgParams{"dir": p}`
// instead of the more verbose `map[string]interface{}{...}` everywhere this
// is used — purely a readability convenience, same underlying type the JSON
// fields and the frontend's t(key, params) both expect.
type msgParams = map[string]interface{}
