package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// analysisSizeBudget is a runaway guard on the explicit-split size scan — NOT a
// normal-case timeout. Sizing only runs when the user opts into splitting a
// backup (the default path does no analysis at all), and a legitimate large
// volume can take several minutes to size (≈5 min for 1 TB), so the budget is
// deliberately generous; it only fires to stop a pathological scan that would
// otherwise never finish. If it fires, the analysis is marked Incomplete and the
// backup proceeds unsplit rather than acting on partial sizes.
const analysisSizeBudget = 30 * time.Minute

const (
	// DefaultSplitSizeGB is the default auto-split threshold and per-bin target,
	// in GB, used when the config does not override SplitSizeGB.
	DefaultSplitSizeGB = 150

	// SplitThreshold: default total-size threshold above which a backup is split.
	SplitThreshold = DefaultSplitSizeGB * 1024 * 1024 * 1024 // 150 GB

	// BinTargetSize: default target size per bin for bin-packing.
	BinTargetSize = DefaultSplitSizeGB * 1024 * 1024 * 1024 // 150 GB
)

// GenerateBackupID creates a backup-id from hostname and path
// Format: hostname_DRIVE_PATH (e.g., SERVER01_D_DATA_Users)
func GenerateBackupID(hostname, path string) string {
	cleanPath := filepath.Clean(path)
	cleanPath = strings.ReplaceAll(cleanPath, "\\", "_")
	cleanPath = strings.ReplaceAll(cleanPath, "/", "_")
	cleanPath = strings.ReplaceAll(cleanPath, ":", "")
	cleanPath = strings.ReplaceAll(cleanPath, " ", "-")

	// Remove any characters not allowed by PBS backup-id format
	// PBS requires: ^[A-Za-z0-9_][A-Za-z0-9._\-]*$
	var sanitized []byte
	for _, c := range []byte(cleanPath) {
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '_' || c == '.' || c == '-' {
			sanitized = append(sanitized, c)
		}
	}
	cleanPath = string(sanitized)

	cleanPath = strings.Trim(cleanPath, "_")

	if cleanPath == "" {
		return hostname
	}
	return fmt.Sprintf("%s_%s", hostname, cleanPath)
}

// generateBinID creates a stable backup-id for a bin based on its folder paths.
// Uses a short hash of the sorted paths so the same set of folders always
// produces the same bin ID (important for PBS dedup across runs).
func generateBinID(hostname string, parentPath string, folders []FolderInfo, binIndex int) string {
	// Build a stable key from sorted folder names
	names := make([]string, len(folders))
	for i, f := range folders {
		names[i] = f.Name
	}
	sort.Strings(names)

	h := sha256.Sum256([]byte(strings.Join(names, "\n")))
	shortHash := hex.EncodeToString(h[:4]) // 8 hex chars

	parentID := GenerateBackupID(hostname, parentPath)
	return fmt.Sprintf("%s_bin%d_%s", parentID, binIndex, shortHash)
}

// FolderInfo represents a top-level folder with its size
type FolderInfo struct {
	Path         string `json:"path"`
	Name         string `json:"name"`
	Size         uint64 `json:"size"`
	AccessDenied bool   `json:"access_denied"`
	BackupID     string `json:"backup_id"`
}

// BackupAnalysis contains the analysis of directories to backup
type BackupAnalysis struct {
	TotalSize      uint64       `json:"total_size"`
	Folders        []FolderInfo `json:"folders"`
	ShouldSplit    bool         `json:"should_split"`
	SuggestedJobs  int          `json:"suggested_jobs"`
	RootsWithFiles []string     `json:"roots_with_files,omitempty"` // selected roots that contain direct files (need a remainder job when split — v2-H-01)
	Incomplete     bool         `json:"incomplete,omitempty"`       // the size scan hit its deadline; TotalSize is a partial undercount, so split decisions must not trust it
}

// cleanKey normalizes a path for exact-path comparison (clean + lowercase) so
// exclusions and folder paths compare uniformly across separators/case (Windows).
func cleanKey(p string) string {
	return strings.ToLower(filepath.Clean(p))
}

// AnalyzeBackupDirs analyzes the top-level folders in the backup directories.
// excludes are absolute folder paths to skip from the split plan (so an excluded
// subfolder is neither sized nor turned into a job — keeps the size estimate and
// the root-remainder job correct; v2-H-01).
//
// This only runs when the user explicitly opts into splitting a backup; it is no
// longer on the default backup path. onProgress (may be nil) is called as each
// top-level folder finishes sizing, with the running done/total count and the
// bytes scanned so far, so the GUI can show progress during a multi-minute scan.
func AnalyzeBackupDirs(backupDirs []string, excludes []string, onProgress func(done, total int, scannedBytes uint64)) (*BackupAnalysis, error) {
	analysis := &BackupAnalysis{
		Folders: make([]FolderInfo, 0),
	}

	// Shared deadline across all folder size scans as a runaway guard (see
	// analysisSizeBudget); a legitimate large volume completes well within it.
	ctx, cancel := context.WithTimeout(context.Background(), analysisSizeBudget)
	defer cancel()

	excludeSet := make(map[string]bool, len(excludes))
	for _, e := range excludes {
		if e = strings.TrimSpace(e); e != "" {
			excludeSet[cleanKey(e)] = true
		}
	}

	// First pass (cheap — only ReadDir): enumerate the top-level folders to size,
	// so we know the total up front and can report "done/total" progress while the
	// expensive recursive sizing runs in the second pass.
	type pendingFolder struct{ path, name string }
	var toSize []pendingFolder
	for _, dir := range backupDirs {
		info, err := os.Stat(dir)
		if err != nil {
			return nil, fmt.Errorf("cannot access directory %s: %w", dir, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("%s is not a directory", dir)
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, fmt.Errorf("cannot read directory %s: %w", dir, err)
		}

		hasDirectFiles := false
		for _, entry := range entries {
			if entry.Type().IsRegular() {
				// A regular file sitting directly in the root belongs to no subfolder
				// job; the root needs a remainder job so a split backup does not drop it.
				hasDirectFiles = true
				continue
			}
			if !entry.IsDir() {
				continue // symlink/device/junction — WriteDir handles or skips these
			}
			folderPath := filepath.Join(dir, entry.Name())
			if excludeSet[cleanKey(folderPath)] {
				continue // excluded folder: don't size it or make a job for it
			}
			toSize = append(toSize, pendingFolder{path: folderPath, name: entry.Name()})
		}
		if hasDirectFiles {
			analysis.RootsWithFiles = append(analysis.RootsWithFiles, dir)
		}
	}

	total := len(toSize)
	if onProgress != nil {
		onProgress(0, total, 0)
	}

	// Second pass: size each folder (the expensive recursive walk), bounded by the
	// shared deadline, reporting progress after each one.
	for i, p := range toSize {
		size, err := calculateDirSizeCtx(ctx, p.path)

		folderInfo := FolderInfo{
			Path: p.path,
			Name: p.name,
			Size: size,
		}

		if err != nil && strings.Contains(err.Error(), "access denied") {
			folderInfo.AccessDenied = true
			folderInfo.Size = 500 * 1024 * 1024 * 1024 // Estimate 500GB
		}

		analysis.Folders = append(analysis.Folders, folderInfo)
		analysis.TotalSize += folderInfo.Size
		if onProgress != nil {
			onProgress(i+1, total, analysis.TotalSize)
		}
	}

	// If the runaway guard fired, TotalSize is a partial undercount and some
	// folders were sized as 0 — never offer a split on numbers we don't trust.
	if ctx.Err() != nil {
		analysis.Incomplete = true
	}

	// Sort folders by size (largest first) for better bin-packing
	sort.Slice(analysis.Folders, func(i, j int) bool {
		return analysis.Folders[i].Size > analysis.Folders[j].Size
	})

	analysis.ShouldSplit = !analysis.Incomplete && analysis.TotalSize > SplitThreshold

	if analysis.ShouldSplit {
		analysis.SuggestedJobs = int((analysis.TotalSize + BinTargetSize - 1) / BinTargetSize)
		if analysis.SuggestedJobs < 2 {
			analysis.SuggestedJobs = 2
		}
	} else {
		analysis.SuggestedJobs = 1
	}

	return analysis, nil
}

// SplitJob represents a backup job (one or more folders grouped together)
type SplitJob struct {
	Index       int      `json:"index"`
	TotalJobs   int      `json:"total_jobs"`
	Folders     []string `json:"folders"`
	TotalSize   uint64   `json:"total_size"`
	BackupID    string   `json:"backup_id"`
	ParentID    string   `json:"parent_id"`
	ExcludeList []string `json:"exclude_list,omitempty"` // exclusions for this job (root remainder job excludes already-covered subfolders — v2-H-01)
}

// CreateSplitJobs groups folders into bins of ~BinTargetSize using first-fit decreasing.
//
// Strategy:
//   - Folders > BinTargetSize get their own job (solo bin)
//   - Remaining folders are packed into bins of ~BinTargetSize
//   - Folders sorted largest-first for better packing
//   - Bin IDs are stable (hash of folder names) for PBS dedup across runs
func CreateSplitJobs(analysis *BackupAnalysis, baseBackupID string, hostname string, binSize uint64) []SplitJob {
	if binSize == 0 {
		binSize = BinTargetSize
	}
	if !analysis.ShouldSplit {
		allFolders := make([]string, len(analysis.Folders))
		for i, f := range analysis.Folders {
			allFolders[i] = f.Path
		}
		return []SplitJob{{
			Index:     1,
			TotalJobs: 1,
			Folders:   allFolders,
			TotalSize: analysis.TotalSize,
			BackupID:  baseBackupID,
			ParentID:  baseBackupID,
		}}
	}

	// Determine parent path for bin ID generation
	parentPath := ""
	if len(analysis.Folders) > 0 {
		parentPath = filepath.Dir(analysis.Folders[0].Path)
	}

	// Separate large folders (solo bins) from packable folders
	var soloFolders []FolderInfo
	var packable []FolderInfo

	for _, folder := range analysis.Folders {
		if folder.Size > binSize || folder.AccessDenied {
			soloFolders = append(soloFolders, folder)
		} else {
			packable = append(packable, folder)
		}
	}

	// If nothing to do, return single job with all folders
	if len(soloFolders) == 0 && len(packable) == 0 {
		allFolders := make([]string, len(analysis.Folders))
		for i, f := range analysis.Folders {
			allFolders[i] = f.Path
		}
		return []SplitJob{{
			Index:     1,
			TotalJobs: 1,
			Folders:   allFolders,
			TotalSize: analysis.TotalSize,
			BackupID:  baseBackupID,
			ParentID:  baseBackupID,
		}}
	}

	jobs := make([]SplitJob, 0)

	// Solo bins: one job per large/denied folder
	for _, folder := range soloFolders {
		backupID := folder.BackupID
		if backupID == "" {
			backupID = GenerateBackupID(hostname, folder.Path)
		}
		jobs = append(jobs, SplitJob{
			Folders:   []string{folder.Path},
			TotalSize: folder.Size,
			BackupID:  backupID,
			ParentID:  baseBackupID,
		})
	}

	// Bin-pack remaining folders (first-fit decreasing, already sorted largest first)
	bins := binPackFolders(packable, binSize)

	for i, bin := range bins {
		folders := make([]string, len(bin))
		var totalSize uint64
		for j, f := range bin {
			folders[j] = f.Path
			totalSize += f.Size
		}

		binID := generateBinID(hostname, parentPath, bin, i+1)

		jobs = append(jobs, SplitJob{
			Folders:   folders,
			TotalSize: totalSize,
			BackupID:  binID,
			ParentID:  baseBackupID,
		})
	}

	// Root remainder jobs (v2-H-01): files directly in a selected root belong to no
	// subfolder job, so without this they are silently dropped from a split backup.
	// For each root that has direct files, back up the root with all its subfolders
	// (already covered above) excluded — capturing only the root-level files.
	for _, root := range analysis.RootsWithFiles {
		var subExcludes []string
		rootKey := cleanKey(root)
		for _, f := range analysis.Folders {
			if cleanKey(filepath.Dir(f.Path)) == rootKey {
				subExcludes = append(subExcludes, f.Path)
			}
		}
		jobs = append(jobs, SplitJob{
			Folders:     []string{root},
			ExcludeList: subExcludes,
			BackupID:    GenerateBackupID(hostname, root) + "_rootfiles",
			ParentID:    baseBackupID,
		})
	}

	// Set indices
	for i := range jobs {
		jobs[i].Index = i + 1
		jobs[i].TotalJobs = len(jobs)
	}

	return jobs
}

// binPackFolders groups folders into bins using first-fit decreasing.
// Folders must be pre-sorted by size (largest first).
func binPackFolders(folders []FolderInfo, targetSize uint64) [][]FolderInfo {
	if len(folders) == 0 {
		return nil
	}

	type bin struct {
		folders []FolderInfo
		used    uint64
	}

	var bins []bin

	for _, folder := range folders {
		// Find first bin that can fit this folder
		placed := false
		for i := range bins {
			if bins[i].used+folder.Size <= targetSize {
				bins[i].folders = append(bins[i].folders, folder)
				bins[i].used += folder.Size
				placed = true
				break
			}
		}

		if !placed {
			// Create new bin
			bins = append(bins, bin{
				folders: []FolderInfo{folder},
				used:    folder.Size,
			})
		}
	}

	result := make([][]FolderInfo, len(bins))
	for i, b := range bins {
		result[i] = b.folders
	}
	return result
}

// FormatSize formats a size in bytes to human-readable format
func FormatSize(bytes uint64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
		TB = 1024 * GB
	)

	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.2f TB", float64(bytes)/float64(TB))
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d bytes", bytes)
	}
}
