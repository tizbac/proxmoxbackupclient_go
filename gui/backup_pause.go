package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/salvanza/salvanza_client/gui/api"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Each run has its own Pause gate (backupRun.pause, see pbscommon/pause.go).
// The walk, the large-file read loop and the disk-image reader wait on it, so
// a paused run keeps its PBS session and carries on from the same place on
// Resume. Stop still works while paused. Pausing one run leaves the others
// running.

// BackupPauseState is what the GUI needs to show a paused run.
type BackupPauseState struct {
	Paused bool  `json:"paused"`
	Since  int64 `json:"since"` // Unix ms; 0 when running
}

// serviceRunPrefix marks, in the window, a run the background service is
// doing: "svc:" + its API job id.
const serviceRunPrefix = "svc:"

func pauseStateOf(r *backupRun) BackupPauseState {
	if r == nil {
		return BackupPauseState{}
	}
	paused, since := r.pause.Paused()
	if !paused {
		return BackupPauseState{}
	}
	return BackupPauseState{Paused: true, Since: since.UnixMilli()}
}

// emitPause tells the window a run's pause state. The service process has
// no window.
func (a *App) emitPause(runID string, st BackupPauseState) {
	if a.ctx != nil && !a.isServiceProcess && !a.headless {
		runtime.EventsEmit(a.ctx, "backup:paused", map[string]interface{}{"runId": runID, "paused": st.Paused, "since": st.Since})
	}
}

// serviceRunsBackups reports whether the backups this window shows run in
// the background service (Run as Service), so Pause, Resume and Stop go to
// it.
func (a *App) serviceRunsBackups() bool {
	return !a.isServiceProcess && a.mode == api.ModeService && a.apiClient != nil
}

// controlServiceRun sends "pause", "resume" or "stop" to the service for its
// run jobID ("" for all its runs) and shows the pause state it reports.
func (a *App) controlServiceRun(action, jobID string) (BackupPauseState, error) {
	st, err := a.apiClient.ControlRun(action, jobID)
	if err != nil {
		return BackupPauseState{}, err
	}
	out := BackupPauseState{Paused: st.Paused, Since: st.PausedSince}
	if jobID != "" {
		a.emitPause(serviceRunPrefix+jobID, out)
	}
	return out, nil
}

// BackupRunInfo is one backup the window can show and control.
type BackupRunInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	BackupID string `json:"backupId"`
	Queued   bool   `json:"queued"`
	Paused   bool   `json:"paused"`
	Since    int64  `json:"since"`
	Started  int64  `json:"started"` // Unix ms
}

// GetBackupRuns lists the backups running or queued, oldest first: this
// process's own, or the service's when it runs them. A window opened (or
// reloaded) during runs builds its cards from this.
func (a *App) GetBackupRuns() []BackupRunInfo {
	out := []BackupRunInfo{}
	for _, r := range activeBackupRuns() {
		st := pauseStateOf(r)
		out = append(out, BackupRunInfo{ID: r.id, Name: r.name, Kind: r.kind, BackupID: r.backup,
			Queued: r.queued.Load(), Paused: st.Paused, Since: st.Since, Started: r.started.UnixMilli()})
	}
	if a.serviceRunsBackups() {
		if runs, err := a.apiClient.GetRuns(); err == nil {
			for _, p := range runs.Runs {
				if !p.Running {
					continue
				}
				started := int64(0)
				if t, err := time.Parse(time.RFC3339, p.StartTime); err == nil {
					started = t.UnixMilli()
				}
				out = append(out, BackupRunInfo{ID: serviceRunPrefix + p.JobID, Name: p.Name, Queued: strings.HasPrefix(p.Message, "Queued: "),
					Paused: p.Paused, Since: p.PausedSince, Started: started})
			}
		}
	}
	return out
}

// PauseBackupRun pauses one backup (id from GetBackupRuns or its events) at
// its next safe point: between files, between 64 KiB blocks of a large file,
// or between disk-image blocks.
func (a *App) PauseBackupRun(id string) (BackupPauseState, error) {
	if strings.HasPrefix(id, serviceRunPrefix) {
		return a.controlServiceRun("pause", strings.TrimPrefix(id, serviceRunPrefix))
	}
	r := backupRunByID(id)
	if r == nil {
		return BackupPauseState{}, fmt.Errorf("that backup is no longer running")
	}
	if r.pause.Pause() {
		runLog(r.logCtx(), "Backup paused by the user (the PBS session stays open)")
	}
	st := pauseStateOf(r)
	a.emitPause(r.id, st)
	return st, nil
}

// ResumeBackupRun lets one paused backup carry on.
func (a *App) ResumeBackupRun(id string) (BackupPauseState, error) {
	if strings.HasPrefix(id, serviceRunPrefix) {
		return a.controlServiceRun("resume", strings.TrimPrefix(id, serviceRunPrefix))
	}
	r := backupRunByID(id)
	if r == nil {
		return BackupPauseState{}, fmt.Errorf("that backup is no longer running")
	}
	if d, ok := r.pause.Resume(); ok {
		runLog(r.logCtx(), fmt.Sprintf("Backup resumed after a pause of %s", d.Round(time.Second)))
	}
	a.emitPause(r.id, BackupPauseState{})
	return BackupPauseState{}, nil
}

// CancelBackupRun stops one backup: a queued one at once, a running one at a
// safe point (between files, so a large file in progress is finished first),
// without committing a partial snapshot.
func (a *App) CancelBackupRun(id string) error {
	if strings.HasPrefix(id, serviceRunPrefix) {
		_, err := a.controlServiceRun("stop", strings.TrimPrefix(id, serviceRunPrefix))
		return err
	}
	r := backupRunByID(id)
	if r == nil {
		return fmt.Errorf("that backup is no longer running")
	}
	stopRun(r)
	return nil
}

func stopRun(r *backupRun) {
	writeDebugLog(fmt.Sprintf("Stop requested for %s (%s)", r.name, r.id))
	runLog(r.logCtx(), "Stop requested by the user (stops after the current file)")
	r.cancel()
}

// PauseBackup pauses every backup running here (the tray's Pause, and an
// older window's). Backups the service runs are paused there.
func (a *App) PauseBackup() (BackupPauseState, error) {
	if !localBackupRunning() {
		if a.serviceRunsBackups() {
			return a.controlServiceRun("pause", "")
		}
		return BackupPauseState{}, fmt.Errorf("no backup is running")
	}
	var st BackupPauseState
	for _, r := range activeBackupRuns() {
		if s, err := a.PauseBackupRun(r.id); err == nil {
			st = s
		}
	}
	return st, nil
}

// ResumeBackup lets every paused backup carry on.
func (a *App) ResumeBackup() (BackupPauseState, error) {
	if !localBackupRunning() && a.serviceRunsBackups() {
		return a.controlServiceRun("resume", "")
	}
	for _, r := range activeBackupRuns() {
		_, _ = a.ResumeBackupRun(r.id)
	}
	return BackupPauseState{}, nil
}

// CancelBackup stops every backup running or queued here (the tray's Stop,
// and an older window's). Backups the service runs are stopped there.
func (a *App) CancelBackup() error {
	if !localBackupRunning() && a.serviceRunsBackups() {
		_, err := a.controlServiceRun("stop", "")
		return err
	}
	runs := activeBackupRuns()
	if len(runs) == 0 {
		writeDebugLog("CancelBackup: no backup running (or already cancelled)")
	}
	for _, r := range runs {
		stopRun(r)
	}
	return nil
}

// anyRunPaused reports the pause state of the first paused run, if any.
func anyRunPaused() BackupPauseState {
	for _, r := range activeBackupRuns() {
		if st := pauseStateOf(r); st.Paused {
			return st
		}
	}
	return BackupPauseState{}
}

// GetBackupPauseState lets a reloaded GUI pick up a pause already in force,
// also on a run of the service. With several runs it reports the first
// paused one; GetBackupRuns gives each run's own.
func (a *App) GetBackupPauseState() BackupPauseState {
	if !localBackupRunning() && a.serviceRunsBackups() {
		if runs, err := a.apiClient.GetRuns(); err == nil {
			for _, r := range runs.Runs {
				if r.Running && r.Paused {
					return BackupPauseState{Paused: true, Since: r.PausedSince}
				}
			}
		}
		return BackupPauseState{}
	}
	return anyRunPaused()
}

// The service's side of the same buttons (api.RunController): the local API
// calls these for a GUI. jobID is the run's API entry; "" means every run.

// PauseRun pauses the service's backup jobID.
func (a *App) PauseRun(jobID string) error {
	runs := runsFor(jobID)
	if len(runs) == 0 {
		return fmt.Errorf("no backup is running")
	}
	for _, r := range runs {
		if r.pause.Pause() {
			runLog(r.logCtx(), "Backup paused from the app (the PBS session stays open)")
		}
	}
	return nil
}

// ResumeRun lets the service's paused backup jobID carry on.
func (a *App) ResumeRun(jobID string) error {
	for _, r := range runsFor(jobID) {
		if d, ok := r.pause.Resume(); ok {
			runLog(r.logCtx(), fmt.Sprintf("Backup resumed from the app after a pause of %s", d.Round(time.Second)))
		}
	}
	return nil
}

// StopRun stops the service's backup jobID (after the current file).
func (a *App) StopRun(jobID string) error {
	runs := runsFor(jobID)
	if len(runs) == 0 {
		return fmt.Errorf("no backup is running")
	}
	for _, r := range runs {
		stopRun(r)
	}
	return nil
}

// RunPauseState reports whether the service's backup jobID is paused, and
// since when (Unix ms); for "" the first paused run.
func (a *App) RunPauseState(jobID string) (bool, int64) {
	if jobID == "" {
		st := anyRunPaused()
		return st.Paused, st.Since
	}
	st := pauseStateOf(backupRunByID(jobID))
	return st.Paused, st.Since
}
