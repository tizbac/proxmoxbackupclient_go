package main

// A folder Backup Set's history on PBS and the file list of each of its
// snapshots, for Undelete and Roll back.

import (
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"pbscommon"
)

// setFolder is one folder of a Backup Set and the archive base name its
// backups use (archiveBaseName, computed exactly as a backup run does).
type setFolder struct {
	Dir  string
	Base string
}

func setFoldersOf(job ScheduledJob) []setFolder {
	used := map[string]int{}
	var out []setFolder
	for _, d := range job.BackupDirs {
		d = strings.TrimSpace(d)
		if d == "" {
			continue
		}
		out = append(out, setFolder{Dir: d, Base: archiveBaseName(d, used)})
	}
	return out
}

// SetSnapshot is one snapshot of a Backup Set, newest first in lists.
type SetSnapshot struct {
	ID        string   `json:"id"` // RFC 3339 UTC, the form RestoreSnapshot takes
	Unix      int64    `json:"unix"`
	Size      int64    `json:"size"`
	Protected bool     `json:"protected"`
	Archives  []string `json:"archives"` // raw names of this set's archives in it
}

func (a *App) loadSetJob(jobID string) (ScheduledJob, error) {
	jobs, err := a.GetScheduledJobs()
	if err != nil {
		return ScheduledJob{}, fmt.Errorf("failed to load Backup Sets: %w", err)
	}
	for _, j := range jobs {
		if j.ID == jobID {
			if j.BackupType == "machine" {
				return ScheduledJob{}, fmt.Errorf("%q is a machine Backup Set: Undelete and Roll back work on folder sets", j.Name)
			}
			if len(setFoldersOf(j)) == 0 {
				return ScheduledJob{}, fmt.Errorf("Backup Set %q has no folders", j.Name)
			}
			return j, nil
		}
	}
	return ScheduledJob{}, fmt.Errorf("Backup Set %s not found", jobID)
}

// setBackupID is the backup ID a set's runs write to (the hostname when the
// set leaves it empty, as a backup run does).
func setBackupID(job ScheduledJob) string {
	if job.BackupID != "" {
		return job.BackupID
	}
	h, _ := os.Hostname()
	return h
}

// listSetSnapshots returns the snapshots of the set's backup group that hold
// at least one of the set's own archives, newest first. Matching archives as
// well as the backup ID keeps two sets that share an ID apart.
func listSetSnapshots(cfg *Config, job ScheduledJob) ([]SetSnapshot, error) {
	client := &pbscommon.PBSClient{
		BaseURL:          cfg.BaseURL,
		CertFingerPrint:  cfg.CertFingerprint,
		AuthID:           cfg.AuthID,
		Secret:           cfg.Secret,
		Ticket:           cfg.Ticket,
		CSRFToken:        cfg.CSRFToken,
		Datastore:        cfg.Datastore,
		Namespace:        cfg.Namespace,
		Insecure:         cfg.CertFingerprint != "",
		CompressionLevel: pbscommon.CompressionFastest,
	}
	manifests, err := client.ListSnapshots()
	if err != nil {
		return nil, fmt.Errorf("failed to list snapshots: %w", err)
	}
	backupID := setBackupID(job)
	bases := map[string]bool{}
	for _, f := range setFoldersOf(job) {
		bases[f.Base] = true
	}
	var out []SetSnapshot
	for _, m := range manifests {
		if m.BackupType != "host" || m.BackupID != backupID {
			continue
		}
		var archives []string
		for _, f := range m.Files {
			if isDataArchive(f.Filename) && bases[archiveBaseOf(f.Filename)] {
				archives = append(archives, f.Filename)
			}
		}
		if len(archives) == 0 {
			continue
		}
		t := time.Unix(m.BackupTime, 0).UTC()
		out = append(out, SetSnapshot{
			ID:        t.Format("2006-01-02T15:04:05Z"),
			Unix:      m.BackupTime,
			Size:      m.Size,
			Protected: m.Protected,
			Archives:  archives,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Unix > out[j].Unix })
	return out, nil
}

// snapshotIndex is the file list of one snapshot: raw archive name -> entries
// sorted by pathKey.
type snapshotIndex map[string][]IndexEntry

func restoreOptsFor(cfg *Config, backupID string, unix int64) RestoreOptions {
	return RestoreOptions{
		BaseURL:         cfg.BaseURL,
		AuthID:          cfg.AuthID,
		Secret:          cfg.Secret,
		Ticket:          cfg.Ticket,
		CSRFToken:       cfg.CSRFToken,
		Datastore:       cfg.Datastore,
		Namespace:       cfg.Namespace,
		CertFingerprint: cfg.CertFingerprint,
		BackupID:        backupID,
		SnapshotTime:    time.Unix(unix, 0),
		Crypt:           cfg.Crypt,
	}
}

// loadSnapshotIndex returns the file list of the given archives of a snapshot,
// from the local cache when it has it (a snapshot never changes), else from
// the snapshot's catalog (a few MB even for hundreds of thousands of files),
// else by walking each archive's metadata.
func loadSnapshotIndex(cfg *Config, backupID string, snap SetSnapshot, cancel func() bool) (snapshotIndex, error) {
	cachePath := snapshotIndexCachePath(cfg, backupID, snap.Unix)
	if idx, err := readIndexCache(cachePath); err == nil && hasArchives(idx, snap.Archives) {
		return idx, nil
	}
	opts := restoreOptsFor(cfg, backupID, snap.Unix)
	idx, ok := indexFromCatalog(opts, cancel)
	if !ok || !hasArchives(idx, snap.Archives) {
		if cancel != nil && cancel() {
			return nil, errCompareCancelled
		}
		idx = snapshotIndex{}
		for _, name := range snap.Archives {
			var entries []IndexEntry
			err := withSnapshotReader(opts, name, "Index", nil, nil, cancel, func(reader *pbscommon.PXARReader) error {
				es, lerr := reader.ListEntries()
				if lerr != nil {
					return lerr
				}
				entries = make([]IndexEntry, 0, len(es))
				for _, e := range es {
					if e.Path == "" {
						continue
					}
					entries = append(entries, IndexEntry{Path: e.Path, IsDir: e.IsDir, Size: e.Size, MTime: e.ModTime})
				}
				return nil
			})
			if err != nil {
				return nil, fmt.Errorf("failed to read the file list of %s: %w", name, err)
			}
			sortIndex(entries)
			idx[name] = entries
		}
	}
	if err := writeIndexCache(cachePath, idx); err != nil {
		writeDebugLog(fmt.Sprintf("Index cache: could not save %s: %v", cachePath, err))
	}
	return idx, nil
}

func hasArchives(idx snapshotIndex, names []string) bool {
	for _, n := range names {
		if _, ok := idx[n]; !ok {
			return false
		}
	}
	return true
}

// indexFromCatalog reads catalog.pcat1.didx and splits it by archive.
func indexFromCatalog(opts RestoreOptions, cancel func() bool) (snapshotIndex, bool) {
	client := &pbscommon.PBSClient{
		BaseURL:          opts.BaseURL,
		CertFingerPrint:  opts.CertFingerprint,
		AuthID:           opts.AuthID,
		Secret:           opts.Secret,
		Ticket:           opts.Ticket,
		CSRFToken:        opts.CSRFToken,
		Datastore:        opts.Datastore,
		Namespace:        opts.Namespace,
		Insecure:         opts.CertFingerprint != "",
		CompressionLevel: pbscommon.CompressionFastest,
		Crypt:            opts.Crypt,
		Manifest: pbscommon.BackupManifest{
			BackupID:   opts.BackupID,
			BackupTime: opts.SnapshotTime.Unix(),
		},
	}
	client.Connect(true, "host")
	defer client.Close()
	ra, size, err := client.NewDIDXReaderAt("catalog.pcat1.didx", 64, nil)
	if err != nil {
		return nil, false
	}
	ra.SetCancelCheck(cancel)
	buf := make([]byte, size)
	if _, rerr := ra.ReadAt(buf, 0); rerr != nil && rerr != io.EOF {
		return nil, false
	}
	cat, perr := pbscommon.ParseCatalog(buf)
	if perr != nil {
		return nil, false
	}
	idx := snapshotIndex{}
	for _, e := range cat {
		archive, rest, found := strings.Cut(e.Path, "/")
		if _, ok := idx[archive]; !ok {
			idx[archive] = nil
		}
		if !found || rest == "" {
			continue
		}
		idx[archive] = append(idx[archive], IndexEntry{Path: rest, IsDir: e.IsDir, Size: e.Size, MTime: e.ModTime})
	}
	for k := range idx {
		sortIndex(idx[k])
	}
	writeBackupLog(fmt.Sprintf("Index: %s@%d from the catalog (%d bytes, %d archive(s))",
		opts.BackupID, opts.SnapshotTime.Unix(), size, len(idx)))
	return idx, true
}

// ---- local cache --------------------------------------------------------

const indexCacheLimit = 1 << 30 // 1 GiB; oldest files are dropped first

func indexCacheDir() string {
	p, err := getConfigPath()
	if err != nil {
		return filepath.Join(os.TempDir(), "pbs-index-cache")
	}
	return filepath.Join(filepath.Dir(p), "index-cache")
}

func snapshotIndexCachePath(cfg *Config, backupID string, unix int64) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%s|%s|%d", cfg.BaseURL, cfg.Datastore, cfg.Namespace, backupID, unix)))
	return filepath.Join(indexCacheDir(), hex.EncodeToString(sum[:12])+".idx.gz")
}

func readIndexCache(path string) (snapshotIndex, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	var idx snapshotIndex
	if err := gob.NewDecoder(zr).Decode(&idx); err != nil {
		return nil, err
	}
	now := time.Now()
	_ = os.Chtimes(path, now, now) // recently used: keep it longest
	return idx, nil
}

func writeIndexCache(path string, idx snapshotIndex) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	zw := gzip.NewWriter(f)
	if err := gob.NewEncoder(zw).Encode(idx); err != nil {
		zw.Close()
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := zw.Close(); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	trimIndexCache(filepath.Dir(path), indexCacheLimit)
	return nil
}

// trimIndexCache deletes the least recently used cache files until the
// folder is under limit bytes.
func trimIndexCache(dir string, limit int64) {
	items, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	type cf struct {
		path string
		size int64
		mod  time.Time
	}
	var files []cf
	var total int64
	for _, it := range items {
		if it.IsDir() || !strings.HasSuffix(it.Name(), ".idx.gz") {
			continue
		}
		info, err := it.Info()
		if err != nil {
			continue
		}
		files = append(files, cf{filepath.Join(dir, it.Name()), info.Size(), info.ModTime()})
		total += info.Size()
	}
	if total <= limit {
		return
	}
	sort.Slice(files, func(i, j int) bool { return files[i].mod.Before(files[j].mod) })
	for _, f := range files {
		if total <= limit {
			break
		}
		if os.Remove(f.path) == nil {
			total -= f.size
		}
	}
}

// ---- roll back state shared with the scheduler and Stop -----------------

var rollbackHolds sync.Map // job ID -> true while a roll back of it runs

func rollbackHeld(jobID string) bool {
	_, held := rollbackHolds.Load(jobID)
	return held
}

// pendingSetOpCancel is the cancel of a set operation that is waiting outside
// the operation slot (a roll back waiting for its backup), so that Stop
// (CancelRestore) reaches it.
var (
	pendingSetOpMu     sync.Mutex
	pendingSetOpCancel context.CancelFunc
)

// setPendingSetOpCancel registers the cancel of a set operation that is
// waiting outside the operation slot, so Stop (CancelRestore) reaches it.
func setPendingSetOpCancel(c context.CancelFunc) {
	pendingSetOpMu.Lock()
	pendingSetOpCancel = c
	pendingSetOpMu.Unlock()
}

func cancelPendingSetOp() bool {
	pendingSetOpMu.Lock()
	defer pendingSetOpMu.Unlock()
	if pendingSetOpCancel != nil {
		pendingSetOpCancel()
		return true
	}
	return false
}
