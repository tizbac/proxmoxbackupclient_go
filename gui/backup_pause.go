package main

import (
	"fmt"
	"time"

	"github.com/tizbac/proxmoxbackupclient_go/gui/api"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"pbscommon"
)

// backupPause is the Pause button's gate for the in-flight backup (see
// pbscommon/pause.go). The walk, the large-file read loop and the disk-image
// reader wait on it, so a paused run keeps its PBS session and carries on
// from the same place on Resume. Stop still works while paused. A pause
// belongs to the run it was pressed in: newBackupContext and
// doneBackupContext open the gate again.
var backupPause = &pbscommon.PauseGate{}

// BackupPauseState is what the GUI needs to show a paused run.
type BackupPauseState struct {
	Paused bool  `json:"paused"`
	Since  int64 `json:"since"` // Unix ms; 0 when running
}

func currentPauseState() BackupPauseState {
	paused, since := backupPause.Paused()
	if !paused {
		return BackupPauseState{}
	}
	return BackupPauseState{Paused: true, Since: since.UnixMilli()}
}

func (a *App) emitPauseState() {
	a.emitPause(currentPauseState())
}

// emitPause tells the window a run's pause state. The service process has
// no window.
func (a *App) emitPause(st BackupPauseState) {
	if a.ctx != nil && !a.isServiceProcess && !a.headless {
		runtime.EventsEmit(a.ctx, "backup:paused", map[string]interface{}{"paused": st.Paused, "since": st.Since})
	}
}

// localBackupRunning reports whether this process runs a backup.
func localBackupRunning() bool {
	currentBackupCancelMutex.Lock()
	defer currentBackupCancelMutex.Unlock()
	return currentBackupCancel != nil
}

// serviceRunsBackups reports whether the backups this window shows run in
// the background service (Run as Service), so Pause, Resume and Stop go to
// it.
func (a *App) serviceRunsBackups() bool {
	return !a.isServiceProcess && a.mode == api.ModeService && a.apiClient != nil
}

// controlServiceRun sends "pause", "resume" or "stop" to the service and
// shows the pause state it reports.
func (a *App) controlServiceRun(action string) (BackupPauseState, error) {
	st, err := a.apiClient.ControlRun(action)
	if err != nil {
		return BackupPauseState{}, err
	}
	out := BackupPauseState{Paused: st.Paused, Since: st.PausedSince}
	a.emitPause(out)
	return out, nil
}

// PauseBackup pauses the running backup at its next safe point: between
// files, between 64 KiB blocks of a large file, or between disk-image blocks.
// A backup the service runs is paused there.
func (a *App) PauseBackup() (BackupPauseState, error) {
	if !localBackupRunning() {
		if a.serviceRunsBackups() {
			return a.controlServiceRun("pause")
		}
		return currentPauseState(), fmt.Errorf("no backup is running")
	}
	if backupPause.Pause() {
		writeBackupLog("Backup paused by the user (the PBS session stays open)")
		a.emitPauseState()
	}
	return currentPauseState(), nil
}

// ResumeBackup lets a paused backup carry on.
func (a *App) ResumeBackup() (BackupPauseState, error) {
	if !localBackupRunning() && a.serviceRunsBackups() {
		return a.controlServiceRun("resume")
	}
	if d, ok := backupPause.Resume(); ok {
		writeBackupLog(fmt.Sprintf("Backup resumed after a pause of %s", d.Round(time.Second)))
		a.emitPauseState()
	}
	return currentPauseState(), nil
}

// GetBackupPauseState lets a reloaded GUI pick up a pause already in force,
// also on a run of the service.
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
	return currentPauseState()
}

// The service's side of the same buttons (api.RunController): the local API
// calls these for a GUI, on whichever run the service has in progress.

// PauseRun pauses the service's running backup.
func (a *App) PauseRun() error {
	if !localBackupRunning() {
		return fmt.Errorf("no backup is running")
	}
	if backupPause.Pause() {
		writeBackupLog("Backup paused from the app (the PBS session stays open)")
	}
	return nil
}

// ResumeRun lets the service's paused backup carry on.
func (a *App) ResumeRun() error {
	if d, ok := backupPause.Resume(); ok {
		writeBackupLog(fmt.Sprintf("Backup resumed from the app after a pause of %s", d.Round(time.Second)))
	}
	return nil
}

// StopRun stops the service's running backup (after the current file).
func (a *App) StopRun() error {
	if !localBackupRunning() {
		return fmt.Errorf("no backup is running")
	}
	writeBackupLog("Backup stopped from the app")
	return a.CancelBackup()
}

// RunPauseState reports the service's pause state (Unix ms since).
func (a *App) RunPauseState() (bool, int64) {
	st := currentPauseState()
	return st.Paused, st.Since
}
