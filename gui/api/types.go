package api

// BackupRequest represents a backup job request
type BackupRequest struct {
	BackupType   string   `json:"backup_type"` // "directory" or "machine"
	BackupID     string   `json:"backup_id"`
	BackupDirs   []string `json:"backup_dirs"`
	DriveLetters []string `json:"drive_letters,omitempty"`
	ExcludeList  []string `json:"exclude_list,omitempty"`
	UseVSS       bool     `json:"use_vss"`
	Compression  string   `json:"compression,omitempty"`   // "none", "fastest", "default", "better", "best"; "" = the Backup Options setting
	PBSServerID  string   `json:"pbs_server_id,omitempty"` // empty = the configured default server
	Comment      string   `json:"comment,omitempty"`       // sent as-is as the PBS snapshot's manifest comment
	// ChangeDetectionMode is a directory Backup Set run's mode ("", "legacy",
	// "data", "metadata"); an older service ignores it (classic format).
	ChangeDetectionMode string `json:"change_detection_mode,omitempty"`
	// ReadFreeSpace makes a machine backup a forensic copy (free space read
	// too); an older service ignores it and skips free space.
	ReadFreeSpace bool `json:"read_free_space,omitempty"`
	// DiskIDs is a machine Backup Set's identity for each disk in
	// DriveLetters, at the same index, so the service finds the set's disks
	// when the system has numbered them differently since; an older service
	// ignores it and backs up the paths.
	DiskIDs []string `json:"disk_ids,omitempty"`
	// LiveRead is a Linux machine Backup Set read without a snapshot, as its
	// owner accepted; an older service ignores it and stops when a mounted
	// disk cannot be snapshotted.
	LiveRead bool `json:"live_read,omitempty"`
	// SkipPartitions are the partitions a machine Backup Set leaves out; an
	// older service ignores them and backs up the whole disks.
	SkipPartitions []PartitionSkip `json:"skip_partitions,omitempty"`
}

// PartitionSkip is a partition a machine Backup Set leaves out: it is not
// read and goes into the image as zeros. The set saves it as its disk (the
// set's diskIds entry for it, or its device path when the disk has none) and
// where the partition starts and how long it is; a run leaves it out only
// when that disk still has that partition.
type PartitionSkip struct {
	DiskID string `json:"diskId,omitempty"`
	Device string `json:"device"`
	Start  uint64 `json:"start"`
	Size   uint64 `json:"size"`
	Number int    `json:"number,omitempty"`
	Label  string `json:"label,omitempty"` // how the editor named it, for the log
}

// BackupResponse represents the result of a backup operation
type BackupResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	JobID   string `json:"job_id,omitempty"`
	Error   string `json:"error,omitempty"`
}

// StatusResponse represents the service status
type StatusResponse struct {
	Running       bool           `json:"running"`
	Version       string         `json:"version"`
	ActiveJobs    int            `json:"active_jobs"`
	LastBackup    string         `json:"last_backup,omitempty"`
	Configuration map[string]any `json:"configuration"`
}

// JobInfo represents information about a scheduled job
type JobInfo struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	BackupType string `json:"backup_type"`
	Schedule   string `json:"schedule"`
	LastRun    string `json:"last_run,omitempty"`
	Status     string `json:"status"` // "idle", "running", "error"
	NextRun    string `json:"next_run,omitempty"`
}

// JobsResponse lists all configured jobs
type JobsResponse struct {
	Jobs []JobInfo `json:"jobs"`
}

// ErrorResponse represents an API error
type ErrorResponse struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
	Details string `json:"details,omitempty"`
}

// BackupProgress represents the current state of a running backup
type BackupProgress struct {
	JobID     string  `json:"job_id"`
	Running   bool    `json:"running"`
	Progress  float64 `json:"progress"` // 0-100
	Message   string  `json:"message"`
	Success   bool    `json:"success"`
	Complete  bool    `json:"complete"`
	Error     string  `json:"error,omitempty"`
	StartTime string  `json:"start_time,omitempty"`
	// Name is the Backup Set's (or one-off's) name, so a GUI showing several
	// runs can tell them apart. BackupID is the run's backup-id (a GUI's
	// request only).
	Name     string `json:"name,omitempty"`
	BackupID string `json:"backup_id,omitempty"`
	// Scheduled marks a run the service started itself.
	Scheduled bool `json:"scheduled,omitempty"`
	// Paused and PausedSince (Unix ms) while the run waits at a pause.
	Paused      bool  `json:"paused,omitempty"`
	PausedSince int64 `json:"paused_since,omitempty"`
	// Stats is the run's latest live statistics, in the form of the app's
	// "backup:stats" event (files, bytes, upload, new and reused chunks, the
	// disk of a machine backup); File is the file being read. A GUI following
	// the run passes both on as its own run's events.
	Stats map[string]interface{} `json:"stats,omitempty"`
	File  string                 `json:"file,omitempty"`
	// MessageKey and MessageParams are a finished run's summary as message
	// code and figures (the app's msgcodes.go), so the GUI's Reports entry
	// reads as one made without the service: translated, with the figures.
	MessageKey    string                 `json:"message_key,omitempty"`
	MessageParams map[string]interface{} `json:"message_params,omitempty"`
}

// RunsResponse lists the service's runs: those in progress and those that
// finished in the last few minutes (their final state).
type RunsResponse struct {
	Runs []BackupProgress `json:"runs"`
}

// RunControlResponse answers Pause, Resume and Stop: the pause state after
// the request.
type RunControlResponse struct {
	Paused      bool  `json:"paused"`
	PausedSince int64 `json:"paused_since,omitempty"`
}

// PBSAccessRequest asks the service for what one PBS operation needs (POST
// /pbs/access): the server ID ("" for the default) and whether the snapshot
// may be encrypted, so the key is wanted too.
type PBSAccessRequest struct {
	ID      string `json:"id"`
	WithKey bool   `json:"with_key,omitempty"`
}

// PBSAccess is what the app gets for one PBS operation (restore, browse,
// search, undelete, roll back) when the service holds the settings: where the
// server is, and how to log in for this operation. A username server gets a
// fresh ticket, never its password (upstream's design). A token server gets
// its token: PBS issues tickets only for a username login, so without it a
// token server could not be browsed (upstream's ticket flow fails there). The
// app keeps all of it in memory for the operation only.
type PBSAccess struct {
	BaseURL         string `json:"baseurl"`
	CertFingerprint string `json:"certfingerprint,omitempty"`
	Datastore       string `json:"datastore"`
	Namespace       string `json:"namespace,omitempty"`
	// Token login.
	AuthID string `json:"authid,omitempty"`
	Secret string `json:"secret,omitempty"`
	// Ticket login.
	Username  string `json:"username,omitempty"`
	Ticket    string `json:"ticket,omitempty"`
	CSRFToken string `json:"csrf_token,omitempty"`
	// The encryption key, when asked for (WithKey) and the server has one:
	// its path (for messages), the key file's contents (the app may not read
	// the settings folder) and the remembered passphrase, if any.
	EncryptionKeyFile string `json:"encryption_key_file,omitempty"`
	KeyFileData       []byte `json:"key_file_data,omitempty"`
	KeyPassphrase     string `json:"key_passphrase,omitempty"`
}

// JobEnabledRequest turns a Backup Set on or off (POST /jobs/enabled).
type JobEnabledRequest struct {
	ID      string `json:"id"`
	Enabled bool   `json:"enabled"`
}

// SettingsExportRequest asks for a settings export (POST /settings/export).
type SettingsExportRequest struct {
	IncludeSecrets bool `json:"include_secrets,omitempty"`
}

// SettingsExportResponse is the export file's contents.
type SettingsExportResponse struct {
	Data []byte `json:"data"`
}

// LogBundleRequest asks for the service's logs as a zip (POST /logs/bundle).
type LogBundleRequest struct {
	// Days is how far back to go; 0 means the service's default.
	Days int `json:"days,omitempty"`
}

// LogBundleResponse is the zip of the service's logs.
type LogBundleResponse struct {
	Data []byte `json:"data"`
}

// SettingsImportRequest merges an exported file (POST /settings/import).
type SettingsImportRequest struct {
	Data []byte `json:"data"`
}

// SettingsImportResponse summarises what an import added or changed.
type SettingsImportResponse struct {
	Summary string `json:"summary"`
}

// KeySaveRequest stores an encryption key in the service's keys folder (POST
// /keys/save): the key file's contents and the name to give it ("" for a
// generated name). The answer is the stored file's path.
type KeySaveRequest struct {
	Name string `json:"name,omitempty"`
	Data []byte `json:"data"`
}

// KeySaveResponse is the path a saved key was stored at.
type KeySaveResponse struct {
	Path string `json:"path"`
}
