package main

import (
	"fmt"
	"os"
)

// applyConfiguredSplit overrides analysis.ShouldSplit / SuggestedJobs using the
// configured policy (SplitSizeGB threshold + DisableSplit) — the SAME policy
// RunBackupInline applies — so the split preview/plan matches what a real backup
// will do. Returns the effective bin size for CreateSplitJobs.
func (a *App) applyConfiguredSplit(analysis *BackupAnalysis) uint64 {
	binSize := uint64(DefaultSplitSizeGB) * 1024 * 1024 * 1024
	disable := false
	if a.config != nil {
		binSize = a.config.SplitSizeBytes()
		disable = a.config.DisableSplit
	}
	analysis.ShouldSplit = !disable && !analysis.Incomplete && analysis.TotalSize > binSize
	if analysis.ShouldSplit {
		analysis.SuggestedJobs = int((analysis.TotalSize + binSize - 1) / binSize)
		if analysis.SuggestedJobs < 2 {
			analysis.SuggestedJobs = 2
		}
	} else {
		analysis.SuggestedJobs = 1
	}
	return binSize
}

// AnalyzeBackup analyzes backup directories and determines if split is needed
// Returns analysis with total size, folder breakdown, and split recommendation
func (a *App) AnalyzeBackup(backupDirs []string, excludeList []string) (map[string]interface{}, error) {
	writeBackupLog(fmt.Sprintf("AnalyzeBackup called for %d directories", len(backupDirs)))

	// Pass the user's exclusions so an excluded top-level folder is neither sized
	// nor turned into its own job (B-1).
	analysis, err := AnalyzeBackupDirs(backupDirs, excludeList, nil)
	if err != nil {
		writeBackupLog(fmt.Sprintf("AnalyzeBackup failed: %v", err))
		return nil, err
	}
	binSize := a.applyConfiguredSplit(analysis)

	// Convert to map for JSON serialization to frontend
	result := map[string]interface{}{
		"total_size":      analysis.TotalSize,
		"total_size_fmt":  FormatSize(analysis.TotalSize),
		"should_split":    analysis.ShouldSplit,
		"suggested_jobs":  analysis.SuggestedJobs,
		"split_threshold": binSize,
		"folders":         make([]map[string]interface{}, len(analysis.Folders)),
	}

	for i, folder := range analysis.Folders {
		result["folders"].([]map[string]interface{})[i] = map[string]interface{}{
			"path":     folder.Path,
			"name":     folder.Name,
			"size":     folder.Size,
			"size_fmt": FormatSize(folder.Size),
		}
	}

	writeBackupLog(fmt.Sprintf("Analysis: %s total, split=%v, %d jobs suggested",
		FormatSize(analysis.TotalSize), analysis.ShouldSplit, analysis.SuggestedJobs))

	return result, nil
}

// CreateBackupSplitPlan creates a plan for splitting a large backup
// Returns the split jobs that will be created
func (a *App) CreateBackupSplitPlan(backupDirs []string, backupID string, excludeList []string) ([]map[string]interface{}, error) {
	writeBackupLog(fmt.Sprintf("CreateBackupSplitPlan called for backup ID: %s", backupID))

	// Report folder-by-folder sizing progress to the GUI (the user opted into the
	// split and may wait several minutes on a large volume). Pass the user's
	// exclusions so an excluded top-level folder is not sized or turned into its
	// own split job (which would back it up despite the exclusion — B-1).
	analysis, err := AnalyzeBackupDirs(backupDirs, excludeList, a.emitAnalysisProgress)
	if err != nil {
		return nil, err
	}

	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}
	binSize := a.applyConfiguredSplit(analysis)
	splitJobs := CreateSplitJobs(analysis, backupID, hostname, binSize)

	// Convert to map array for JSON
	result := make([]map[string]interface{}, len(splitJobs))
	for i, job := range splitJobs {
		result[i] = map[string]interface{}{
			"index":        job.Index,
			"total_jobs":   job.TotalJobs,
			"folders":      job.Folders,
			"total_size":   job.TotalSize,
			"size_fmt":     FormatSize(job.TotalSize),
			"backup_id":    job.BackupID,
			"parent_id":    job.ParentID,
			"exclude_list": job.ExcludeList, // v2-H-01: remainder job's subfolder exclusions
		}
	}

	writeBackupLog(fmt.Sprintf("Split plan created: %d jobs", len(splitJobs)))
	return result, nil
}
