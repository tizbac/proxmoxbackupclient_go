package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

// newSchedulerTestApp pins a PRIVATE config dir (scheduled_jobs.json and
// job_history.json must never leak between tests) and returns an App with no
// api client, i.e. standalone: nothing is delegated to a service, every call
// goes through the local files — the same code path a service process uses.
func newSchedulerTestApp(t *testing.T) *App {
	t.Helper()
	SetConfigDir(t.TempDir())
	t.Cleanup(func() { SetConfigDir("") })
	return &App{config: &Config{PBSServers: make(map[string]*PBSServer)}}
}

// sampleJob is a job using every field of the struct — the wire tests below
// compare against it field by field, so a field missing from a map is a failure.
func sampleJob() ScheduledJob {
	return ScheduledJob{
		ID:           "job-1",
		Name:         "Nightly machine backup",
		ScheduleTime: "02:30",
		RunAtStartup: true,
		BackupDirs:   []string{"/data", "/etc/app"},
		DriveLetters: []string{"C:", "D:"},
		BackupID:     "workstation-01",
		UseVSS:       true,
		BackupType:   "machine",
		ExcludeList:  []string{"*.tmp", "cache/"},
		Compression:  "better",
		Enabled:      true, // a stored job is enabled (SaveScheduledJob forces it)
	}
}

// writeJobs replaces the jobs file (used to install fixtures such as an
// overdue nextRun that SaveScheduledJob would recompute).
func writeJobs(t *testing.T, jobs []ScheduledJob) {
	t.Helper()
	path, err := getScheduledJobsPath()
	if err != nil {
		t.Fatalf("getScheduledJobsPath: %v", err)
	}
	data, err := json.MarshalIndent(jobs, "", "  ")
	if err != nil {
		t.Fatalf("marshal jobs: %v", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatalf("write jobs: %v", err)
	}
}

// waitFor polls cond until it holds (or the deadline expires).
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// waitForJobDone waits for an asynchronous run to be completely over: lastRun
// is the last thing a run persists, and the released claim means the runner
// goroutine no longer touches any global state (so it cannot race with the
// next test under -race).
func waitForJobDone(t *testing.T, a *App, jobID string) {
	t.Helper()
	waitFor(t, fmt.Sprintf("lastRun of job %s", jobID), func() bool {
		jobs, err := a.GetScheduledJobs()
		if err != nil {
			return false
		}
		for _, j := range jobs {
			if j.ID == jobID && j.LastRun != "" {
				return true
			}
		}
		return false
	})
	waitFor(t, fmt.Sprintf("claim of job %s to be released", jobID), func() bool {
		runningJobsMutex.Lock()
		defer runningJobsMutex.Unlock()
		return !runningJobs[jobID]
	})
}

// --- saving jobs -----------------------------------------------------------

// Saving a job and reading it back must preserve every field: a job whose
// schedule, directories, compression or drive letters are lost on save runs
// the wrong backup (or never runs at all).
func TestSaveScheduledJobKeepsEveryField(t *testing.T) {
	a := newSchedulerTestApp(t)

	job := sampleJob()
	if err := a.SaveScheduledJob(job); err != nil {
		t.Fatalf("SaveScheduledJob() error = %v", err)
	}

	jobs, err := a.GetScheduledJobs()
	if err != nil {
		t.Fatalf("GetScheduledJobs() error = %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("got %d jobs, want 1", len(jobs))
	}
	got := jobs[0]

	if got.ScheduleTime != job.ScheduleTime {
		t.Errorf("ScheduleTime = %q, want %q", got.ScheduleTime, job.ScheduleTime)
	}
	if !got.RunAtStartup {
		t.Errorf("RunAtStartup = false, want true")
	}
	if !reflect.DeepEqual(got.BackupDirs, job.BackupDirs) {
		t.Errorf("BackupDirs = %v, want %v", got.BackupDirs, job.BackupDirs)
	}
	if !reflect.DeepEqual(got.DriveLetters, job.DriveLetters) {
		t.Errorf("DriveLetters = %v, want %v", got.DriveLetters, job.DriveLetters)
	}
	if got.Compression != job.Compression {
		t.Errorf("Compression = %q, want %q", got.Compression, job.Compression)
	}
	if !reflect.DeepEqual(got.ExcludeList, job.ExcludeList) {
		t.Errorf("ExcludeList = %v, want %v", got.ExcludeList, job.ExcludeList)
	}
	if got.BackupType != job.BackupType || got.BackupID != job.BackupID {
		t.Errorf("BackupType/BackupID = %q/%q, want %q/%q", got.BackupType, got.BackupID, job.BackupType, job.BackupID)
	}
	if !got.UseVSS {
		t.Errorf("UseVSS = false, want true")
	}
	if !got.Enabled {
		t.Errorf("Enabled = false, want true (a new job is enabled)")
	}
	if got.NextRun == "" {
		t.Errorf("NextRun = empty, want a calculated next run")
	} else if _, err := time.Parse(time.RFC3339, got.NextRun); err != nil {
		t.Errorf("NextRun %q is not RFC3339: %v", got.NextRun, err)
	}
}

// The service-side wire (jobToMap → json → ScheduledJob) is how the GUI in
// service mode creates/updates jobs. The receiving end decodes with the struct
// tags, so any key that isn't a json tag is silently dropped — which used to
// leave service-stored jobs with nothing but an id and a name.
func TestJobWireMapDecodesBackIntoEveryField(t *testing.T) {
	job := sampleJob()
	job.LastRun = "2026-01-02T03:04:05Z"
	job.NextRun = "2026-01-03T02:30:00Z"
	job.Enabled = true

	m := jobToMap(&job)
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal wire map: %v", err)
	}
	var back ScheduledJob
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("decode wire map: %v", err)
	}

	if !reflect.DeepEqual(back, job) {
		t.Errorf("wire round-trip changed the job:\n got %+v\nwant %+v", back, job)
	}
}

// The service list endpoint must hand the GUI its jobs unchanged, and still
// expose the historical snake_case aliases older GUI builds read.
func TestGetScheduledJobsForAPIRoundTrip(t *testing.T) {
	a := newSchedulerTestApp(t)
	if err := a.SaveScheduledJob(sampleJob()); err != nil {
		t.Fatalf("SaveScheduledJob() error = %v", err)
	}

	list := a.GetScheduledJobsForAPI()
	if len(list) != 1 {
		t.Fatalf("got %d API jobs, want 1", len(list))
	}

	want := list0(t, a)
	back := apiJobsToSlice(list)
	if len(back) != 1 {
		t.Fatalf("got %d decoded jobs, want 1", len(back))
	}
	if !reflect.DeepEqual(back[0], want) {
		t.Errorf("API round-trip changed the job:\n got %+v\nwant %+v", back[0], want)
	}

	// Legacy aliases kept for older GUI builds.
	for _, key := range []string{"backup_type", "backup_id", "schedule", "use_vss", "backup_dirs", "exclude_list", "last_run", "next_run"} {
		if _, ok := list[0][key]; !ok {
			t.Errorf("API job map is missing the legacy key %q", key)
		}
	}
	if list[0]["schedule"] != "02:30" {
		t.Errorf("legacy schedule alias = %v, want 02:30", list[0]["schedule"])
	}
}

// list0 returns the single stored job as saved (the reference for round-trips).
func list0(t *testing.T, a *App) ScheduledJob {
	t.Helper()
	jobs, err := a.GetScheduledJobs()
	if err != nil {
		t.Fatalf("GetScheduledJobs() error = %v", err)
	}
	if len(jobs) != 1 {
		t.Fatalf("got %d jobs, want 1", len(jobs))
	}
	return jobs[0]
}

// A job that could never run as configured is rejected at save time, and the
// rejection leaves the stored jobs untouched.
func TestSaveScheduledJobValidates(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*ScheduledJob)
		wantErr string
	}{
		{"missing name", func(j *ScheduledJob) { j.Name = "  " }, "name requis"},
		{"unparseable schedule", func(j *ScheduledJob) { j.ScheduleTime = "tomorrow" }, "HH:MM"},
		{"hour out of range", func(j *ScheduledJob) { j.ScheduleTime = "25:00" }, "HH:MM"},
		{"minute out of range", func(j *ScheduledJob) { j.ScheduleTime = "02:75" }, "HH:MM"},
		{"no schedule and no startup run", func(j *ScheduledJob) { j.ScheduleTime = ""; j.RunAtStartup = false }, "planification requise"},
		{"nothing to back up", func(j *ScheduledJob) { j.BackupDirs = nil; j.DriveLetters = nil }, "repertoire ou un disque"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newSchedulerTestApp(t)
			if err := a.SaveScheduledJob(sampleJob()); err != nil {
				t.Fatalf("SaveScheduledJob() error = %v", err)
			}
			before := readJobsFile(t)

			job := sampleJob()
			job.ID = "job-bad"
			job.Name = "Broken"
			tt.mutate(&job)

			err := a.SaveScheduledJob(job)
			if err == nil {
				t.Fatalf("SaveScheduledJob() accepted an invalid job")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %q, want it to contain %q", err, tt.wantErr)
			}
			if after := readJobsFile(t); after != before {
				t.Errorf("rejected save modified the jobs file:\n got %s\nwant %s", after, before)
			}
		})
	}
}

func readJobsFile(t *testing.T) string {
	t.Helper()
	path, err := getScheduledJobsPath()
	if err != nil {
		t.Fatalf("getScheduledJobsPath: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read jobs: %v", err)
	}
	return string(data)
}

// An update must not erase the run record: the edit form never carries
// lastRun (nor the enabled flag), so a save would otherwise wipe them.
func TestUpdateScheduledJobKeepsLastRunAndEnabled(t *testing.T) {
	a := newSchedulerTestApp(t)
	if err := a.SaveScheduledJob(sampleJob()); err != nil {
		t.Fatalf("SaveScheduledJob() error = %v", err)
	}

	jobs := readStoredJobs(t)
	jobs[0].Enabled = false
	jobs[0].LastRun = "2026-01-02T03:04:05Z"
	writeJobs(t, jobs)

	updated := jobs[0]
	updated.ScheduleTime = "04:45"
	if err := a.UpdateScheduledJob(updated); err != nil {
		t.Fatalf("UpdateScheduledJob() error = %v", err)
	}

	got := list0(t, a)
	if got.LastRun != "2026-01-02T03:04:05Z" {
		t.Errorf("LastRun = %q, want it preserved", got.LastRun)
	}
	if got.Enabled {
		t.Errorf("Enabled = true, want it preserved (false)")
	}
	if got.ScheduleTime != "04:45" {
		t.Errorf("ScheduleTime = %q, want 04:45", got.ScheduleTime)
	}
	if got.NextRun == "" {
		t.Errorf("NextRun = empty, want it recalculated for the new schedule")
	}
}

func readStoredJobs(t *testing.T) []ScheduledJob {
	t.Helper()
	path, err := getScheduledJobsPath()
	if err != nil {
		t.Fatalf("getScheduledJobsPath: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read jobs: %v", err)
	}
	var jobs []ScheduledJob
	if err := json.Unmarshal(data, &jobs); err != nil {
		t.Fatalf("decode jobs: %v", err)
	}
	return jobs
}

// A corrupt jobs file must never be replaced by a one-entry file: the save has
// to fail instead of destroying every stored job.
func TestSaveScheduledJobRefusesToOverwriteUnreadableFile(t *testing.T) {
	a := newSchedulerTestApp(t)
	path, err := getScheduledJobsPath()
	if err != nil {
		t.Fatalf("getScheduledJobsPath: %v", err)
	}
	if err := os.WriteFile(path, []byte("{ not json"), 0600); err != nil {
		t.Fatalf("write corrupt jobs: %v", err)
	}

	job := sampleJob()
	if err := a.SaveScheduledJob(job); err == nil {
		t.Fatalf("SaveScheduledJob() succeeded on a corrupt jobs file")
	}
	if got := readJobsFile(t); got != "{ not json" {
		t.Errorf("jobs file = %q, want it untouched", got)
	}
}

// --- running jobs ----------------------------------------------------------

// The backup of a scheduled run must receive the job's stored parameters —
// they are exactly what the previous wire dropped.
func TestRunClaimedJobPassesJobParameters(t *testing.T) {
	a := newSchedulerTestApp(t)
	job := sampleJob()
	job.Enabled = true

	var mu sync.Mutex
	var seen ScheduledJob
	a.scheduledBackupFn = func(j ScheduledJob) error {
		mu.Lock()
		defer mu.Unlock()
		seen = j
		return nil
	}

	a.runClaimedJob(job)

	mu.Lock()
	defer mu.Unlock()
	if !reflect.DeepEqual(seen, job) {
		t.Errorf("backup ran with %+v, want %+v", seen, job)
	}
}

func TestRunClaimedJobRecordsSuccess(t *testing.T) {
	a := newSchedulerTestApp(t)
	if err := a.SaveScheduledJob(sampleJob()); err != nil {
		t.Fatalf("SaveScheduledJob() error = %v", err)
	}
	job := list0(t, a)

	a.scheduledBackupFn = func(ScheduledJob) error { return nil }
	a.runClaimedJob(job)

	history, err := a.GetJobHistory()
	if err != nil {
		t.Fatalf("GetJobHistory() error = %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("got %d history entries, want 1", len(history))
	}
	if history[0].Status != "success" {
		t.Errorf("history status = %q, want success (%s)", history[0].Status, history[0].Message)
	}
	if history[0].Name != job.Name {
		t.Errorf("history name = %q, want %q", history[0].Name, job.Name)
	}

	got := list0(t, a)
	if got.LastRun == "" {
		t.Errorf("LastRun = empty, want a timestamp")
	}
	if _, err := time.Parse(time.RFC3339, got.LastRun); err != nil {
		t.Errorf("LastRun %q is not RFC3339: %v", got.LastRun, err)
	}
	if got.NextRun == "" {
		t.Errorf("NextRun = empty, want it rescheduled")
	}
}

// A failed backup must be recorded as failed, with the reason — and the job
// must still be rescheduled instead of being stuck on its last run.
func TestRunClaimedJobRecordsFailure(t *testing.T) {
	a := newSchedulerTestApp(t)
	if err := a.SaveScheduledJob(sampleJob()); err != nil {
		t.Fatalf("SaveScheduledJob() error = %v", err)
	}
	job := list0(t, a)

	a.scheduledBackupFn = func(ScheduledJob) error { return fmt.Errorf("pbs refused the snapshot") }
	a.runClaimedJob(job)

	history, err := a.GetJobHistory()
	if err != nil {
		t.Fatalf("GetJobHistory() error = %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("got %d history entries, want 1", len(history))
	}
	if history[0].Status != "failed" {
		t.Errorf("history status = %q, want failed", history[0].Status)
	}
	if !strings.Contains(history[0].Message, "pbs refused the snapshot") {
		t.Errorf("history message = %q, want the backup error", history[0].Message)
	}
	if got := list0(t, a); got.NextRun == "" {
		t.Errorf("NextRun = empty, want the failed job rescheduled")
	}
}

// One job can only run once at a time: the claim is the shared guard between
// the scheduler tick, the startup runner and "run now".
func TestExecuteScheduledJobSkipsAlreadyRunningJob(t *testing.T) {
	a := newSchedulerTestApp(t)
	job := sampleJob()

	var calls int
	a.scheduledBackupFn = func(ScheduledJob) error {
		calls++
		return nil
	}

	if !claimJob(job.ID) {
		t.Fatalf("claimJob() = false, want true")
	}
	defer releaseJob(job.ID)

	a.executeScheduledJob(job)

	if calls != 0 {
		t.Errorf("backup ran %d times while the job was claimed, want 0", calls)
	}
}

// A job whose next run has passed (machine asleep, service stopped…) must be
// caught up on the next tick; a future or disabled job must not run.
func TestCheckAndRunScheduledJobsFiresOnlyDueJobs(t *testing.T) {
	a := newSchedulerTestApp(t)

	overdue := sampleJob()
	overdue.ID = "overdue"
	future := sampleJob()
	future.ID = "future"
	future.ScheduleTime = "23:59"
	disabled := sampleJob()
	disabled.ID = "disabled"
	disabled.Enabled = false

	now := time.Now()
	overdue.NextRun = now.Add(-time.Hour).Format(time.RFC3339)
	future.NextRun = now.Add(time.Hour).Format(time.RFC3339)
	disabled.NextRun = now.Add(-time.Hour).Format(time.RFC3339)

	var mu sync.Mutex
	ran := map[string]int{}
	a.scheduledBackupFn = func(j ScheduledJob) error {
		mu.Lock()
		defer mu.Unlock()
		ran[j.ID]++
		return nil
	}

	writeJobs(t, []ScheduledJob{overdue, future, disabled})
	a.checkAndRunScheduledJobs()

	waitForJobDone(t, a, "overdue")
	mu.Lock()
	defer mu.Unlock()
	if ran["overdue"] != 1 {
		t.Errorf("overdue job ran %d times, want 1", ran["overdue"])
	}
	if ran["future"] != 0 {
		t.Errorf("future job ran %d times, want 0", ran["future"])
	}
	if ran["disabled"] != 0 {
		t.Errorf("disabled job ran %d times, want 0", ran["disabled"])
	}
}

// --- run now ---------------------------------------------------------------

// "Run now" starts the job immediately, regardless of its schedule.
func TestRunScheduledJobNowIgnoresSchedule(t *testing.T) {
	a := newSchedulerTestApp(t)
	if err := a.SaveScheduledJob(sampleJob()); err != nil {
		t.Fatalf("SaveScheduledJob() error = %v", err)
	}
	job := list0(t, a)
	if job.NextRun == "" {
		t.Fatalf("job has no next run")
	}

	ran := make(chan struct{}, 1)
	a.scheduledBackupFn = func(j ScheduledJob) error {
		ran <- struct{}{}
		return nil
	}

	if err := a.RunScheduledJobNow(job.ID); err != nil {
		t.Fatalf("RunScheduledJobNow() error = %v", err)
	}

	select {
	case <-ran:
	case <-time.After(5 * time.Second):
		t.Fatalf("job did not start")
	}
	waitForJobDone(t, a, job.ID)

	if got := list0(t, a); got.LastRun == "" {
		t.Errorf("LastRun = empty, want the manual run recorded")
	}
}

func TestRunScheduledJobNowUnknownJob(t *testing.T) {
	a := newSchedulerTestApp(t)
	err := a.RunScheduledJobNow("nope")
	if err == nil {
		t.Fatalf("RunScheduledJobNow() succeeded for an unknown job")
	}
	if !strings.Contains(err.Error(), "introuvable") {
		t.Errorf("error = %q, want it to mention the unknown job", err)
	}
}

// A second start of a running job is reported to the caller instead of being
// silently swallowed.
func TestRunScheduledJobNowRejectsRunningJob(t *testing.T) {
	a := newSchedulerTestApp(t)
	if err := a.SaveScheduledJob(sampleJob()); err != nil {
		t.Fatalf("SaveScheduledJob() error = %v", err)
	}
	job := list0(t, a)

	if !claimJob(job.ID) {
		t.Fatalf("claimJob() = false, want true")
	}
	defer releaseJob(job.ID)

	var calls int
	a.scheduledBackupFn = func(ScheduledJob) error {
		calls++
		return nil
	}

	err := a.RunScheduledJobNow(job.ID)
	if err == nil {
		t.Fatalf("RunScheduledJobNow() accepted a job that is already running")
	}
	if !strings.Contains(err.Error(), "deja en cours") {
		t.Errorf("error = %q, want it to report the running job", err)
	}
	if calls != 0 {
		t.Errorf("backup ran %d times, want 0", calls)
	}
}

// calculateNextRun rejects schedules that can never fire and computes a future
// time for valid ones (a DST-safe calendar day ahead once today's time passed).
func TestCalculateNextRun(t *testing.T) {
	if got := calculateNextRun(""); got != "" {
		t.Errorf("calculateNextRun(%q) = %q, want empty", "", got)
	}
	if got := calculateNextRun("25:00"); got != "" {
		t.Errorf("calculateNextRun(%q) = %q, want empty", "25:00", got)
	}
	if got := calculateNextRun("not-a-time"); got != "" {
		t.Errorf("calculateNextRun(%q) = %q, want empty", "not-a-time", got)
	}

	next := calculateNextRun("02:30")
	if next == "" {
		t.Fatalf("calculateNextRun(02:30) = empty, want a timestamp")
	}
	parsed, err := time.Parse(time.RFC3339, next)
	if err != nil {
		t.Fatalf("calculateNextRun(02:30) = %q, not RFC3339: %v", next, err)
	}
	if !parsed.After(time.Now()) {
		t.Errorf("next run %s is not in the future", next)
	}
	if parsed.Hour() != 2 || parsed.Minute() != 30 {
		t.Errorf("next run %s is not at 02:30", next)
	}
}
