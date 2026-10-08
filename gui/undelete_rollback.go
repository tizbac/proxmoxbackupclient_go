//go:build !service

package main

// Undelete (bring back files that are in a Backup Set's snapshots but gone
// from disk) and Roll back (put a Backup Set's folders back as a snapshot has
// them). Both compare snapshots with the live folders (set_compare.go,
// set_index.go) and then restore a list of files with RestoreSnapshotInline.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"pbscommon"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ---- scan progress and cancel ---------------------------------------------

var setScanCancel atomic.Bool

// CancelSetScan stops a running Undelete scan or Roll back preview.
func (a *App) CancelSetScan() { setScanCancel.Store(true) }

func (a *App) scanProgress(phase, detail string, done, total int) {
	if a.ctx == nil || a.isServiceProcess {
		return
	}
	runtime.EventsEmit(a.ctx, "setscan:progress", map[string]interface{}{
		"phase": phase, "detail": detail, "done": done, "total": total,
	})
}

// ListSetSnapshots returns a folder Backup Set's snapshots, newest first.
func (a *App) ListSetSnapshots(jobID string) ([]SetSnapshot, error) {
	job, err := a.loadSetJob(jobID)
	if err != nil {
		return nil, err
	}
	cfg, err := a.resolvePBS(job.PBSServerID)
	if err != nil {
		return nil, err
	}
	return listSetSnapshots(cfg, job)
}

// liveFolders walks every folder of a set.
func (a *App) liveFolders(job ScheduledJob, cancel func() bool) (map[string]*LiveIndex, error) {
	out := map[string]*LiveIndex{}
	folders := setFoldersOf(job)
	for i, f := range folders {
		a.scanProgress("walk", f.Dir, i, len(folders))
		li, err := walkLiveFolder(f.Dir, job.ExcludeList, cancel)
		if err != nil {
			if errors.Is(err, errCompareCancelled) {
				return nil, err
			}
			return nil, fmt.Errorf("cannot read %s: %w", f.Dir, err)
		}
		out[f.Base] = li
	}
	return out, nil
}

// archiveFor returns the raw name of the set folder's archive in a snapshot.
func archiveFor(snap SetSnapshot, base string) string {
	for _, n := range snap.Archives {
		if archiveBaseOf(n) == base {
			return n
		}
	}
	return ""
}

// ---- Undelete ----------------------------------------------------------------

// UndeleteFile is a file that is in a snapshot but not on disk.
type UndeleteFile struct {
	Folder       string `json:"folder"`  // the set's folder, as typed in the set
	Archive      string `json:"archive"` // raw archive name in SnapshotID
	Path         string `json:"path"`    // relative to Folder, forward slashes
	Size         uint64 `json:"size"`
	MTime        int64  `json:"mtime"`
	SnapshotID   string `json:"snapshotId"`   // the newest snapshot that has it
	SnapshotUnix int64  `json:"snapshotUnix"` // ditto
	// GoneByUnix is the oldest snapshot that no longer has it; 0 means it was
	// still there at the newest snapshot (deleted since).
	GoneByUnix int64  `json:"goneByUnix"`
	MovedTo    string `json:"movedTo,omitempty"` // a live file with the same name, size and time
}

// UndeleteScan is the result of ScanUndelete.
type UndeleteScan struct {
	Files             []UndeleteFile `json:"files"`
	SnapshotsChecked  int            `json:"snapshotsChecked"`
	NewestUnix        int64          `json:"newestUnix"`
	OldestUnix        int64          `json:"oldestUnix"`
	Unknown           int            `json:"unknown"`           // under folders that could not be read
	UnreadableFolders []string       `json:"unreadableFolders"` // "<folder>\<relative>"
}

// ScanUndelete lists the files of a Backup Set that are in its snapshots but
// not on disk. days = 0 checks the newest snapshot only; otherwise every
// snapshot of the last days days (and always the newest). Each file comes
// from the NEWEST snapshot that still has it.
func (a *App) ScanUndelete(jobID string, days int) (*UndeleteScan, error) {
	setScanCancel.Store(false)
	cancel := func() bool { return setScanCancel.Load() }
	job, err := a.loadSetJob(jobID)
	if err != nil {
		return nil, err
	}
	cfg, err := a.resolveRestorePBS(job.PBSServerID)
	if err != nil {
		return nil, err
	}
	a.scanProgress("snapshots", "", 0, 0)
	snaps, err := listSetSnapshots(cfg, job)
	if err != nil {
		return nil, err
	}
	if len(snaps) == 0 {
		return nil, fmt.Errorf("Backup Set %q has no snapshots on this server yet", job.Name)
	}
	if days > 0 {
		cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour).Unix()
		n := 1
		for n < len(snaps) && snaps[n].Unix >= cutoff {
			n++
		}
		snaps = snaps[:n]
	} else {
		snaps = snaps[:1]
	}

	return a.scanUndelete(cfg, job, snaps, cancel)
}

// scanUndelete checks snaps (newest first) against the live folders.
func (a *App) scanUndelete(cfg *Config, job ScheduledJob, snaps []SetSnapshot, cancel func() bool) (*UndeleteScan, error) {
	if cancel == nil {
		cancel = func() bool { return false }
	}
	live, err := a.liveFolders(job, cancel)
	if err != nil {
		return nil, err
	}
	folders := setFoldersOf(job)
	backupID := setBackupID(job)
	res := &UndeleteScan{NewestUnix: snaps[0].Unix, OldestUnix: snaps[len(snaps)-1].Unix}
	for _, f := range folders {
		for _, u := range live[f.Base].Unreadable {
			res.UnreadableFolders = append(res.UnreadableFolders, filepath.Join(f.Dir, filepath.FromSlash(u)))
		}
	}
	found := map[string]bool{}
	unknown := map[string]bool{}
	for si, snap := range snaps {
		if cancel() {
			return nil, errCompareCancelled
		}
		a.scanProgress("index", snap.ID, si, len(snaps))
		idx, err := loadSnapshotIndex(cfg, backupID, snap, cancel)
		if err != nil {
			return nil, err
		}
		res.SnapshotsChecked++
		var goneBy int64
		if si > 0 {
			goneBy = snaps[si-1].Unix
		}
		for _, f := range folders {
			archive := archiveFor(snap, f.Base)
			if archive == "" {
				continue
			}
			li := live[f.Base]
			for _, e := range idx[archive] {
				if e.IsDir || e.Path == backupMetaFileName {
					continue
				}
				key := f.Base + "|" + pathKey(e.Path)
				if found[key] || containsFile(li.Entries, e.Path) {
					continue
				}
				if underAny(e.Path, li.Unreadable) {
					unknown[key] = true
					continue
				}
				found[key] = true
				res.Files = append(res.Files, UndeleteFile{
					Folder: f.Dir, Archive: archive, Path: e.Path, Size: e.Size, MTime: e.MTime,
					SnapshotID: snap.ID, SnapshotUnix: snap.Unix, GoneByUnix: goneBy,
				})
			}
		}
	}
	res.Unknown = len(unknown)

	// Moved or renamed: same name, size and time somewhere else in the folder.
	a.scanProgress("compare", "", 0, 0)
	byFolder := map[string][]int{}
	for i, u := range res.Files {
		byFolder[u.Folder] = append(byFolder[u.Folder], i)
	}
	for _, f := range folders {
		ids := byFolder[f.Dir]
		if len(ids) == 0 {
			continue
		}
		missing := make([]IndexEntry, len(ids))
		for k, i := range ids {
			u := res.Files[i]
			missing[k] = IndexEntry{Path: u.Path, Size: u.Size, MTime: u.MTime}
		}
		hints := moveHints(missing, live[f.Base].Entries)
		for _, i := range ids {
			if to, ok := hints[res.Files[i].Path]; ok {
				res.Files[i].MovedTo = to
			}
		}
	}
	sort.Slice(res.Files, func(i, j int) bool {
		if res.Files[i].Folder != res.Files[j].Folder {
			return res.Files[i].Folder < res.Files[j].Folder
		}
		return pathKey(res.Files[i].Path) < pathKey(res.Files[j].Path)
	})
	writeDebugLog(fmt.Sprintf("Undelete scan of %s: %d snapshot(s), %d file(s) missing from disk, %d unknown",
		job.Name, res.SnapshotsChecked, len(res.Files), res.Unknown))
	return res, nil
}

// UndeleteItem is one file to bring back (from an UndeleteScan).
type UndeleteItem struct {
	Archive      string `json:"archive"`
	Path         string `json:"path"`
	SnapshotUnix int64  `json:"snapshotUnix"`
}

// StartUndelete restores the given files, each from its own snapshot. dest ""
// puts them back where they were (never replacing a file that has come back
// since); otherwise they go under dest, in a folder per set folder.
func (a *App) StartUndelete(jobID string, items []UndeleteItem, dest string, verify bool) error {
	job, err := a.loadSetJob(jobID)
	if err != nil {
		return err
	}
	if len(items) == 0 {
		return fmt.Errorf("no files selected")
	}
	cfg, err := a.resolveRestorePBS(job.PBSServerID)
	if err != nil {
		return err
	}
	roots := archiveRootsFor(job)
	bySnap := map[int64]map[string][]string{}
	for _, it := range items {
		if it.Path == "" || strings.Contains(it.Path, "..") {
			return fmt.Errorf("invalid path %q", it.Path)
		}
		if _, ok := roots[archiveBaseOf(it.Archive)]; !ok {
			return fmt.Errorf("%s is not one of this set's folders", it.Archive)
		}
		if bySnap[it.SnapshotUnix] == nil {
			bySnap[it.SnapshotUnix] = map[string][]string{}
		}
		bySnap[it.SnapshotUnix][it.Archive] = append(bySnap[it.SnapshotUnix][it.Archive], it.Path)
	}
	unixes := make([]int64, 0, len(bySnap))
	for u := range bySnap {
		unixes = append(unixes, u)
	}
	sort.Slice(unixes, func(i, j int) bool { return unixes[i] > unixes[j] })

	runs := undeleteRuns(cfg, job, bySnap, unixes, dest, verify, a.config != nil && a.config.ParallelRestore)
	histDest := dest
	if dest == "" {
		histDest = "(original location)"
	}
	a.runSetRestore(setRestoreOp{
		Label:    fmt.Sprintf("Undelete: %s (%d files)", job.Name, len(items)),
		Kind:     "undelete",
		BackupID: setBackupID(job),
		Snapshot: time.Unix(unixes[0], 0).UTC().Format("2006-01-02T15:04:05Z"),
		Dest:     histDest,
		Verify:   verify,
		Runs:     runs,
	})
	return nil
}

// undeleteRuns builds one restore per snapshot (unixes, newest first).
func undeleteRuns(cfg *Config, job ScheduledJob, bySnap map[int64]map[string][]string, unixes []int64, dest string, verify, parallel bool) []RestoreOptions {
	roots := archiveRootsFor(job)
	backupID := setBackupID(job)
	var runs []RestoreOptions
	for _, u := range unixes {
		o := restoreOptsFor(cfg, backupID, u)
		o.ArchiveIncludes = bySnap[u]
		o.RestoreACLs = true
		o.RestoreTimestamps = true
		o.VerifyAfterRestore = verify
		o.ParallelExtraction = parallel
		if dest == "" {
			o.Mode = RestoreModeOriginal
			o.ArchiveRoots = map[string]string{}
			for archive := range bySnap[u] {
				o.ArchiveRoots[archive] = roots[archiveBaseOf(archive)]
			}
			o.KeepExisting = true
		} else {
			o.Mode = RestoreModeAlternateAbs
			o.DestPath = dest
		}
		runs = append(runs, o)
	}
	return runs
}

func archiveRootsFor(job ScheduledJob) map[string]string {
	roots := map[string]string{}
	for _, f := range setFoldersOf(job) {
		roots[f.Base] = f.Dir
	}
	return roots
}

// ---- Roll back -------------------------------------------------------------

// RollbackItem is one file in a roll back preview.
type RollbackItem struct {
	Folder    string `json:"folder"`
	Path      string `json:"path"`
	Size      uint64 `json:"size"`      // the snapshot's (missing, changed) or the live one's (new)
	MTime     int64  `json:"mtime"`     // ditto
	LiveSize  uint64 `json:"liveSize"`  // changed files: the version on disk now
	LiveMTime int64  `json:"liveMtime"` // ditto
}

// RollbackPreview is what a roll back to a snapshot would do.
type RollbackPreview struct {
	SnapshotID    string         `json:"snapshotId"`
	SnapshotUnix  int64          `json:"snapshotUnix"`
	Unchanged     int            `json:"unchanged"`
	MissingCount  int            `json:"missingCount"`
	MissingBytes  uint64         `json:"missingBytes"`
	ChangedCount  int            `json:"changedCount"`
	ChangedBytes  uint64         `json:"changedBytes"`
	NewCount      int            `json:"newCount"`
	NewBytes      uint64         `json:"newBytes"`
	NewDirCount   int            `json:"newDirCount"`
	Unknown       int            `json:"unknown"`
	Missing       []RollbackItem `json:"missing"` // first previewListLimit of each
	Changed       []RollbackItem `json:"changed"`
	New           []RollbackItem `json:"new"`
	SafetyFolders []string       `json:"safetyFolders"`
}

const previewListLimit = 2000

type rollbackPlan struct {
	job      ScheduledJob
	cfg      *Config
	snap     SetSnapshot
	folders  []setFolder
	compares map[string]CompareResult // by folder base
	preview  *RollbackPreview
}

func (a *App) planRollback(jobID string, snapshotUnix int64, cancel func() bool) (*rollbackPlan, error) {
	job, err := a.loadSetJob(jobID)
	if err != nil {
		return nil, err
	}
	cfg, err := a.resolveRestorePBS(job.PBSServerID)
	if err != nil {
		return nil, err
	}
	return a.planRollbackFor(cfg, job, snapshotUnix, cancel)
}

// planRollbackFor compares one snapshot of the set with its live folders.
func (a *App) planRollbackFor(cfg *Config, job ScheduledJob, snapshotUnix int64, cancel func() bool) (*rollbackPlan, error) {
	if cancel == nil {
		cancel = func() bool { return false }
	}
	a.scanProgress("snapshots", "", 0, 0)
	snaps, err := listSetSnapshots(cfg, job)
	if err != nil {
		return nil, err
	}
	var snap *SetSnapshot
	for i := range snaps {
		if snaps[i].Unix == snapshotUnix {
			snap = &snaps[i]
		}
	}
	if snap == nil {
		return nil, fmt.Errorf("snapshot %s of %q not found", time.Unix(snapshotUnix, 0).UTC().Format(time.RFC3339), job.Name)
	}
	a.scanProgress("index", snap.ID, 0, 1)
	idx, err := loadSnapshotIndex(cfg, setBackupID(job), *snap, cancel)
	if err != nil {
		return nil, err
	}
	live, err := a.liveFolders(job, cancel)
	if err != nil {
		return nil, err
	}
	a.scanProgress("compare", "", 0, 0)
	p := &rollbackPlan{job: job, cfg: cfg, snap: *snap, folders: setFoldersOf(job), compares: map[string]CompareResult{}}
	pv := &RollbackPreview{SnapshotID: snap.ID, SnapshotUnix: snap.Unix}
	safety := map[string]bool{}
	for _, f := range p.folders {
		archive := archiveFor(*snap, f.Base)
		if archive == "" {
			// This folder was added to the set after the snapshot: nothing
			// to roll back to, so leave it alone.
			continue
		}
		c := compareIndexes(idx[archive], live[f.Base])
		p.compares[f.Base] = c
		pv.Unchanged += c.Unchanged
		pv.Unknown += c.Unknown
		pv.MissingCount += len(c.Missing)
		pv.ChangedCount += len(c.Changed)
		pv.NewCount += len(c.New)
		pv.NewDirCount += len(c.NewDirs)
		for _, m := range c.Missing {
			pv.MissingBytes += m.Size
			if len(pv.Missing) < previewListLimit {
				pv.Missing = append(pv.Missing, RollbackItem{Folder: f.Dir, Path: m.Path, Size: m.Size, MTime: m.MTime})
			}
		}
		for _, ch := range c.Changed {
			pv.ChangedBytes += ch.Snap.Size
			if len(pv.Changed) < previewListLimit {
				pv.Changed = append(pv.Changed, RollbackItem{Folder: f.Dir, Path: ch.Snap.Path, Size: ch.Snap.Size, MTime: ch.Snap.MTime, LiveSize: ch.Live.Size, LiveMTime: ch.Live.MTime})
			}
		}
		for _, n := range c.New {
			pv.NewBytes += n.Size
			if len(pv.New) < previewListLimit {
				pv.New = append(pv.New, RollbackItem{Folder: f.Dir, Path: n.Path, Size: n.Size, MTime: n.MTime})
			}
		}
		safety[rollbackSafetyRoot(f.Dir)] = true
	}
	for s := range safety {
		pv.SafetyFolders = append(pv.SafetyFolders, s)
	}
	sort.Strings(pv.SafetyFolders)
	p.preview = pv
	return p, nil
}

// PreviewRollback compares a snapshot with the live folders. Nothing is written.
func (a *App) PreviewRollback(jobID string, snapshotUnix int64) (*RollbackPreview, error) {
	setScanCancel.Store(false)
	p, err := a.planRollback(jobID, snapshotUnix, func() bool { return setScanCancel.Load() })
	if err != nil {
		return nil, err
	}
	return p.preview, nil
}

// Roll back policies.
const (
	rollbackReplace = "replace" // restore missing files and replace changed ones; keep files added since
	rollbackMissing = "missing" // restore missing files only
	rollbackExact   = "exact"   // replace and also remove files added since
)

// rollbackManifest is written into the safety folder (rollback.json) so a
// roll back can be undone, also after the app has been restarted.
type rollbackManifest struct {
	JobID       string         `json:"jobId"`
	SetName     string         `json:"setName"`
	SnapshotID  string         `json:"snapshotId"`
	Policy      string         `json:"policy"`
	Created     time.Time      `json:"created"`
	Status      string         `json:"status"` // "moving", "restoring", "done", "failed", "undone"
	Moved       []rollbackMove `json:"moved"`
	MoveFailed  []string       `json:"moveFailed,omitempty"`
	Restored    []string       `json:"restored"`
	CreatedDirs []string       `json:"createdDirs,omitempty"`
	RemovedDirs []string       `json:"removedDirs,omitempty"`
	SafetyRoots []string       `json:"safetyRoots"`
	Message     string         `json:"message,omitempty"`
}

type rollbackMove struct {
	From string `json:"from"`
	To   string `json:"to"`
	Kind string `json:"kind"` // "replaced" or "removed"
}

// rollbackSafetyRoot is the safety folder for a set folder: at the root of its
// volume, so moving a file there is a rename. A variable so tests can put it
// in a temporary folder.
var rollbackSafetyRoot = func(dir string) string {
	return filepath.Join(rollbackVolumeRoot(dir), pbscommon.RollbackSafetyFolder)
}

var unsafeNameChars = regexp.MustCompile(`[^A-Za-z0-9._ -]+`)

func safetyName(setName string) string {
	n := strings.TrimSpace(unsafeNameChars.ReplaceAllString(setName, "_"))
	if n == "" || n == "." || n == ".." {
		n = "set"
	}
	if len(n) > 60 {
		n = n[:60]
	}
	return n
}

// StartRollback puts the set's folders back as snapshotUnix has them, with
// policy replace, missing or exact. Every file it replaces or removes is
// first moved into the safety folder on the same volume, so UndoRollback can
// put everything back. backupFirst runs the set once before anything changes.
// For exact, confirmRemove must equal the number of files to be removed (the
// number the user confirmed in the preview); if the folders changed since,
// nothing is done.
func (a *App) StartRollback(jobID string, snapshotUnix int64, policy string, backupFirst bool, confirmRemove int, verify bool) error {
	switch policy {
	case rollbackReplace, rollbackMissing, rollbackExact:
	default:
		return fmt.Errorf("unknown roll back policy %q", policy)
	}
	job, err := a.loadSetJob(jobID)
	if err != nil {
		return err
	}
	runningJobsMutex.Lock()
	busy := runningJobs[job.ID]
	runningJobsMutex.Unlock()
	if busy {
		return fmt.Errorf("Backup Set %q is running a backup; roll back when it has finished", job.Name)
	}
	if _, loaded := rollbackHolds.LoadOrStore(job.ID, true); loaded {
		return fmt.Errorf("a roll back of %q is already running", job.Name)
	}
	if backupFirst {
		// The backup is started before the hold, or the hold would stop it.
		rollbackHolds.Delete(job.ID)
		if err := a.RunScheduledJobNow(job.ID); err != nil {
			return fmt.Errorf("could not start the backup before the roll back: %w", err)
		}
	}
	started := time.Now()
	snapID := time.Unix(snapshotUnix, 0).UTC().Format("2006-01-02T15:04:05Z")
	label := fmt.Sprintf("Roll back: %s to %s", job.Name, time.Unix(snapshotUnix, 0).Format("2006-01-02 15:04"))

	var manifest *rollbackManifest
	var manifestPath string
	a.runSetRestore(setRestoreOp{
		Label:    label,
		Kind:     "rollback",
		BackupID: setBackupID(job),
		Snapshot: snapID,
		Dest:     "(original location)",
		Verify:   verify,
		BeforeSlot: func(ctx context.Context, emit func(float64, string), stage func(string, string)) error {
			if !backupFirst {
				return nil
			}
			// Outside the operation slot: the backup needs it.
			stage("backupfirst", "")
			emit(0.01, fmt.Sprintf("Backing up %s before the roll back...", job.Name))
			if err := a.waitForSetRun(ctx, job, started); err != nil {
				return err
			}
			rollbackHolds.Store(job.ID, true)
			return nil
		},
		Prepare: func(ctx context.Context, emit func(float64, string), stage func(string, string)) ([]RestoreOptions, error) {
			cancel := func() bool { return ctx.Err() != nil }
			stage("comparing", "")
			emit(0.02, "Comparing the snapshot with the folders...")
			plan, err := a.planRollback(job.ID, snapshotUnix, cancel)
			if err != nil {
				return nil, err
			}
			if policy == rollbackExact && plan.preview.NewCount != confirmRemove {
				return nil, fmt.Errorf("the folders have changed since the preview: %d files would now be removed, not %d. Nothing was changed; preview again", plan.preview.NewCount, confirmRemove)
			}
			stage("safety", "")
			m, mpath, runs, err := a.applyRollbackMoves(ctx, plan, policy, verify, emit)
			manifest, manifestPath = m, mpath
			return runs, err
		},
		Finish: func(success bool, extracted []pbscommon.PXARExtractedFile, runErr error) string {
			rollbackHolds.Delete(job.ID)
			if manifest == nil {
				return ""
			}
			for _, f := range extracted {
				if !f.Skipped && !f.IsDir && f.Path != "" {
					manifest.Restored = append(manifest.Restored, f.Path)
				}
			}
			if success {
				manifest.Status = "done"
			} else {
				manifest.Status = "failed"
				if runErr != nil {
					manifest.Message = runErr.Error()
				}
			}
			if err := writeRollbackManifest(manifestPath, manifest); err != nil {
				writeDebugLog(fmt.Sprintf("Roll back: could not update %s: %v", manifestPath, err))
			}
			moved, removed := 0, 0
			for _, mv := range manifest.Moved {
				if mv.Kind == "removed" {
					removed++
				} else {
					moved++
				}
			}
			summary := fmt.Sprintf("Restored %d files, replaced %d, removed %d", len(manifest.Restored)-moved, moved, removed)
			if len(manifest.MoveFailed) > 0 {
				summary += fmt.Sprintf("; %d files in use were left as they are", len(manifest.MoveFailed))
			}
			return summary + ". Previous versions kept in " + filepath.Dir(manifestPath)
		},
	})
	return nil
}

// waitForSetRun waits for the run of job started at or after since to finish,
// reading the job history (the run may be in this process or the service).
func (a *App) waitForSetRun(ctx context.Context, job ScheduledJob, since time.Time) error {
	tick := time.NewTicker(3 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("roll back cancelled while waiting for the backup")
		case <-tick.C:
		}
		hist, err := a.GetJobHistory()
		if err != nil {
			continue
		}
		for _, h := range hist {
			if h.Name != job.Name || h.Kind == "restore" || h.Status == "running" {
				continue
			}
			ts, err := time.Parse(time.RFC3339, h.Timestamp)
			if err != nil || ts.Before(since.Add(-2*time.Second)) {
				continue
			}
			if h.Status == "success" || h.Status == "warning" {
				return nil
			}
			return fmt.Errorf("the backup before the roll back did not succeed (%s); nothing was changed", h.Status)
		}
	}
}

// applyRollbackMoves moves every file the roll back will replace or remove
// into the safety folder, records it in rollback.json, removes folders an
// exact roll back empties, and returns the restores still to run.
func (a *App) applyRollbackMoves(ctx context.Context, p *rollbackPlan, policy string, verify bool, emit func(float64, string)) (*rollbackManifest, string, []RestoreOptions, error) {
	stamp := time.Now().Format("20060102-150405")
	m := &rollbackManifest{
		JobID: p.job.ID, SetName: p.job.Name, SnapshotID: p.snap.ID, Policy: policy,
		Created: time.Now(), Status: "moving",
	}
	type folderWork struct {
		f       setFolder
		archive string
		safety  string // this folder's own safety subfolder
	}
	var work []folderWork
	var manifestPath string
	for _, f := range p.folders {
		archive := archiveFor(p.snap, f.Base)
		if archive == "" {
			continue
		}
		root := filepath.Join(rollbackSafetyRoot(f.Dir), safetyName(p.job.Name), stamp)
		if err := os.MkdirAll(root, 0o700); err != nil {
			return nil, "", nil, fmt.Errorf("cannot create the safety folder %s: %w. Nothing was changed", root, err)
		}
		if manifestPath == "" {
			manifestPath = filepath.Join(root, "rollback.json")
		}
		found := false
		for _, r := range m.SafetyRoots {
			if r == root {
				found = true
			}
		}
		if !found {
			m.SafetyRoots = append(m.SafetyRoots, root)
		}
		work = append(work, folderWork{f: f, archive: archive, safety: filepath.Join(root, f.Base)})
	}
	if manifestPath == "" {
		return nil, "", nil, fmt.Errorf("none of the set's folders is in this snapshot")
	}
	if err := writeRollbackManifest(manifestPath, m); err != nil {
		return nil, "", nil, fmt.Errorf("cannot write %s: %w. Nothing was changed", manifestPath, err)
	}

	total := 0
	for _, w := range work {
		c := p.compares[w.f.Base]
		if policy != rollbackMissing {
			total += len(c.Changed)
		}
		if policy == rollbackExact {
			total += len(c.New)
		}
	}
	done := 0
	includes := map[string][]string{}
	roots := map[string]string{}
	move := func(w folderWork, rel, kind string) bool {
		from := filepath.Join(w.f.Dir, filepath.FromSlash(rel))
		to := filepath.Join(w.safety, filepath.FromSlash(rel))
		done++
		if done%200 == 0 || done == total {
			emit(0.03+0.07*float64(done)/float64(max(total, 1)), fmt.Sprintf("Moving files aside: %d of %d", done, total))
		}
		if err := os.MkdirAll(filepath.Dir(to), 0o700); err == nil {
			if err = os.Rename(from, to); err == nil {
				m.Moved = append(m.Moved, rollbackMove{From: from, To: to, Kind: kind})
				return true
			} else {
				m.MoveFailed = append(m.MoveFailed, fmt.Sprintf("%s: %v", from, err))
			}
		} else {
			m.MoveFailed = append(m.MoveFailed, fmt.Sprintf("%s: %v", from, err))
		}
		return false
	}
	for _, w := range work {
		c := p.compares[w.f.Base]
		var restore []string
		for _, ms := range c.Missing {
			restore = append(restore, ms.Path)
			m.CreatedDirs = append(m.CreatedDirs, missingParents(w.f.Dir, ms.Path)...)
		}
		if policy != rollbackMissing {
			for _, ch := range c.Changed {
				if ctx.Err() != nil {
					break
				}
				if move(w, ch.Snap.Path, "replaced") {
					restore = append(restore, ch.Snap.Path)
				}
			}
		}
		if policy == rollbackExact {
			for _, n := range c.New {
				if ctx.Err() != nil {
					break
				}
				move(w, n.Path, "removed")
			}
			// Folders the snapshot does not have, deepest first, if now empty.
			dirs := append([]IndexEntry(nil), c.NewDirs...)
			sort.Slice(dirs, func(i, j int) bool { return len(dirs[i].Path) > len(dirs[j].Path) })
			for _, d := range dirs {
				full := filepath.Join(w.f.Dir, filepath.FromSlash(d.Path))
				if os.Remove(full) == nil {
					m.RemovedDirs = append(m.RemovedDirs, full)
				}
			}
		}
		if len(restore) > 0 {
			includes[w.archive] = restore
			roots[w.archive] = w.f.Dir
		}
		if ctx.Err() != nil {
			break
		}
	}
	m.CreatedDirs = dedupeStrings(m.CreatedDirs)
	m.Status = "restoring"
	if err := writeRollbackManifest(manifestPath, m); err != nil {
		writeDebugLog(fmt.Sprintf("Roll back: could not update %s: %v", manifestPath, err))
	}
	if ctx.Err() != nil {
		return m, manifestPath, nil, fmt.Errorf("roll back cancelled while moving files aside; use Undo to put back the %d files already moved", len(m.Moved))
	}
	if len(includes) == 0 {
		return m, manifestPath, nil, nil
	}
	o := restoreOptsFor(p.cfg, setBackupID(p.job), p.snap.Unix)
	o.Mode = RestoreModeOriginal
	o.ArchiveRoots = roots
	o.ArchiveIncludes = includes
	o.KeepExisting = true // everything being replaced has been moved aside
	o.RestoreACLs = true
	o.RestoreTimestamps = true
	o.VerifyAfterRestore = verify
	o.ParallelExtraction = a.config != nil && a.config.ParallelRestore
	return m, manifestPath, []RestoreOptions{o}, nil
}

// missingParents returns the folders above rel (inside dir) that do not
// exist yet, so an undo can remove the ones the restore created.
func missingParents(dir, rel string) []string {
	var out []string
	cur := filepath.Dir(filepath.Join(dir, filepath.FromSlash(rel)))
	stop := filepath.Clean(dir)
	for cur != stop && len(cur) > len(stop) {
		if _, err := os.Stat(cur); err == nil {
			break
		}
		out = append(out, cur)
		cur = filepath.Dir(cur)
	}
	return out
}

func dedupeStrings(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func writeRollbackManifest(path string, m *rollbackManifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return atomicWriteFile(path, data, 0o600)
}

// RollbackRecord is a roll back that left a safety folder.
type RollbackRecord struct {
	Path       string `json:"path"` // its rollback.json
	SnapshotID string `json:"snapshotId"`
	Policy     string `json:"policy"`
	Created    string `json:"created"`
	Status     string `json:"status"`
	Moved      int    `json:"moved"`
	Removed    int    `json:"removed"`
	Restored   int    `json:"restored"`
	Bytes      int64  `json:"bytes"` // size of what the safety folder holds
}

// ListRollbacks lists the set's earlier roll backs that can still be undone
// or cleaned up, newest first.
func (a *App) ListRollbacks(jobID string) ([]RollbackRecord, error) {
	job, err := a.loadSetJob(jobID)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var out []RollbackRecord
	for _, f := range setFoldersOf(job) {
		base := filepath.Join(rollbackSafetyRoot(f.Dir), safetyName(job.Name))
		if seen[base] {
			continue
		}
		seen[base] = true
		items, err := os.ReadDir(base)
		if err != nil {
			continue
		}
		for _, it := range items {
			mp := filepath.Join(base, it.Name(), "rollback.json")
			m, err := readRollbackManifest(mp)
			if err != nil || m.JobID != job.ID {
				continue
			}
			r := RollbackRecord{Path: mp, SnapshotID: m.SnapshotID, Policy: m.Policy, Created: m.Created.Format(time.RFC3339), Status: m.Status}
			for _, mv := range m.Moved {
				if mv.Kind == "removed" {
					r.Removed++
				} else {
					r.Moved++
				}
				if fi, err := os.Stat(mv.To); err == nil {
					r.Bytes += fi.Size()
				}
			}
			r.Restored = len(m.Restored)
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Created > out[j].Created })
	return out, nil
}

func readRollbackManifest(path string) (*rollbackManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m rollbackManifest
	if err := json.Unmarshal(stripUTF8BOM(data), &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// checkManifestPath makes sure path is a rollback.json inside a safety folder.
func checkManifestPath(path string) error {
	clean := filepath.Clean(path)
	if filepath.Base(clean) != "rollback.json" || !strings.Contains(filepath.ToSlash(clean), "/"+pbscommon.RollbackSafetyFolder+"/") {
		return fmt.Errorf("not a roll back record: %s", path)
	}
	return nil
}

// UndoRollback puts the folders back as they were before a roll back: the
// files it restored are deleted, the files it moved aside go back, folders it
// removed are made again and folders it created are removed if empty.
func (a *App) UndoRollback(manifestPath string) (string, error) {
	if err := checkManifestPath(manifestPath); err != nil {
		return "", err
	}
	m, err := readRollbackManifest(manifestPath)
	if err != nil {
		return "", err
	}
	if m.Status == "undone" {
		return "", fmt.Errorf("this roll back has already been undone")
	}
	if rollbackHeld(m.JobID) {
		return "", fmt.Errorf("a roll back of this set is running")
	}
	release := acquireOperationSlot("Undo roll back: "+m.SetName, nil)
	defer release()
	movedFrom := map[string]bool{}
	for _, mv := range m.Moved {
		movedFrom[mv.From] = true
	}
	var problems []string
	deleted := 0
	for _, p := range m.Restored {
		if movedFrom[p] {
			continue // removed below, just before its old version goes back
		}
		if err := removeFileForce(p); err != nil && !os.IsNotExist(err) {
			problems = append(problems, fmt.Sprintf("%s: %v", p, err))
		} else {
			deleted++
		}
	}
	back := 0
	for _, mv := range m.Moved {
		if _, err := os.Stat(mv.To); err != nil {
			problems = append(problems, fmt.Sprintf("%s: the kept copy is gone", mv.From))
			continue
		}
		if err := os.MkdirAll(filepath.Dir(mv.From), 0o755); err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", mv.From, err))
			continue
		}
		if err := removeFileForce(mv.From); err != nil && !os.IsNotExist(err) {
			problems = append(problems, fmt.Sprintf("%s: %v", mv.From, err))
			continue
		}
		if err := os.Rename(mv.To, mv.From); err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", mv.From, err))
			continue
		}
		back++
	}
	for _, d := range m.RemovedDirs {
		_ = os.MkdirAll(d, 0o755)
	}
	dirs := append([]string(nil), m.CreatedDirs...)
	sort.Slice(dirs, func(i, j int) bool { return len(dirs[i]) > len(dirs[j]) })
	for _, d := range dirs {
		_ = os.Remove(d) // only succeeds when empty
	}
	if len(problems) == 0 {
		m.Status = "undone"
	} else {
		m.Message = fmt.Sprintf("Undo left %d problem(s): %s", len(problems), strings.Join(problems[:min(len(problems), 20)], "; "))
	}
	if err := writeRollbackManifest(manifestPath, m); err != nil {
		writeDebugLog(fmt.Sprintf("Undo roll back: could not update %s: %v", manifestPath, err))
	}
	msg := fmt.Sprintf("Undo roll back of %s: %d files put back, %d restored files removed", m.SetName, back, deleted)
	level := "info"
	if len(problems) > 0 {
		msg += fmt.Sprintf(", %d problem(s), see the debug log", len(problems))
		level = "error"
		for _, p := range problems {
			writeDebugLog("Undo roll back: " + p)
		}
	}
	LogMessage("Restore", level, msg, "", "", nil)
	if len(problems) == 0 {
		// Nothing left in it but the record: tidy it away.
		for _, r := range m.SafetyRoots {
			removeEmptyTree(r, manifestPath)
		}
	}
	return msg, nil
}

// removeFileForce deletes a file, clearing a read-only attribute that would
// stop Windows from deleting it.
func removeFileForce(p string) error {
	err := os.Remove(p)
	if err == nil || os.IsNotExist(err) {
		return err
	}
	if fi, serr := os.Lstat(p); serr == nil && fi.Mode().IsRegular() && fi.Mode().Perm()&0o200 == 0 {
		if os.Chmod(p, fi.Mode().Perm()|0o200) == nil {
			return os.Remove(p)
		}
	}
	return err
}

// removeEmptyTree removes root if it holds nothing but empty folders and keep.
func removeEmptyTree(root, keep string) {
	hasFiles := false
	_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && p != keep {
			hasFiles = true
			return filepath.SkipAll
		}
		return nil
	})
	if !hasFiles {
		_ = os.RemoveAll(root)
	}
}

// DeleteRollbackCopies deletes a roll back's safety folder (the old versions
// it kept). The roll back can no longer be undone afterwards.
func (a *App) DeleteRollbackCopies(manifestPath string) error {
	if err := checkManifestPath(manifestPath); err != nil {
		return err
	}
	m, err := readRollbackManifest(manifestPath)
	if err != nil {
		return err
	}
	for _, r := range m.SafetyRoots {
		if err := checkManifestPath(filepath.Join(r, "rollback.json")); err != nil {
			return err
		}
		// Kept copies can be read-only, which stops Windows deleting them.
		_ = filepath.WalkDir(r, func(p string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				if fi, ierr := d.Info(); ierr == nil && fi.Mode().Perm()&0o200 == 0 {
					_ = os.Chmod(p, fi.Mode().Perm()|0o200)
				}
			}
			return nil
		})
		if err := os.RemoveAll(r); err != nil {
			return err
		}
	}
	LogMessage("Restore", "info", fmt.Sprintf("Deleted the copies kept by the roll back of %s to %s", m.SetName, m.SnapshotID), "", "", nil)
	return nil
}

// ---- one operation, several restores ------------------------------------------

type setRestoreOp struct {
	Label    string
	Kind     string // "undelete" or "rollback"
	BackupID string
	Snapshot string
	Dest     string
	Verify   bool
	Runs     []RestoreOptions
	// BeforeSlot runs before the operation slot is taken (Roll back waits
	// there for the backup it started, which needs the slot itself). Stop
	// cancels its ctx.
	BeforeSlot func(ctx context.Context, emit func(float64, string), stage func(string, string)) error
	// Prepare runs inside the operation slot before the restores and may
	// return the restores to run (Roll back computes them there).
	Prepare func(ctx context.Context, emit func(float64, string), stage func(string, string)) ([]RestoreOptions, error)
	// Finish runs after the restores; its text is added to the result.
	Finish func(success bool, extracted []pbscommon.PXARExtractedFile, err error) string
}

// runSetRestore runs an Undelete or Roll back as one restore on the restore
// card: same events, Stop, Reports entry and notification as RestoreSnapshot.
func (a *App) runSetRestore(op setRestoreOp) {
	emit := func(percent float64, message string) {
		markRestoreProgress()
		if a.ctx == nil {
			return
		}
		runtime.EventsEmit(a.ctx, "restore:progress", map[string]interface{}{
			"percent": percent, "message": message, "name": op.Label,
		})
	}
	stage := func(name, detail string) {
		markRestoreProgress()
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "restore:stage", map[string]interface{}{"stage": name, "detail": detail})
		}
	}
	a.UpdateTrayTooltip(op.Label)
	go func() {
		start := time.Now()
		var extracted []pbscommon.PXARExtractedFile
		var files int
		var bytes int64
		var verified int
		var verifyFails []string
		var err error
		if op.BeforeSlot != nil {
			pctx, pcancel := context.WithCancel(context.Background())
			setPendingSetOpCancel(pcancel)
			err = op.BeforeSlot(pctx, emit, stage)
			setPendingSetOpCancel(nil)
			pcancel()
		}
		var release func()
		if err == nil {
			release = acquireOperationSlot(op.Label, func(heldBy string) {
				emit(0.01, fmt.Sprintf("Queued: %s — waiting for %s to finish...", op.Label, heldBy))
			})
		} else {
			release = func() {}
		}
		defer release()
		markRestoreStarted()
		defer markRestoreDone()
		ctx, _ := newRestoreContext()
		defer doneRestoreContext()
		func() {
			if err != nil {
				return
			}
			defer func() {
				if r := recover(); r != nil {
					err = fmt.Errorf("%s panic: %v", op.Kind, r)
					writeDebugLog(fmt.Sprintf("CRITICAL: %s panic: %v\n%s", op.Kind, r, debug.Stack()))
				}
			}()
			runs := op.Runs
			if op.Prepare != nil {
				runs, err = op.Prepare(ctx, emit, stage)
				if err != nil {
					return
				}
			}
			var fileMu sync.Mutex
			var lastFile time.Time
			for i, o := range runs {
				if ctx.Err() != nil {
					err = fmt.Errorf("%s cancelled", op.Kind)
					return
				}
				lo, hi := 0.10, 1.0
				if op.Prepare == nil {
					lo = 0
				}
				span := (hi - lo) / float64(len(runs))
				base := lo + span*float64(i)
				detail := ""
				if len(runs) > 1 {
					detail = fmt.Sprintf("snapshot %d/%d", i+1, len(runs))
				}
				o.Ctx = ctx
				o.OnProgress = func(p float64, msg string) {
					if detail != "" {
						msg = detail + ": " + msg
					}
					emit(base+span*p, msg)
				}
				o.OnStage = func(s, d string) {
					if d == "" {
						d = detail
					}
					stage(s, d)
				}
				o.OnFile = func(path string) {
					fileMu.Lock()
					if time.Since(lastFile) < 200*time.Millisecond {
						fileMu.Unlock()
						return
					}
					lastFile = time.Now()
					fileMu.Unlock()
					if a.ctx != nil {
						runtime.EventsEmit(a.ctx, "restore:file", map[string]interface{}{"path": path})
					}
				}
				o.OnStats = func(s *RestoreProgressStats) {
					markRestoreProgress()
					if a.ctx != nil {
						runtime.EventsEmit(a.ctx, "restore:stats", map[string]interface{}{
							"bytesDone": s.BytesDone, "bytesTotal": s.BytesTotal, "currentArchive": s.CurrentArchive,
						})
					}
				}
				o.OnSummary = func(f, d int, b int64) { files += f; bytes += b }
				o.OnVerifySummary = func(v int, fails []string) { verified += v; verifyFails = append(verifyFails, fails...) }
				o.OnExtracted = func(ex []pbscommon.PXARExtractedFile) { extracted = append(extracted, ex...) }
				if rerr := RestoreSnapshotInline(o); rerr != nil {
					err = rerr
					return
				}
			}
		}()
		success := err == nil
		extra := ""
		if op.Finish != nil {
			extra = op.Finish(success, extracted, err)
		}
		what := map[string]string{"undelete": "Undelete", "rollback": "Roll back"}[op.Kind]
		msg := fmt.Sprintf("%s completed: %d files (%s)", what, files, formatByteSize(uint64(bytes)))
		if op.Kind == "rollback" && extra != "" {
			msg = what + " completed. " + extra
		}
		if err != nil {
			msg = fmt.Sprintf("%s failed: %s", what, err.Error())
			if extra != "" {
				msg += ". " + extra
			}
		}
		level := "info"
		status := "success"
		if !success {
			level, status = "error", "failed"
			if strings.Contains(strings.ToLower(err.Error()), "cancel") {
				status = "cancelled"
			}
		}
		LogMessage("Restore", level, msg, "", "", nil)
		if herr := a.appendJobHistory(JobHistory{
			ID:                  fmt.Sprintf("%s-%d", op.Kind, start.UnixNano()),
			Name:                op.Label,
			Timestamp:           time.Now().Format(time.RFC3339),
			Status:              status,
			Message:             msg,
			BackupID:            op.BackupID,
			Kind:                "restore",
			RestoreKind:         op.Kind,
			RestoreSnapshot:     op.Snapshot,
			RestoreDest:         op.Dest,
			RestoreFiles:        files,
			RestoreBytes:        bytes,
			RestoreVerified:     verified,
			RestoreVerifyFailed: len(verifyFails),
			RestoreVerifyRan:    op.Verify && (success || len(verifyFails) > 0),
			DurationSec:         int(time.Since(start).Seconds()),
		}); herr != nil {
			writeDebugLog(fmt.Sprintf("%s: failed to record Reports history: %v", what, herr))
		}
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "restore:complete", map[string]interface{}{
				"success":       success,
				"message":       msg,
				"verify_ran":    op.Verify,
				"verified":      verified,
				"verify_failed": len(verifyFails),
				"kind":          op.Kind,
			})
		}
		if success {
			a.ShowToastNotification(what+" complete", op.Label, false)
		} else {
			a.ShowToastNotification(what+" failed", msg, true)
		}
		a.RefreshIdleTooltip()
	}()
}
