package main

import "strings"

// Backup result + live-progress types shared across the backup engine, the GUI,
// the local API and the scheduler. BackupStatus is the single source of truth for
// "what happened in a backup run": it is (a) returned by the engine, (b) carried to
// the OnResult callback, and (c) (Group 1) serialized into the sidecar blob/manifest.
//
// Keeping one struct here avoids designing it twice for the result contract and the
// sidecar. The ExcludedByPolicy and Corrupted buckets are defined now but stay empty
// until Group 1 (user exclusions H-04 and mode-B corruption) fills them.

// BackupStatusFilename is the PBS blob name for the per-snapshot status sidecar.
// PBS requires a bare basename (no leading dot) ending in ".blob"; the payload is
// plain JSON. Listed in the manifest so the GUI can read it without a full restore.
const BackupStatusFilename = "nimbus-status.json.blob"

// BackupSidecar is the per-snapshot status persisted as a manifest blob: the files
// excluded by policy and the files skipped on read errors for THIS directory's
// snapshot. It lets the GUI show "in this backup, files X/Y were excluded/skipped"
// without restoring the archive.
type BackupSidecar struct {
	FormatVersion    int         `json:"format_version"`
	BackupID         string      `json:"backup_id"`
	Directory        string      `json:"directory"`
	GeneratedAt      int64       `json:"generated_at"`
	ExcludedByPolicy []FileIssue `json:"excluded_by_policy,omitempty"`
	SkippedReadError []FileIssue `json:"skipped_read_error,omitempty"`
}

// BackupOutcome is the three-state result of a backup run.
type BackupOutcome string

const (
	// OutcomeVerifiedSuccess: every selected directory committed, no chunk failed,
	// and no file was unreadable / changed during read / excluded — fully complete.
	OutcomeVerifiedSuccess BackupOutcome = "verified_success"
	// OutcomeSuccessWithExclusions: complete except for files the user deliberately
	// excluded by policy. Still a success (the absences are expected).
	OutcomeSuccessWithExclusions BackupOutcome = "success_with_policy_exclusions"
	// OutcomePartial: usable but incomplete — at least one directory failed, or some
	// files could not be read / changed during read (NOT verified, not policy-excluded).
	OutcomePartial BackupOutcome = "partial"
	// OutcomeFailed: nothing usable was produced (fatal error, no directory
	// committed, or a chunk upload failure that would make a committed index corrupt).
	OutcomeFailed BackupOutcome = "failed"
)

// FileIssue records one file that was excluded, skipped or corrupted, with the reason.
type FileIssue struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

// DirResult records the outcome of one selected backup directory.
type DirResult struct {
	Path  string `json:"path"`
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

// BackupStatus is the authoritative description of a finished backup run.
type BackupStatus struct {
	Outcome      BackupOutcome `json:"outcome"`
	BackupID     string        `json:"backup_id"`
	BackupTime   int64         `json:"backup_time"`
	DurationSec  float64       `json:"duration_sec"`
	TotalBytes   uint64        `json:"total_bytes"`
	NewChunks    uint64        `json:"new_chunks"`
	ReusedChunks uint64        `json:"reused_chunks"`
	FailedChunks uint64        `json:"failed_chunks"`
	Directories  []DirResult   `json:"directories"`

	// Three buckets of files that did not make it into the backup intact.
	// ExcludedByPolicy and Corrupted are populated in Group 1; SkippedReadError
	// comes from the existing PBSClient.SkippedFiles list today.
	ExcludedByPolicy []FileIssue `json:"excluded_by_policy,omitempty"`
	SkippedReadError []FileIssue `json:"skipped_read_error,omitempty"`
	Corrupted        []FileIssue `json:"corrupted,omitempty"`

	// Message is the human-readable summary already shown in logs and the UI.
	Message string `json:"message"`
}

// Success reports whether the run reached a success level — fully verified, or
// complete except for deliberate policy exclusions. Partial and failed are not.
func (s *BackupStatus) Success() bool {
	return s != nil && (s.Outcome == OutcomeVerifiedSuccess || s.Outcome == OutcomeSuccessWithExclusions)
}

// outcomeRank orders outcomes worst-to-best (failed=0 … verified=3).
func outcomeRank(o BackupOutcome) int {
	switch o {
	case OutcomeFailed:
		return 0
	case OutcomePartial:
		return 1
	case OutcomeSuccessWithExclusions:
		return 2
	default: // OutcomeVerifiedSuccess
		return 3
	}
}

// worstOutcome returns the worse (lower-ranked) of two outcomes.
func worstOutcome(a, b BackupOutcome) BackupOutcome {
	if outcomeRank(a) <= outcomeRank(b) {
		return a
	}
	return b
}

// merge folds a child run's counts, file buckets and worst outcome into agg.
// Used to build one aggregate result for multi-folder and auto-split runs.
func (s *BackupStatus) merge(child *BackupStatus) {
	if child == nil {
		return
	}
	s.Outcome = worstOutcome(s.Outcome, child.Outcome)
	s.NewChunks += child.NewChunks
	s.ReusedChunks += child.ReusedChunks
	s.FailedChunks += child.FailedChunks
	s.TotalBytes += child.TotalBytes
	s.Directories = append(s.Directories, child.Directories...)
	s.ExcludedByPolicy = append(s.ExcludedByPolicy, child.ExcludedByPolicy...)
	s.SkippedReadError = append(s.SkippedReadError, child.SkippedReadError...)
	s.Corrupted = append(s.Corrupted, child.Corrupted...)
}

// skippedToIssues wraps the engine's free-form SkippedFiles descriptions into the
// FileIssue bucket. The descriptions already embed the reason; Group 1 will record
// these with a clean path/reason split at the source (pxar.go).
func skippedToIssues(skipped []string) []FileIssue {
	if len(skipped) == 0 {
		return nil
	}
	issues := make([]FileIssue, 0, len(skipped))
	for _, s := range skipped {
		issues = append(issues, FileIssue{Reason: s})
	}
	return issues
}

// toLogicalPaths replaces the VSS shadow-copy root (from) with the original
// logical root (to) in each path/description, so excluded/skipped status lists
// show user-meaningful paths instead of \\?\GLOBALROOT\... shadow paths. No-op
// when from==to or from is empty (non-VSS backups).
func toLogicalPaths(items []string, from, to string) []string {
	if from == to || from == "" || len(items) == 0 {
		return items
	}
	out := make([]string, len(items))
	for i, s := range items {
		out[i] = strings.ReplaceAll(s, from, to)
	}
	return out
}

// excludedToIssues wraps the list of policy-excluded paths (H-04) into FileIssue.
func excludedToIssues(excluded []string) []FileIssue {
	if len(excluded) == 0 {
		return nil
	}
	issues := make([]FileIssue, 0, len(excluded))
	for _, p := range excluded {
		issues = append(issues, FileIssue{Path: p, Reason: "excluded by user policy"})
	}
	return issues
}

// BackupProgressStats is a structured snapshot of in-flight backup progress so the
// GUI can render real statistics instead of parsing them out of a log string.
type BackupProgressStats struct {
	Percent      float64 `json:"percent"` // 0.0 - 1.0
	BytesDone    uint64  `json:"bytes_done"`
	BytesTotal   uint64  `json:"bytes_total"` // 0 when the background size scan has not finished
	NewChunks    uint64  `json:"new_chunks"`
	ReusedChunks uint64  `json:"reused_chunks"`
	FailedChunks uint64  `json:"failed_chunks"`
	CurrentDir   string  `json:"current_dir,omitempty"`
	Message      string  `json:"message"`
}
