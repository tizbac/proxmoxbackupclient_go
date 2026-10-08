package pbscommon

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
)

const (
	// FileMetaBlobName is the PBS blob name of the per-snapshot metadata
	// side-car (NTFS ACLs on Windows, extended attributes incl. POSIX ACLs on
	// Linux). It must match the PBS file-name regex: bare basename, no leading
	// dot, ending in ".blob". The payload is gzipped JSON.
	FileMetaBlobName = "proxmox-client-acls.json.gz.blob"
	FileMetaVersion  = 1
)

// FileMetaEntry is the per-file/dir NTFS (Windows) or extended-attribute
// (Linux) metadata captured during the PXAR walk.
type FileMetaEntry struct {
	Path    string `json:"p"` // archive-relative path using forward slashes
	IsDir   bool   `json:"d,omitempty"`
	SDDLIdx int    `json:"s"`           // Windows: index into BackupFileMeta.SDDLs (dedup)
	Attrs   uint32 `json:"a"`           // Windows file attributes bitmask
	Reparse uint32 `json:"r,omitempty"` // reparse tag (0 if not a reparse point)
	// Created is the Windows creation time as a FILETIME (100 ns ticks since
	// 1601-01-01 UTC), 0 when not captured. pxar has no field for it (Linux
	// has no settable creation time), so it travels here. Added 2026-10-08;
	// older readers ignore it.
	Created int64 `json:"c,omitempty"`
	// Xattrs is Linux-only: every extended attribute on this file, name ->
	// raw value. POSIX ACLs travel here under their kernel names
	// (system.posix_acl_access / system.posix_acl_default), as raw bytes.
	Xattrs map[string][]byte `json:"x,omitempty"`
}

// BackupFileMeta is the root of the metadata side-car for one archive.
type BackupFileMeta struct {
	Version   int             `json:"version"`
	Root      string          `json:"root"`     // filesystem root that was walked
	Captured  string          `json:"captured"` // RFC3339 timestamp
	Host      string          `json:"host"`
	Collected int             `json:"collected"` // count of entries
	Errors    int             `json:"errors"`    // count of files that failed lookup
	SDDLs     []string        `json:"sddl"`      // dedup dictionary
	Entries   []FileMetaEntry `json:"entries"`
}

// CombinedBackupFileMeta aggregates one BackupFileMeta per archive into a
// single blob for the snapshot, keyed by the bare archive name (without the
// ".pxar.didx" suffix).
type CombinedBackupFileMeta struct {
	Version  int                        `json:"version"`
	Captured string                     `json:"captured"` // RFC3339 timestamp
	Host     string                     `json:"host"`
	Archives map[string]*BackupFileMeta `json:"archives"`
}

// SerializeFileMeta gzips a single archive's metadata.
func SerializeFileMeta(meta *BackupFileMeta) ([]byte, error) {
	if meta == nil {
		return nil, nil
	}
	return gzipJSON(meta)
}

// Serialize gzips the combined metadata.
func (m *CombinedBackupFileMeta) Serialize() ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	return gzipJSON(m)
}

func gzipJSON(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if err := json.NewEncoder(gz).Encode(v); err != nil {
		_ = gz.Close()
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
