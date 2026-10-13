//go:build !service

package main

import (
	"sync"
	"time"

	"github.com/tizbac/proxmoxbackupclient_go/gui/api"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// polledRuns holds the service runs this window started and follows itself
// (pollBackupProgress); watchServiceRuns leaves those alone.
var polledRuns sync.Map // job ID -> struct{}

// watchServiceRuns shows the service's own runs in this window. While the
// service runs the schedules, it lists the service's runs every 3 s and
// follows one this window did not start (a scheduled run), sending the same
// progress, pause and completion events as a run of its own: the progress
// card appears, with Pause and Stop, which go to the service.
func (a *App) watchServiceRuns() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	following := ""
	lastPaused := false
	lastFile := ""
	for range ticker.C {
		if a.ctx == nil || a.mode != api.ModeService {
			following = ""
			continue
		}
		resp, err := a.apiClient.GetRuns()
		if err != nil {
			continue
		}
		var run *api.BackupProgress
		if following != "" {
			for i := range resp.Runs {
				if resp.Runs[i].JobID == following {
					run = &resp.Runs[i]
				}
			}
			if run == nil {
				// Gone from the service (restarted): the card cannot stay.
				runtime.EventsEmit(a.ctx, "backup:complete", map[string]interface{}{
					"success": false, "message": "Lost contact with the backup in the service",
				})
				following = ""
				continue
			}
		} else {
			for i := range resp.Runs {
				r := &resp.Runs[i]
				if _, mine := polledRuns.Load(r.JobID); r.Running && !mine {
					run, following, lastPaused, lastFile = r, r.JobID, false, ""
					writeDebugLog("[Service Mode] Showing the service's run " + r.JobID + " (" + r.Name + ")")
					break
				}
			}
			if run == nil {
				continue
			}
		}

		if run.Complete {
			runtime.EventsEmit(a.ctx, "backup:complete", map[string]interface{}{
				"success": run.Success, "message": run.Message,
			})
			if lastPaused {
				a.emitPause(BackupPauseState{})
			}
			following = ""
			continue
		}
		runtime.EventsEmit(a.ctx, "backup:progress", map[string]interface{}{
			"percent": run.Progress, "message": run.Message, "name": run.Name,
		})
		a.emitRunDetail(run, &lastFile)
		if run.Paused != lastPaused {
			lastPaused = run.Paused
			a.emitPause(BackupPauseState{Paused: run.Paused, Since: run.PausedSince})
		}
	}
}

// emitRunDetail passes a service run's live statistics and current file on
// as this window's own "backup:stats" and "backup:file" events, so the card's
// Files, Processing, Upload and New/Reused rows fill in as for a run of its
// own. lastFile holds the file last sent, so an unchanged one is not resent.
func (a *App) emitRunDetail(run *api.BackupProgress, lastFile *string) {
	if run.Stats != nil {
		runtime.EventsEmit(a.ctx, "backup:stats", run.Stats)
	}
	if run.File != "" && run.File != *lastFile {
		*lastFile = run.File
		runtime.EventsEmit(a.ctx, "backup:file", map[string]interface{}{"path": run.File})
	}
}
