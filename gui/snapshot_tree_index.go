package main

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// snapshotTreeIndex is the in-memory, per-directory view of ONE snapshot's
// listing. The restore tree asks for one folder's children at a time, so the
// (potentially ~900k entry) listing never crosses the Wails IPC boundary or
// gets JSON-parsed by the UI in one go.
type snapshotTreeIndex struct {
	key      snapshotCacheKey
	count    int
	children map[string][]SnapshotEntry // parent dir path ("" = root) -> direct children
	dirBytes map[string]uint64          // dir path -> total bytes of every file below it
}

var (
	snapshotIndexMu  sync.Mutex
	snapshotIndexCur *snapshotTreeIndex
)

// SnapshotTreeOpen is returned when a snapshot is opened: the entry count (for
// status text) and the top level, so the first paint needs one small payload.
type SnapshotTreeOpen struct {
	Count int             `json:"count"`
	Root  []SnapshotEntry `json:"root"`
}

func snapshotParentOf(p string) string {
	if i := strings.LastIndexByte(p, '/'); i >= 0 {
		return p[:i]
	}
	return ""
}

func buildSnapshotTreeIndex(key snapshotCacheKey, entries []SnapshotEntry) *snapshotTreeIndex {
	idx := &snapshotTreeIndex{
		key:      key,
		count:    len(entries),
		children: make(map[string][]SnapshotEntry),
		dirBytes: make(map[string]uint64),
	}
	idx.children[""] = nil
	for _, e := range entries {
		parent := snapshotParentOf(e.Path)
		idx.children[parent] = append(idx.children[parent], e)
		if e.IsDir {
			if _, ok := idx.children[e.Path]; !ok {
				idx.children[e.Path] = nil
			}
			if _, ok := idx.dirBytes[e.Path]; !ok {
				idx.dirBytes[e.Path] = 0
			}
			continue
		}
		if e.Size == 0 {
			continue
		}
		// Credit the file to every ancestor, including ancestors the archive
		// never emitted a directory entry for.
		for p := parent; ; p = snapshotParentOf(p) {
			idx.dirBytes[p] += e.Size
			if p == "" {
				break
			}
		}
	}
	return idx
}

// selectionBytes sums what restoring paths would write. A selected directory
// covers everything below it, so a path with a selected ancestor is skipped
// instead of being counted twice.
func (idx *snapshotTreeIndex) selectionBytes(paths []string) uint64 {
	selected := make(map[string]struct{}, len(paths))
	for _, p := range paths {
		selected[p] = struct{}{}
	}
	var sum uint64
	for p := range selected {
		covered := false
		for a := snapshotParentOf(p); a != ""; a = snapshotParentOf(a) {
			if _, ok := selected[a]; ok {
				covered = true
				break
			}
		}
		if covered {
			continue
		}
		if b, isDir := idx.dirBytes[p]; isDir {
			sum += b
			continue
		}
		for _, e := range idx.children[snapshotParentOf(p)] {
			if e.Path == p && !e.IsDir {
				sum += e.Size
				break
			}
		}
	}
	return sum
}

// snapshotIndexFor returns the index for the snapshot in opts, building it
// from the (cached) listing when it is not the one currently held.
func snapshotIndexFor(opts RestoreOptions, forceRefresh bool) (*snapshotTreeIndex, error) {
	key := buildSnapshotCacheKey(opts)
	snapshotIndexMu.Lock()
	defer snapshotIndexMu.Unlock()
	if !forceRefresh && snapshotIndexCur != nil && snapshotIndexCur.key == key {
		return snapshotIndexCur, nil
	}
	snapshotIndexCur = nil
	entries, err := ListSnapshotContentsInline(opts, "", forceRefresh)
	if err != nil {
		return nil, err
	}
	start := time.Now()
	idx := buildSnapshotTreeIndex(key, entries)
	writeBackupLog(fmt.Sprintf("Snapshot tree index built: %d entries, %d dirs in %v",
		idx.count, len(idx.dirBytes), time.Since(start).Round(time.Millisecond)))
	snapshotIndexCur = idx
	return idx, nil
}

func (a *App) snapshotOpts(pbsID, backupID string, snapshotUnix int64) (RestoreOptions, error) {
	cfg, err := a.resolveRestorePBS(pbsID)
	if err != nil {
		return RestoreOptions{}, err
	}
	if backupID == "" {
		return RestoreOptions{}, fmt.Errorf("backup ID required")
	}
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
		SnapshotTime:    time.Unix(snapshotUnix, 0),
		Crypt:           cfg.Crypt,
	}, nil
}

// OpenSnapshotTree loads a snapshot's listing (local cache, else the catalog)
// into the in-memory index and returns only its top level.
func (a *App) OpenSnapshotTree(pbsID, backupID string, snapshotUnix int64, forceRefresh bool) (*SnapshotTreeOpen, error) {
	writeDebugLog(fmt.Sprintf("OpenSnapshotTree(pbs=%s, backupID=%s, unix=%d, force=%v)", pbsID, backupID, snapshotUnix, forceRefresh))
	opts, err := a.snapshotOpts(pbsID, backupID, snapshotUnix)
	if err != nil {
		return nil, err
	}
	idx, err := snapshotIndexFor(opts, forceRefresh)
	if err != nil {
		return nil, err
	}
	return &SnapshotTreeOpen{Count: idx.count, Root: idx.children[""]}, nil
}

// ListSnapshotChildren returns the direct children of one directory ("" = root).
func (a *App) ListSnapshotChildren(pbsID, backupID string, snapshotUnix int64, dir string) ([]SnapshotEntry, error) {
	opts, err := a.snapshotOpts(pbsID, backupID, snapshotUnix)
	if err != nil {
		return nil, err
	}
	idx, err := snapshotIndexFor(opts, false)
	if err != nil {
		return nil, err
	}
	return idx.children[dir], nil
}

// SnapshotSelectionBytes returns the total size restoring the given paths
// would write (directories count everything below them).
func (a *App) SnapshotSelectionBytes(pbsID, backupID string, snapshotUnix int64, paths []string) (uint64, error) {
	opts, err := a.snapshotOpts(pbsID, backupID, snapshotUnix)
	if err != nil {
		return 0, err
	}
	idx, err := snapshotIndexFor(opts, false)
	if err != nil {
		return 0, err
	}
	return idx.selectionBytes(paths), nil
}
