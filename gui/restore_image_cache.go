package main

import (
	"compress/gzip"
	"encoding/gob"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// The volumes and file lists of a disk-image snapshot, kept on disk next to
// the restore tree cache. Snapshots never change, so after a restart a
// restore (or a second browse) uses this instead of reading the volumes'
// file tables again, which for a large NTFS or ext4 volume means downloading
// gigabytes of chunks and a minute or more.

const imageCacheSchema = 1

type imageVolumesCache struct {
	Schema int
	Key    snapshotCacheKey
	Vols   []imageVolume
}

func imageCachePath(key snapshotCacheKey) (string, error) {
	dir, err := getRestoreCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, key.fingerprint()+".img.gz"), nil
}

// loadImageVolumes returns the cached volumes of the snapshot key names.
func loadImageVolumes(key snapshotCacheKey) ([]imageVolume, bool) {
	p, err := imageCachePath(key)
	if err != nil {
		return nil, false
	}
	f, err := os.Open(p) // #nosec G304 -- name is a fingerprint of the key
	if err != nil {
		return nil, false
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		return nil, false
	}
	var c imageVolumesCache
	if err := gob.NewDecoder(zr).Decode(&c); err != nil || c.Schema != imageCacheSchema || c.Key != key {
		return nil, false
	}
	return c.Vols, true
}

// saveImageVolumes stores the volumes of a listed snapshot (atomically).
func saveImageVolumes(key snapshotCacheKey, vols []imageVolume) error {
	p, err := imageCachePath(key)
	if err != nil {
		return err
	}
	tmp := p + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	zw, _ := gzip.NewWriterLevel(f, gzip.BestSpeed)
	err = gob.NewEncoder(zw).Encode(imageVolumesCache{Schema: imageCacheSchema, Key: key, Vols: vols})
	if cerr := zw.Close(); err == nil {
		err = cerr
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(tmp)
		return fmt.Errorf("image list cache: %w", err)
	}
	if err := os.Rename(tmp, p); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// trimImageVolumeCache removes cached image lists older than maxAge.
func trimImageVolumeCache(maxAge time.Duration) {
	dir, err := getRestoreCacheDir()
	if err != nil {
		return
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range ents {
		if !strings.HasSuffix(e.Name(), ".img.gz") {
			continue
		}
		if fi, err := e.Info(); err == nil && time.Since(fi.ModTime()) > maxAge {
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}
