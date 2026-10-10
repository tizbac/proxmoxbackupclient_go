package main

import (
	"fmt"
	"time"

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
	if a.ctx != nil {
		st := currentPauseState()
		runtime.EventsEmit(a.ctx, "backup:paused", map[string]interface{}{"paused": st.Paused, "since": st.Since})
	}
}

// PauseBackup pauses the running backup at its next safe point: between
// files, between 64 KiB blocks of a large file, or between disk-image blocks.
func (a *App) PauseBackup() (BackupPauseState, error) {
	currentBackupCancelMutex.Lock()
	running := currentBackupCancel != nil
	currentBackupCancelMutex.Unlock()
	if !running {
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
	if d, ok := backupPause.Resume(); ok {
		writeBackupLog(fmt.Sprintf("Backup resumed after a pause of %s", d.Round(time.Second)))
		a.emitPauseState()
	}
	return currentPauseState(), nil
}

// GetBackupPauseState lets a reloaded GUI pick up a pause already in force.
func (a *App) GetBackupPauseState() BackupPauseState { return currentPauseState() }
