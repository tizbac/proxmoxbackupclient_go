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
	DriveLetters []string `json:"driveLetters"` // physical disks for machine backups
	BackupID     string   `json:"backupId"`
	UseVSS       bool     `json:"useVSS"`
	BackupType   string   `json:"backupType"`
	ExcludeList  []string `json:"excludeList"`
	Compression  string   `json:"compression"`       // "fastest", "default", "better", "best"
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

// getScheduledJobsPath resolves scheduled_jobs.json inside the config dir.
// The config dir is pinned at startup (SetConfigDir): the service state dir
// when the scheduler runs inside the service (or the GUI in service mode),
// the home dir for a standalone GUI.
func getScheduledJobsPath() (string, error) {
	configDir, err := getConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "scheduled_jobs.json"), nil
}

// getJobHistoryPath resolves job_history.json inside the config dir (see
// getScheduledJobsPath).
func getJobHistoryPath() (string, error) {
	configDir, err := getConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "job_history.json"), nil
}

// SaveScheduledJob saves a new scheduled job
func (a *App) SaveScheduledJob(job ScheduledJob) error {
	writeDebugLog(fmt.Sprintf("SaveScheduledJob called for: %s", job.Name))

	// In service mode the GUI delegates to the service, which owns the jobs file.
	if a.isDelegatedToService() {
		return a.apiClient.SaveScheduledJob(jobToMap(&job))
	}

	if err := job.validate(); err != nil {
		return err
	}

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
	// which installs Proxmox Backup Client as a Windows Service

	return nil
}

// GetScheduledJobs returns all scheduled jobs
func (a *App) GetScheduledJobs() ([]ScheduledJob, error) {
	// In service mode the GUI delegates to the service.
	if a.isDelegatedToService() {
		apiJobs, err := a.apiClient.GetScheduledJobs()
		if err != nil {
			return nil, err
		}
		return apiJobsToSlice(apiJobs), nil
	}

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

// GetScheduledJobsForAPI returns scheduled jobs as map[string]interface{} for
// API compatibility.
//
// The map is a JSON round-trip of the struct (canonical json tag keys) plus
// aliases under the historical snake_case names, so both this build and older
// GUI builds — which read backup_type/schedule/last_run/… — decode the full
// job. Emitting only the old subset is what used to strip runAtStartup,
// compression, driveLetters, scheduleTime and the backup fields on the way to
// the frontend.
// This method is used by the BackupHandler interface for HTTP API
func (a *App) GetScheduledJobsForAPI() []map[string]interface{} {
	jobs, err := a.GetScheduledJobs()
	if err != nil {
		writeDebugLog(fmt.Sprintf("GetScheduledJobsForAPI error: %v", err))
		return []map[string]interface{}{}
	}

	result := make([]map[string]interface{}, len(jobs))
	for i := range jobs {
		m := jobToMap(&jobs[i])
		// Legacy aliases (older GUI builds).
		m["backup_type"] = m["backupType"]
		m["backup_id"] = m["backupId"]
		m["schedule"] = m["scheduleTime"]
		m["use_vss"] = m["useVSS"]
		m["backup_dirs"] = m["backupDirs"]
		m["exclude_list"] = m["excludeList"]
		m["last_run"] = m["lastRun"]
		m["next_run"] = m["nextRun"]
		result[i] = m
	}
	return result
}

// UpdateScheduledJob updates an existing scheduled job
func (a *App) UpdateScheduledJob(job ScheduledJob) error {
	writeDebugLog(fmt.Sprintf("UpdateScheduledJob called for: %s", job.Name))

	// In service mode the GUI delegates to the service.
	if a.isDelegatedToService() {
		return a.apiClient.UpdateScheduledJob(jobToMap(&job))
	}

	if err := job.validate(); err != nil {
		return err
	}

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
			// The edit form never carries the run record: keep it instead of
			// overwriting it with an empty value (an update must not erase when
			// the job last ran).
			job.LastRun = j.LastRun
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

	// In service mode the GUI delegates to the service.
	if a.isDelegatedToService() {
		return a.apiClient.DeleteScheduledJob(jobID)
	}

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
	// In service mode the GUI delegates to the service.
	if a.isDelegatedToService() {
		apiHist, err := a.apiClient.GetJobHistory()
		if err != nil {
			return nil, err
		}
		return apiHistoryToSlice(apiHist), nil
	}

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
	hour, min, ok := parseScheduleTime(scheduleTime)
	if !ok {
		return ""
	}

	now := time.Now()

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

// parseScheduleTime parses an "HH:MM" schedule (single-digit hour/minute
// accepted) and checks the ranges. Everything else — "" , "25:00", "02:75",
// "morning" — is rejected: calculateNextRun used to turn those into an empty
// nextRun, which the scheduler silently skips, i.e. a job that never runs.
func parseScheduleTime(scheduleTime string) (hour, min int, ok bool) {
	var h, m int
	if n, err := fmt.Sscanf(scheduleTime, "%d:%d", &h, &m); err != nil || n != 2 {
		return 0, 0, false
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, 0, false
	}
	return h, m, true
}

// validate reports a job that could not be executed as configured. It runs on
// every save/update (service side included), so a broken job is rejected where
// the user edits it instead of failing silently at its scheduled time.
func (j *ScheduledJob) validate() error {
	if strings.TrimSpace(j.Name) == "" {
		return fmt.Errorf("job name requis")
	}
	if _, _, ok := parseScheduleTime(j.ScheduleTime); !ok && j.ScheduleTime != "" {
		return fmt.Errorf("heure de planification invalide (format HH:MM attendu): %q", j.ScheduleTime)
	} else if j.ScheduleTime == "" && !j.RunAtStartup {
		return fmt.Errorf("heure de planification requise (format HH:MM)")
	}
	if len(j.BackupDirs) == 0 && len(j.DriveLetters) == 0 {
		return fmt.Errorf("au moins un repertoire ou un disque a sauvegarder est requis")
	}
	return nil
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

// claimJob marks a job as running and returns false when it already is. It is
// the single duplicate guard shared by the scheduler tick, the startup runner
// and the manual "run now" action, so one job can never execute twice
// concurrently (a second start is reported instead of silently swallowed).
func claimJob(jobID string) bool {
	runningJobsMutex.Lock()
	defer runningJobsMutex.Unlock()
	if runningJobs[jobID] {
		return false
	}
	runningJobs[jobID] = true
	return true
}

// releaseJob frees the claim taken by claimJob.
func releaseJob(jobID string) {
	runningJobsMutex.Lock()
	defer runningJobsMutex.Unlock()
	delete(runningJobs, jobID)
}

// runScheduledBackup performs the backup of a scheduled job: it goes through
// the test seam App.scheduledBackupFn when one is installed, otherwise through
// StartBackup (which routes through mode detection: service or direct).
func (a *App) runScheduledBackup(job ScheduledJob) error {
	if a.scheduledBackupFn != nil {
		return a.scheduledBackupFn(job)
	}

	// Default to "fastest" if compression not set in job
	compression := job.Compression
	if compression == "" {
		compression = "fastest"
	}

	// driveLetters carries the physical disks for machine backups (empty for
	// directory backups); it is persisted on the job so a scheduled run restores
	// the exact same selection.
	driveLetters := job.DriveLetters
	if len(driveLetters) == 0 {
		driveLetters = []string{}
	}

	// StartBackup routes through mode detection (service or direct).
	return a.StartBackup(
		job.BackupType,
		job.BackupDirs,
		driveLetters,
		job.ExcludeList,
		job.BackupID,
		job.UseVSS,
		compression,
	)
}

// executeScheduledJob claims the job and executes it in the calling goroutine.
// Callers (scheduler tick, startup runner) already run in their own goroutine;
// a job that is already running is skipped.
func (a *App) executeScheduledJob(job ScheduledJob) {
	if !claimJob(job.ID) {
		writeDebugLog(fmt.Sprintf("Job %s is already running, skipping", job.Name))
		return
	}
	defer releaseJob(job.ID)
	a.runClaimedJob(job)
}

// runClaimedJob runs the backup of an already claimed job and records the
// outcome: a job history entry, then lastRun/nextRun on the job itself. The
// caller MUST hold the job's claim (claimJob) and release it afterwards.
//
// Tests call it directly (synchronously) with scheduledBackup stubbed.
func (a *App) runClaimedJob(job ScheduledJob) {
	writeDebugLog(fmt.Sprintf("Executing scheduled job: %s", job.Name))

	// Prepare history entry (will be added at the end with final status)
	startTime := time.Now()

	// Use StartBackup to route through mode detection (service or direct)
	writeDebugLog(fmt.Sprintf("[Scheduled Job] Executing via StartBackup (mode: %s)", a.mode.String()))

	err := a.runScheduledBackup(job)

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

// RunScheduledJobNow starts a scheduled job immediately, ignoring its
// schedule — an overdue or disabled job still runs, because this is an
// explicit user action. The backup runs in the background; the returned error
// only reports why the run could NOT be started (unknown job, already
// running).
//
// In service mode the whole action is delegated to the service: it owns the
// jobs file and the job history, so lastRun/nextRun and the history entry must
// be written there — the GUI cannot write into the root-only state dir.
func (a *App) RunScheduledJobNow(jobID string) error {
	writeDebugLog(fmt.Sprintf("RunScheduledJobNow(%s) called", jobID))
	if a.isDelegatedToService() {
		return a.apiClient.RunScheduledJob(jobID)
	}
	return a.runScheduledJobNow(jobID)
}

// RunScheduledJobForAPI is the service-side entry point behind
// POST /jobs/run/{id} (see api.BackupHandler).
func (a *App) RunScheduledJobForAPI(jobID string) error {
	return a.runScheduledJobNow(jobID)
}

// runScheduledJobNow resolves jobID against the stored jobs and starts the
// run in the background. The claim is taken BEFORE spawning so a duplicate
// start is reported to the caller rather than silently skipped.
func (a *App) runScheduledJobNow(jobID string) error {
	jobs, err := a.GetScheduledJobs()
	if err != nil {
		return fmt.Errorf("failed to load scheduled jobs: %w", err)
	}
	for _, job := range jobs {
		if job.ID != jobID {
			continue
		}
		if !claimJob(job.ID) {
			return fmt.Errorf("job %q est deja en cours d'execution", job.Name)
		}
		writeDebugLog(fmt.Sprintf("Manual run of scheduled job: %s", job.Name))
		go func() {
			defer releaseJob(job.ID)
			a.runClaimedJob(job)
		}()
		return nil
	}
	return fmt.Errorf("job planifie %q introuvable", jobID)
}

// jobToMap converts a ScheduledJob to the wire map understood by
// SaveScheduledJobFromMap/UpdateScheduledJobFromMap on the service side.
//
// It is a plain JSON round-trip of the struct rather than a hand-written field
// list: the receiving end decodes the map back into a ScheduledJob, which
// IGNORES every key that does not match a json tag. The previous hand-picked
// list used "schedule"/"backup_dirs"/… (no such tags), so in service mode the
// service stored jobs with nothing but id and name — no schedule, no backup
// directories, no compression, no drive letters — and the runAtStartup,
// driveLetters and compression fields never even made it into the map.
func jobToMap(job *ScheduledJob) map[string]interface{} {
	m := map[string]interface{}{}
	if job == nil {
		return m
	}
	b, err := json.Marshal(job)
	if err != nil {
		writeDebugLog(fmt.Sprintf("jobToMap: cannot marshal job %s: %v", job.ID, err))
		return m
	}
	if err := json.Unmarshal(b, &m); err != nil {
		writeDebugLog(fmt.Sprintf("jobToMap: cannot decode job %s: %v", job.ID, err))
		return map[string]interface{}{}
	}
	return m
}

// apiJobsToSlice converts the API's []map[string]interface{} to []ScheduledJob.
// Every field is read under its canonical key first and under the historical
// snake_case alias second, so a job list coming from an older service (which
// only emitted the aliases) still decodes completely.
func apiJobsToSlice(apiJobs []map[string]interface{}) []ScheduledJob {
	out := make([]ScheduledJob, 0, len(apiJobs))
	for _, m := range apiJobs {
		j := ScheduledJob{
			ID:           firstStr(m, "id"),
			Name:         firstStr(m, "name"),
			ScheduleTime: firstStr(m, "scheduleTime", "schedule"),
			RunAtStartup: firstBool(m, "runAtStartup", "run_at_startup"),
			BackupDirs:   firstStrSlice(m, "backupDirs", "backup_dirs"),
			DriveLetters: firstStrSlice(m, "driveLetters", "drive_letters"),
			BackupID:     firstStr(m, "backupId", "backup_id"),
			UseVSS:       firstBool(m, "useVSS", "use_vss"),
			BackupType:   firstStr(m, "backupType", "backup_type"),
			ExcludeList:  firstStrSlice(m, "excludeList", "exclude_list"),
			Compression:  firstStr(m, "compression"),
			LastRun:      firstStr(m, "lastRun", "last_run"),
			NextRun:      firstStr(m, "nextRun", "next_run"),
			Enabled:      firstBool(m, "enabled"),
		}
		out = append(out, j)
	}
	return out
}

// apiHistoryToSlice converts the API's []map[string]interface{} to []JobHistory.
func apiHistoryToSlice(apiHist []map[string]interface{}) []JobHistory {
	out := make([]JobHistory, 0, len(apiHist))
	for _, m := range apiHist {
		h := JobHistory{
			ID:         getStr(m, "id"),
			Name:       getStr(m, "name"),
			Timestamp:  getStr(m, "timestamp"),
			Status:     getStr(m, "status"),
			Message:    getStr(m, "message"),
			BackupDirs: getStrSlice(m, "backup_dirs"),
			BackupID:   getStr(m, "backup_id"),
			UseVSS:     getBool(m, "use_vss"),
		}
		out = append(out, h)
	}
	return out
}

func getStr(m map[string]interface{}, k string) string {
	if v, ok := m[k].(string); ok {
		return v
	}
	return ""
}

func getBool(m map[string]interface{}, k string) bool {
	if v, ok := m[k].(bool); ok {
		return v
	}
	return false
}

func getStrSlice(m map[string]interface{}, k string) []string {
	if v, ok := m[k].([]interface{}); ok {
		out := make([]string, 0, len(v))
		for _, x := range v {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// firstStr/firstBool/firstStrSlice return the value of the first key present,
// so a decoded job map can use either this build's canonical keys or the
// historical aliases a different build emitted.
func firstStr(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k].(string); ok {
			return v
		}
	}
	return ""
}

func firstBool(m map[string]interface{}, keys ...string) bool {
	for _, k := range keys {
		if v, ok := m[k].(bool); ok {
			return v
		}
	}
	return false
}

func firstStrSlice(m map[string]interface{}, keys ...string) []string {
	for _, k := range keys {
		if v, ok := m[k].([]interface{}); ok {
			out := make([]string, 0, len(v))
			for _, x := range v {
				if s, ok := x.(string); ok {
					out = append(out, s)
				}
			}
			return out
		}
	}
	return nil
}
