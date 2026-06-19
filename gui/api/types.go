package api

// BackupRequest represents a backup job request
type BackupRequest struct {
	BackupType   string   `json:"backup_type"`   // "directory" or "machine"
	BackupID     string   `json:"backup_id"`
	BackupDirs   []string `json:"backup_dirs"`
	DriveLetters []string `json:"drive_letters,omitempty"`
	ExcludeList  []string `json:"exclude_list,omitempty"`
	UseVSS       bool     `json:"use_vss"`
	Compression  string   `json:"compression,omitempty"` // "fastest", "default", "better", "best"
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
	Running       bool                 `json:"running"`
	Version       string               `json:"version"`
	ActiveJobs    int                  `json:"active_jobs"`
	LastBackup    string               `json:"last_backup,omitempty"`
	Configuration map[string]any `json:"configuration"`
}

// JobInfo represents information about a scheduled job
type JobInfo struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	BackupType   string `json:"backup_type"`
	Schedule     string `json:"schedule"`
	LastRun      string `json:"last_run,omitempty"`
	Status       string `json:"status"` // "idle", "running", "error"
	NextRun      string `json:"next_run,omitempty"`
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
	Progress  float64 `json:"progress"`  // 0-100
	Message   string  `json:"message"`
	Success   bool    `json:"success"`
	Complete  bool    `json:"complete"`
	Error     string  `json:"error,omitempty"`
	StartTime string  `json:"start_time,omitempty"`
}
