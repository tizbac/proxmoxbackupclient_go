package pbscommon

import (
	"encoding/json"
	"runtime"
	"time"
)

// BackupMetaFilename is the virtual file injected at the root of an archive
// (PXARArchive.VirtualFiles) recording where and on what the backup was taken;
// the GUI reads it to offer "restore to original path".
const BackupMetaFilename = ".proxmox_backup_client_meta.json"

// BackupStatusFilename is the PBS blob name for the per-snapshot status sidecar.
// PBS requires a bare basename ending in ".blob"; the payload is plain JSON.
const BackupStatusFilename = "proxmox-client-status.json.blob"

type BackupMeta struct {
	BackupID      string `json:"backup_id"`
	OriginalPath  string `json:"original_path"`
	Hostname      string `json:"hostname"`
	BackupTime    string `json:"backup_time"`
	ClientVersion string `json:"client_version"`
	OS            string `json:"os"`
	VSSUsed       bool   `json:"vss_used"`
}

func GenerateBackupMeta(backupID, originalPath, hostname, clientVersion string, vssUsed bool) ([]byte, error) {
	meta := BackupMeta{
		BackupID:      backupID,
		OriginalPath:  originalPath,
		Hostname:      hostname,
		BackupTime:    time.Now().UTC().Format(time.RFC3339),
		ClientVersion: clientVersion,
		OS:            runtime.GOOS,
		VSSUsed:       vssUsed,
	}
	return json.MarshalIndent(meta, "", "  ")
}

// FileIssue records one file that was excluded, skipped or corrupted, with the reason.
type FileIssue struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

// BackupSidecar is the per-snapshot status persisted as a manifest blob: the
// files excluded by policy and the files skipped on read errors for THIS
// snapshot, so the GUI can show them without restoring the archive.
type BackupSidecar struct {
	FormatVersion    int         `json:"format_version"`
	BackupID         string      `json:"backup_id"`
	Directories      []string    `json:"directories"`
	GeneratedAt      int64       `json:"generated_at"`
	ExcludedByPolicy []FileIssue `json:"excluded_by_policy,omitempty"`
	SkippedReadError []FileIssue `json:"skipped_read_error,omitempty"`
}

// SkippedToIssues wraps the engine's free-form SkippedFiles descriptions
// (which already embed the reason) into FileIssue entries.
func SkippedToIssues(skipped []string) []FileIssue {
	if len(skipped) == 0 {
		return nil
	}
	issues := make([]FileIssue, 0, len(skipped))
	for _, s := range skipped {
		issues = append(issues, FileIssue{Reason: s})
	}
	return issues
}
