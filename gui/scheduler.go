package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// runningJobs tracks currently executing jobs to prevent duplicates
var runningJobs = make(map[string]bool)
var runningJobsMutex sync.Mutex

// ScheduledJob represents a scheduled backup job
type ScheduledJob struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	ScheduleTime string   `json:"scheduleTime"` // HH:MM format
	RunAtStartup bool     `json:"runAtStartup"`
	BackupDirs   []string `json:"backupDirs"`
	BackupID     string   `json:"backupId"`
	UseVSS       bool     `json:"useVSS"`
	BackupType   string   `json:"backupType"`
	ExcludeList  []string `json:"excludeList"`
	Compression  string   `json:"compression"` // "fastest", "default", "better", "best"
	LastRun      string   `json:"lastRun,omitempty"` // ISO timestamp
	NextRun      string   `json:"nextRun,omitempty"` // ISO timestamp
	Enabled      bool     `json:"enabled"`
}

// JobHistory represents a completed backup job
type JobHistory struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Timestamp  string   `json:"timestamp"` // ISO format
	Status     string   `json:"status"`    // "success", "failed", "running"
	Message    string   `json:"message"`
	BackupDirs []string `json:"backupDirs"`
	BackupID   string   `json:"backupId"`
	UseVSS     bool     `json:"useVSS"`
}

func getScheduledJobsPath() (string, error) {
	// Use ProgramData on Windows (shared between GUI and Service)
	var configDir string

	if programData := os.Getenv("ProgramData"); programData != "" {
		// Windows: C:\ProgramData\NimbusBackup
		configDir = filepath.Join(programData, "NimbusBackup")
	} else if systemDrive := os.Getenv("SystemDrive"); systemDrive != "" {
		// Windows fallback: if ProgramData not set, use C:\ProgramData hardcoded
		configDir = filepath.Join(systemDrive, "ProgramData", "NimbusBackup")
	} else {
		// Unix-like: use ~/.proxmox-backup-guardian
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		configDir = filepath.Join(homeDir, ".proxmox-backup-guardian")
	}

	// #nosec G703 -- ProgramData is a trusted Windows system environment variable, not user input
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", err
	}

	return filepath.Join(configDir, "scheduled_jobs.json"), nil
}

func getJobHistoryPath() (string, error) {
	// Use ProgramData on Windows (shared between GUI and Service)
	var configDir string

	if programData := os.Getenv("ProgramData"); programData != "" {
		// Windows: C:\ProgramData\NimbusBackup
		configDir = filepath.Join(programData, "NimbusBackup")
	} else if systemDrive := os.Getenv("SystemDrive"); systemDrive != "" {
		// Windows fallback: if ProgramData not set, use C:\ProgramData hardcoded
		configDir = filepath.Join(systemDrive, "ProgramData", "NimbusBackup")
	} else {
		// Unix-like: use ~/.proxmox-backup-guardian
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		configDir = filepath.Join(homeDir, ".proxmox-backup-guardian")
	}

	// #nosec G703 -- ProgramData is a trusted Windows system environment variable, not user input
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", err
	}

	return filepath.Join(configDir, "job_history.json"), nil
}

// SaveScheduledJob saves a new scheduled job
func (a *App) SaveScheduledJob(job ScheduledJob) error {
	writeDebugLog(fmt.Sprintf("SaveScheduledJob called for: %s", job.Name))

	// Load existing jobs
	jobs, err := a.GetScheduledJobs()
	if err != nil {
		// Refuse to continue: appending to an empty list and writing it back
		// would delete every existing job if the read/parse failed transiently.
		return fmt.Errorf("failed to load existing jobs (refusing to overwrite): %w", err)
	}

	// Set enabled by default
	job.Enabled = true

	// Calculate next run
	job.NextRun = calculateNextRun(job.ScheduleTime)

	// Add new job
	jobs = append(jobs, job)

	// Save to file
	jobsPath, err := getScheduledJobsPath()
	if err != nil {
		return fmt.Errorf("failed to get jobs path: %w", err)
	}

	data, err := json.MarshalIndent(jobs, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal jobs: %w", err)
	}

	if err := atomicWriteFile(jobsPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write jobs file: %w", err)
	}

	writeDebugLog(fmt.Sprintf("Scheduled job saved: %s (next run: %s)", job.Name, job.NextRun))

	// Note: For automatic execution after reboot, use the MSI installer
	// which installs NimbusBackup as a Windows Service

	return nil
}

// GetScheduledJobs returns all scheduled jobs
func (a *App) GetScheduledJobs() ([]ScheduledJob, error) {
	jobsPath, err := getScheduledJobsPath()
	if err != nil {
		return []ScheduledJob{}, err
	}

	data, err := os.ReadFile(jobsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []ScheduledJob{}, nil // No jobs yet
		}
		return nil, err
	}

	var jobs []ScheduledJob
	if err := json.Unmarshal(data, &jobs); err != nil {
		return nil, err
	}

	return jobs, nil
}

// GetScheduledJobsForAPI returns scheduled jobs as map[string]interface{} for API compatibility
// This method is used by the BackupHandler interface for HTTP API
func (a *App) GetScheduledJobsForAPI() []map[string]interface{} {
	jobs, err := a.GetScheduledJobs()
	if err != nil {
		writeDebugLog(fmt.Sprintf("GetScheduledJobsForAPI error: %v", err))
		return []map[string]interface{}{}
	}

	result := make([]map[string]interface{}, len(jobs))
	for i, job := range jobs {
		result[i] = map[string]interface{}{
			"id":           job.ID,
			"name":         job.Name,
			"backup_type":  job.BackupType,
			"backup_id":    job.BackupID,
			"schedule":     job.ScheduleTime,
			"use_vss":      job.UseVSS,
			"backup_dirs":  job.BackupDirs,
			"exclude_list": job.ExcludeList,
			"last_run":     job.LastRun,
			"next_run":     job.NextRun,
			"enabled":      job.Enabled,
		}
	}
	return result
}

// UpdateScheduledJob updates an existing scheduled job
func (a *App) UpdateScheduledJob(job ScheduledJob) error {
	writeDebugLog(fmt.Sprintf("UpdateScheduledJob called for: %s", job.Name))

	// Load existing jobs
	jobs, err := a.GetScheduledJobs()
	if err != nil {
		return fmt.Errorf("failed to load jobs: %w", err)
	}

	// Find and update the job
	found := false
	for i, j := range jobs {
		if j.ID == job.ID {
			// Preserve enabled state
			job.Enabled = j.Enabled
			// Recalculate next run with new schedule time
			job.NextRun = calculateNextRun(job.ScheduleTime)
			jobs[i] = job
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("job with ID %s not found", job.ID)
	}

	// Save to file
	jobsPath, err := getScheduledJobsPath()
	if err != nil {
		return fmt.Errorf("failed to get jobs path: %w", err)
	}

	data, err := json.MarshalIndent(jobs, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal jobs: %w", err)
	}

	if err := atomicWriteFile(jobsPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write jobs file: %w", err)
	}

	writeDebugLog(fmt.Sprintf("Scheduled job updated: %s (next run: %s)", job.Name, job.NextRun))
	return nil
}

// DeleteScheduledJob removes a scheduled job by ID
func (a *App) DeleteScheduledJob(jobID string) error {
	writeDebugLog(fmt.Sprintf("DeleteScheduledJob called for ID: %s", jobID))

	jobs, err := a.GetScheduledJobs()
	if err != nil {
		return err
	}

	// Filter out the job to delete
	filtered := []ScheduledJob{}
	for _, job := range jobs {
		if job.ID != jobID {
			filtered = append(filtered, job)
		}
	}

	// Save updated list
	jobsPath, err := getScheduledJobsPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(filtered, "", "  ")
	if err != nil {
		return err
	}

	return atomicWriteFile(jobsPath, data, 0600)
}

// GetJobHistory returns job history
func (a *App) GetJobHistory() ([]JobHistory, error) {
	historyPath, err := getJobHistoryPath()
	if err != nil {
		return []JobHistory{}, err
	}

	data, err := os.ReadFile(historyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []JobHistory{}, nil
		}
		return nil, err
	}

	var history []JobHistory
	if err := json.Unmarshal(data, &history); err != nil {
		return nil, err
	}

	return history, nil
}

// AddJobHistory adds a job to history
func (a *App) AddJobHistory(entry JobHistory) error {
	history, err := a.GetJobHistory()
	if err != nil {
		// Refuse to overwrite the history file with just this entry when the
		// existing history could not be read — that would destroy the record.
		return fmt.Errorf("failed to load job history (refusing to overwrite): %w", err)
	}

	// Add new entry at the beginning
	history = append([]JobHistory{entry}, history...)

	// Keep only last 50 entries
	if len(history) > 50 {
		history = history[:50]
	}

	// Save
	historyPath, err := getJobHistoryPath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return err
	}

	return atomicWriteFile(historyPath, data, 0600)
}

// calculateNextRun calculates the next run time based on schedule time (HH:MM)
func calculateNextRun(scheduleTime string) string {
	parts := strings.Split(scheduleTime, ":")
	if len(parts) != 2 {
		return ""
	}

	now := time.Now()
	var hour, min int
	if _, err := fmt.Sscanf(scheduleTime, "%d:%d", &hour, &min); err != nil {
		writeDebugLog(fmt.Sprintf("Error parsing schedule time %s: %v", scheduleTime, err))
		return ""
	}

	// Schedule for today at the specified time
	nextRun := time.Date(now.Year(), now.Month(), now.Day(), hour, min, 0, 0, now.Location())

	// If time has already passed today, schedule for tomorrow. Use AddDate (a
	// calendar day) rather than Add(24h): across a DST transition 24h of absolute
	// duration lands an hour off the intended wall-clock time, so a job at "02:30"
	// would fire at 01:30 or 03:30 on the switch day.
	if nextRun.Before(now) {
		nextRun = nextRun.AddDate(0, 0, 1)
	}

	return nextRun.Format(time.RFC3339)
}

// RecalculateNextRuns repairs jobs whose nextRun is MISSING or unparseable by
// giving them a valid future run time. It deliberately leaves an overdue but
// valid nextRun in the past: the scheduler now catches missed runs up on its
// next tick (see checkAndRunScheduledJobs). Pushing overdue runs forward here —
// as this used to — silently dropped the run a rebooted/off machine had missed.
func (a *App) RecalculateNextRuns() {
	writeDebugLog("RecalculateNextRuns called - repairing missing/invalid nextRun values")

	jobs, err := a.GetScheduledJobs()
	if err != nil {
		writeDebugLog(fmt.Sprintf("Error loading scheduled jobs: %v", err))
		return
	}

	modified := false

	for i, job := range jobs {
		if !job.Enabled || job.ScheduleTime == "" {
			continue
		}

		// A present, parseable nextRun is left alone — even if it is in the past,
		// so the scheduler's catch-up can run the missed backup.
		if job.NextRun != "" {
			if _, perr := time.Parse(time.RFC3339, job.NextRun); perr == nil {
				continue
			}
		}

		newNextRun := calculateNextRun(job.ScheduleTime)
		writeDebugLog(fmt.Sprintf("[RecalculateNextRuns] Job %s: nextRun was missing/invalid (%q), set to %s",
			job.Name, job.NextRun, newNextRun))
		jobs[i].NextRun = newNextRun
		modified = true
	}

	if modified {
		jobsPath, err := getScheduledJobsPath()
		if err != nil {
			writeDebugLog(fmt.Sprintf("Error getting jobs path: %v", err))
			return
		}

		data, err := json.MarshalIndent(jobs, "", "  ")
		if err != nil {
			writeDebugLog(fmt.Sprintf("Error marshaling jobs: %v", err))
			return
		}

		if err := atomicWriteFile(jobsPath, data, 0600); err != nil {
			writeDebugLog(fmt.Sprintf("Error saving recalculated jobs: %v", err))
		} else {
			writeDebugLog("Successfully recalculated stale nextRun values")
		}
	}
}

// StartScheduler starts the background job scheduler
func (a *App) StartScheduler() {
	writeDebugLog("Starting background job scheduler")

	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				a.checkAndRunScheduledJobs()
			case <-a.stopScheduler:
				writeDebugLog("Scheduler stopped")
				return
			}
		}
	}()
}

// StopScheduler stops the background job scheduler
func (a *App) StopScheduler() {
	writeDebugLog("Stopping background job scheduler")
	close(a.stopScheduler)
}

// CleanupAbandonedJobs marks any "running" jobs as abandoned on app startup
func (a *App) CleanupAbandonedJobs() {
	writeDebugLog("CleanupAbandonedJobs called - cleaning up stale running jobs")

	history, err := a.GetJobHistory()
	if err != nil {
		writeDebugLog(fmt.Sprintf("Error loading job history: %v", err))
		return
	}

	modified := false
	for i, entry := range history {
		if entry.Status == "running" {
			writeDebugLog(fmt.Sprintf("Marking abandoned job as failed: %s", entry.Name))
			history[i].Status = "failed"
			history[i].Message = "Abandonné (application interrompue)"
			history[i].Timestamp = time.Now().Format(time.RFC3339)
			modified = true
		}
	}

	if modified {
		// Save updated history
		historyPath, err := getJobHistoryPath()
		if err != nil {
			writeDebugLog(fmt.Sprintf("Error getting history path: %v", err))
			return
		}

		data, err := json.MarshalIndent(history, "", "  ")
		if err != nil {
			writeDebugLog(fmt.Sprintf("Error marshaling history: %v", err))
			return
		}

		if err := atomicWriteFile(historyPath, data, 0600); err != nil {
			writeDebugLog(fmt.Sprintf("Error saving updated history: %v", err))
		} else {
			writeDebugLog("Successfully cleaned up abandoned jobs")
		}
	}
}

// HandleStartupRun executes scheduled jobs that have runAtStartup enabled
func (a *App) HandleStartupRun() {
	writeDebugLog("HandleStartupRun called - checking for startup jobs")

	// Wait a bit to avoid conflict with scheduler if app starts at scheduled time
	time.Sleep(5 * time.Second)

	jobs, err := a.GetScheduledJobs()
	if err != nil {
		writeDebugLog(fmt.Sprintf("Error loading scheduled jobs: %v", err))
		return
	}

	for _, job := range jobs {
		if !job.Enabled || !job.RunAtStartup {
			continue
		}

		// Check if this job is already running (mutex protection)
		// If scheduler already started it, the mutex will prevent duplicate execution
		writeDebugLog(fmt.Sprintf("Executing startup job: %s", job.Name))
		go a.executeScheduledJob(job)
	}
}

// schedulerTickCount tracks ticks for periodic verbose logging
var schedulerTickCount int

// checkAndRunScheduledJobs checks if any jobs need to run
func (a *App) checkAndRunScheduledJobs() {
	jobs, err := a.GetScheduledJobs()
	if err != nil {
		writeDebugLog(fmt.Sprintf("[Scheduler] Error loading scheduled jobs: %v", err))
		return
	}

	schedulerTickCount++
	// Log status every 15 minutes (15 ticks at 1 tick/min) instead of every minute
	verbose := schedulerTickCount%15 == 1

	if len(jobs) == 0 {
		if verbose {
			writeDebugLog("[Scheduler] No scheduled jobs found")
		}
		return
	}

	now := time.Now()
	if verbose {
		writeDebugLog(fmt.Sprintf("[Scheduler] Checking %d jobs at %s", len(jobs), now.Format("15:04:05")))
	}

	for _, job := range jobs {
		if !job.Enabled {
			continue
		}

		// Parse next run time
		if job.NextRun == "" {
			continue
		}

		nextRun, err := time.Parse(time.RFC3339, job.NextRun)
		if err != nil {
			writeDebugLog(fmt.Sprintf("[Scheduler] Error parsing next run time for %s: %v", job.Name, err))
			continue
		}

		// Fire as soon as we are at or past NextRun, with NO upper bound. The old
		// 2-minute window meant a missed tick (machine asleep/hibernating, heavy
		// load, the process not running at NextRun) left now permanently past the
		// window, so the job never ran again until the next process restart —
		// silent loss of scheduled protection. A late run is caught up instead.
		// Repeated firing is prevented by the runningJobs guard and by
		// executeScheduledJob advancing NextRun on completion.
		shouldRun := now.After(nextRun)

		if verbose {
			writeDebugLog(fmt.Sprintf("[Scheduler] Job %s: NextRun=%s, Now=%s, ShouldRun=%v",
				job.Name, nextRun.Format("15:04:05"), now.Format("15:04:05"), shouldRun))
		}

		if shouldRun {
			writeDebugLog(fmt.Sprintf("[Scheduler] Executing scheduled job: %s", job.Name))
			go a.executeScheduledJob(job)
		}
	}
}

// executeScheduledJob executes a scheduled job
func (a *App) executeScheduledJob(job ScheduledJob) {
	// Check if job is already running
	runningJobsMutex.Lock()
	if runningJobs[job.ID] {
		writeDebugLog(fmt.Sprintf("Job %s is already running, skipping", job.Name))
		runningJobsMutex.Unlock()
		return
	}
	runningJobs[job.ID] = true
	runningJobsMutex.Unlock()

	// Ensure we mark as not running when done
	defer func() {
		runningJobsMutex.Lock()
		delete(runningJobs, job.ID)
		runningJobsMutex.Unlock()
	}()

	writeDebugLog(fmt.Sprintf("Executing scheduled job: %s", job.Name))

	// Prepare history entry (will be added at the end with final status)
	startTime := time.Now()

	// Use StartBackup to route through mode detection (service or direct)
	writeDebugLog(fmt.Sprintf("[Scheduled Job] Executing via StartBackup (mode: %s)", a.mode.String()))

	// Default to "fastest" if compression not set in job
	compression := job.Compression
	if compression == "" {
		compression = "fastest"
	}

	err := a.StartBackup(
		job.BackupType,
		job.BackupDirs,
		[]string{}, // driveLetters - empty for directory backups
		job.ExcludeList,
		job.BackupID,
		job.UseVSS,
		compression,
	)

	// Add history entry derived from the REAL outcome. In service mode StartBackup
	// runs synchronously (app_service_stubs.go returns RunBackupInline's error), so
	// err here reflects the finished backup: nil = success, non-nil = partial/failed.
	// NOTE: in GUI-standalone mode StartBackup is still fire-and-forget (the backup
	// runs in a goroutine and its error is not awaited here), so err is nil at this
	// point; the honest standalone history is recorded by startBackupDirect's
	// OnComplete instead. Making the standalone path awaitable belongs to the
	// service/GUI unification (Group 5).
	historyEntry := JobHistory{
		ID:         fmt.Sprintf("%d", startTime.Unix()),
		Name:       job.Name,
		Timestamp:  time.Now().Format(time.RFC3339),
		Status:     "success",
		Message:    "Backup terminé",
		BackupDirs: job.BackupDirs,
		BackupID:   job.BackupID,
		UseVSS:     job.UseVSS,
	}

	if err != nil {
		writeDebugLog(fmt.Sprintf("Scheduled job error: %v", err))
		historyEntry.Status = "failed"
		historyEntry.Message = fmt.Sprintf("Erreur: %v", err)
	}

	if err := a.AddJobHistory(historyEntry); err != nil {
		writeDebugLog(fmt.Sprintf("Warning: Failed to add job history: %v", err))
	}

	// Update job's last run and calculate next run.
	// Re-read from file to pick up any changes made by the GUI while backup was running.
	jobs, err := a.GetScheduledJobs()
	if err != nil {
		// Never rewrite the jobs file from a failed read — it would wipe every job.
		writeDebugLog(fmt.Sprintf("Warning: skipping LastRun/NextRun update, could not load jobs: %v", err))
		return
	}
	found := false
	for i, j := range jobs {
		if j.ID == job.ID {
			jobs[i].LastRun = time.Now().Format(time.RFC3339)
			jobs[i].NextRun = calculateNextRun(j.ScheduleTime)
			found = true
			break
		}
	}
	if !found {
		// Job was deleted while running; nothing to update.
		return
	}

	// Save updated jobs
	jobsPath, err := getScheduledJobsPath()
	if err != nil {
		writeDebugLog(fmt.Sprintf("Warning: cannot resolve jobs path: %v", err))
		return
	}
	data, err := json.MarshalIndent(jobs, "", "  ")
	if err != nil {
		writeDebugLog(fmt.Sprintf("Warning: cannot marshal jobs: %v", err))
		return
	}
	if err := atomicWriteFile(jobsPath, data, 0600); err != nil {
		writeDebugLog(fmt.Sprintf("Warning: Failed to save updated jobs: %v", err))
	}
}

