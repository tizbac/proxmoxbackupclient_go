//go:build !service

package main

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/tizbac/proxmoxbackupclient_go/gui/api"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// polledRuns holds the service runs this window started and follows itself
// (pollBackupProgress); watchServiceRuns leaves those alone.
var polledRuns sync.Map // job ID -> struct{}

// serviceRunsSnapshot is the service's run list as watchServiceRuns last
// fetched it, for the tray's Pause/Resume item. at is when the fetch started.
type serviceRunsSnapshot struct {
	at   time.Time
	runs *api.RunsResponse
}

// latestServiceRuns is nil while the service does not run the schedules.
var latestServiceRuns atomic.Pointer[serviceRunsSnapshot]

// followedServiceRun is what the window was last told about one service run.
type followedServiceRun struct {
	paused bool
	file   string
}

// watchServiceRuns shows the service's own runs in this window. While the
// service runs the schedules, it lists the service's runs every 3 s and
// follows every one this window did not start (scheduled runs, several at
// once), sending the same progress, pause and completion events as a run of
// its own, each under its own run id: each gets its card, with Pause and
// Stop, which go to the service.
//
// While the app holds no token (not connected yet, or dropped after a while
// in the tray: token_drop.go), the run list would only be refused: it reads
// the service's public status instead, which needs none, for the tray. Either
// request also tells whether the service is running (gui/api/answer.go).
func (a *App) watchServiceRuns() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	following := map[string]*followedServiceRun{}
	for range ticker.C {
		if a.ctx == nil || a.mode != api.ModeService {
			following = map[string]*followedServiceRun{}
			latestServiceRuns.Store(nil)
			a.notePublicStatus(nil)
			continue
		}
		if a.holdsNoToken() {
			latestServiceRuns.Store(nil)
			st, err := a.apiClient.PublicStatus()
			if err != nil {
				st = nil
			}
			a.notePublicStatus(st)
			continue
		}
		if a.notePublicStatus(nil) {
			// Connected again: the tray's own text (the next run) is back.
			a.RefreshIdleTooltip()
		}
		fetched := time.Now()
		resp, err := a.apiClient.GetRuns()
		if err != nil {
			continue
		}
		latestServiceRuns.Store(&serviceRunsSnapshot{at: fetched, runs: resp})
		listed := map[string]bool{}
		for i := range resp.Runs {
			run := &resp.Runs[i]
			listed[run.JobID] = true
			if _, mine := polledRuns.Load(run.JobID); mine {
				continue
			}
			f := following[run.JobID]
			if f == nil {
				if !run.Running {
					continue // finished before this window saw it
				}
				f = &followedServiceRun{}
				following[run.JobID] = f
				writeDebugLog("[Service Mode] Showing the service's run " + run.JobID + " (" + run.Name + ")")
			}
			id := serviceRunPrefix + run.JobID
			if run.Complete {
				runtime.EventsEmit(a.ctx, "backup:complete", map[string]interface{}{
					"runId": id, "name": run.Name, "success": run.Success, "message": run.Message,
				})
				if f.paused {
					a.emitPause(id, BackupPauseState{})
				}
				delete(following, run.JobID)
				continue
			}
			runtime.EventsEmit(a.ctx, "backup:progress", map[string]interface{}{
				"runId": id, "percent": run.Progress, "message": run.Message, "name": run.Name,
			})
			a.emitRunDetail(run, &f.file, id)
			if run.Paused != f.paused {
				f.paused = run.Paused
				a.emitPause(id, BackupPauseState{Paused: run.Paused, Since: run.PausedSince})
			}
		}
		for jobID := range following {
			if !listed[jobID] {
				// Gone from the service (restarted): its card cannot stay.
				runtime.EventsEmit(a.ctx, "backup:complete", map[string]interface{}{
					"runId": serviceRunPrefix + jobID, "success": false, "message": "Lost contact with the backup in the service",
				})
				delete(following, jobID)
			}
		}
	}
}

// emitRunDetail passes a service run's live statistics and current file on
// as this window's own "backup:stats" and "backup:file" events, under the
// run's id, so the card's Files, Processing, Upload and New/Reused rows fill
// in as for a run of its own. lastFile holds the file last sent, so an
// unchanged one is not resent.
func (a *App) emitRunDetail(run *api.BackupProgress, lastFile *string, runID string) {
	if run.Stats != nil {
		stats := make(map[string]interface{}, len(run.Stats)+1)
		for k, v := range run.Stats {
			stats[k] = v
		}
		stats["runId"] = runID
		runtime.EventsEmit(a.ctx, "backup:stats", stats)
	}
	if run.File != "" && run.File != *lastFile {
		*lastFile = run.File
		runtime.EventsEmit(a.ctx, "backup:file", map[string]interface{}{"runId": runID, "path": run.File})
	}
}
