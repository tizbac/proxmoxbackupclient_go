package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// cachedTreeSchema gates the on-disk format. Bump when SnapshotEntry layout or
// the cache envelope changes so stale files are ignored instead of misparsed.
//
// v2: added optional Meta (BackupMeta sidecar) so GetSnapshotMeta is free
// after a listing.
const cachedTreeSchema = 2

// snapshotCacheKey identifies a single snapshot's content listing. The whole
// tuple is hashed into the filename so unrelated cache lines stay isolated
// even when several PBS servers reuse a backup-id.
type snapshotCacheKey struct {
	PBSID      string
	Datastore  string
	Namespace  string
	BackupType string
	BackupID   string
	SnapshotAt int64
}

// cachedSnapshotTree is the on-disk envelope. We keep the key inside the file
// so a stray hash collision (or a copied cache dir) doesn't return the wrong
// tree silently.
//
// Meta is the `.nimbus_backup_meta.json` sidecar parsed at listing time. nil
// means "snapshot has no sidecar" (legacy backup), not "we haven't looked".
type cachedSnapshotTree struct {
	Schema      int              `json:"schema"`
	GeneratedAt int64            `json:"generated_at"`
	Key         snapshotCacheKey `json:"key"`
	Entries     []SnapshotEntry  `json:"entries"`
	Meta        *BackupMeta      `json:"meta,omitempty"`
}

func getRestoreCacheDir() (string, error) {
	base, err := getConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "restore_cache")
	// #nosec G303 -- cache files are local to the user's profile, never executable
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create restore cache dir: %w", err)
	}
	return dir, nil
}

func (k snapshotCacheKey) fingerprint() string {
	h := sha256.New()
	// Use byte 0 as a separator so "ab|c" and "a|bc" don't collide.
	for _, part := range []string{k.PBSID, k.Datastore, k.Namespace, k.BackupType, k.BackupID, fmt.Sprintf("%d", k.SnapshotAt)} {
		h.Write([]byte(part))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func (k snapshotCacheKey) filename() (string, error) {
	dir, err := getRestoreCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, k.fingerprint()+".json"), nil
}

// loadSnapshotTreeCache returns (envelope, true) on a fresh hit, or (nil, false)
// on any miss/parse/age failure. The key inside the file must match exactly —
// if it doesn't, we treat the line as garbage and let the caller refetch.
//
// Returns the full envelope (entries + meta) so callers can use either one
// without re-reading the file.
func loadSnapshotTreeCache(key snapshotCacheKey) (*cachedSnapshotTree, bool) {
	path, err := key.filename()
	if err != nil {
		return nil, false
	}
	// #nosec G304 -- path is derived from a SHA-256 fingerprint of trusted inputs
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var c cachedSnapshotTree
	if err := json.Unmarshal(data, &c); err != nil {
		writeBackupLog(fmt.Sprintf("Restore cache: ignoring malformed file %s: %v", filepath.Base(path), err))
		return nil, false
	}
	if c.Schema != cachedTreeSchema {
		return nil, false
	}
	if c.Key != key {
		// Collision or copied cache from another profile.
		writeBackupLog(fmt.Sprintf("Restore cache: key mismatch on %s — refetching", filepath.Base(path)))
		return nil, false
	}
	return &c, true
}

func saveSnapshotTreeCache(key snapshotCacheKey, entries []SnapshotEntry, meta *BackupMeta) error {
	path, err := key.filename()
	if err != nil {
		return err
	}
	envelope := cachedSnapshotTree{
		Schema:      cachedTreeSchema,
		GeneratedAt: time.Now().Unix(),
		Key:         key,
		Entries:     entries,
		Meta:        meta,
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("marshal cache: %w", err)
	}
	// Atomic write: temp file in the same dir + rename. Avoids leaving a
	// half-written JSON behind if the GUI is killed mid-flush.
	tmp := path + ".tmp"
	// #nosec G306 -- 0644 is fine; cache content is non-sensitive listing metadata
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return fmt.Errorf("write tmp cache: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename cache: %w", err)
	}
	return nil
}

// trimSnapshotTreeCache deletes cache files older than maxAge. Returns the
// number of files removed. Errors on individual files are logged but never
// fail the whole pass — best-effort housekeeping.
func trimSnapshotTreeCache(maxAge time.Duration) int {
	dir, err := getRestoreCacheDir()
	if err != nil {
		writeBackupLog(fmt.Sprintf("Restore cache trim: cannot resolve dir: %v", err))
		return 0
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		writeBackupLog(fmt.Sprintf("Restore cache trim: cannot list %s: %v", dir, err))
		return 0
	}
	cutoff := time.Now().Add(-maxAge)
	removed := 0
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			if err := os.Remove(filepath.Join(dir, e.Name())); err != nil {
				writeBackupLog(fmt.Sprintf("Restore cache trim: remove %s failed: %v", e.Name(), err))
				continue
			}
			removed++
		}
	}
	if removed > 0 {
		writeBackupLog(fmt.Sprintf("Restore cache trim: removed %d entries older than %s", removed, maxAge))
	}
	return removed
}
