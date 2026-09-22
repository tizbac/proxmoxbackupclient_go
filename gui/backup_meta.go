package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"runtime"
	"time"
)

const (
	BackupMetaFilename  = ".proxmox_backup_client_meta.json"
	// BackupAclsFilename is the PBS blob name. It must match the PBS
	// file-name regex: bare basename, no leading dot, and end in ".blob".
	// The payload is still gzipped JSON — the ".blob" suffix is the PBS
	// container extension, the ".json.gz" inside describes the content.
	BackupAclsFilename  = "proxmox-client-acls.json.gz.blob"
	FileMetaFormatVers  = 1
)

// FileMetaEntry is the per-file/dir NTFS metadata captured before the PXAR walk.
// Stored in a gzipped JSON side-car inside the archive root (as a VirtualFile).
//
// Field names use short keys to keep the serialized size small: for a tree of
// 500k files and 50 unique SDDLs, the whole file is typically 5-15 MB gzipped.
type FileMetaEntry struct {
	Path    string `json:"p"` // archive-relative path using forward slashes
	IsDir   bool   `json:"d,omitempty"`
	SDDLIdx int    `json:"s"`     // index into BackupFileMeta.SDDLs (dedup)
	Attrs   uint32 `json:"a"`     // Windows file attributes bitmask
	Reparse uint32 `json:"r,omitempty"` // reparse tag (0 if not a reparse point)
}

// BackupFileMeta is the root of the ACL side-car. SDDLs are deduplicated into
// a string array and each entry references by index.
type BackupFileMeta struct {
	Version   int             `json:"version"`
	Root      string          `json:"root"`    // filesystem root that was walked
	Captured  string          `json:"captured"` // RFC3339 timestamp
	Host      string          `json:"host"`
	Collected int             `json:"collected"`  // count of entries
	Errors    int             `json:"errors"`     // count of files that failed ACL lookup
	SDDLs     []string        `json:"sddl"`       // dedup dictionary
	Entries   []FileMetaEntry `json:"entries"`
}

// SerializeFileMeta streams the metadata through gzip to minimize peak memory.
func SerializeFileMeta(meta *BackupFileMeta) ([]byte, error) {
	if meta == nil {
		return nil, nil
	}
	return gzipJSON(meta)
}

// CombinedBackupFileMeta aggregates one BackupFileMeta per archive into a
// SINGLE blob for a multi-archive snapshot (one job → one snapshot, per
// directory/folder selected). Each archive's entries keep their own Root, so
// two archives that happen to contain a same-named relative path never
// collide — this is the fix for the per-archive-blob-name-collision problem
// found 2026-09-21 (see project_windows_pbs_client_fork.md): uploading
// BackupAclsFilename once per directory into a shared multi-archive session
// would just overwrite the same blob N times, silently losing every
// directory's ACL data but the last. One combined blob per snapshot instead.
type CombinedBackupFileMeta struct {
	Version  int                        `json:"version"`
	Captured string                     `json:"captured"` // RFC3339 timestamp
	Host     string                     `json:"host"`
	Archives map[string]*BackupFileMeta `json:"archives"` // keyed by archive base name
}

// Serialize gzips the combined metadata the same way a single-archive
// BackupFileMeta is serialized (see SerializeFileMeta).
func (m *CombinedBackupFileMeta) Serialize() ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	return gzipJSON(m)
}

// gzipJSON is the shared gzip+JSON encoder behind SerializeFileMeta and
// CombinedBackupFileMeta.Serialize.
func gzipJSON(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	enc := json.NewEncoder(gz)
	if err := enc.Encode(v); err != nil {
		_ = gz.Close()
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// BackupMeta stores metadata about the backup, injected as a virtual file
// at the root of the PXAR archive. Allows restore tools to recover the
// original path (with spaces, accents) from the sanitized backup-id.
type BackupMeta struct {
	BackupID      string `json:"backup_id"`
	OriginalPath  string `json:"original_path"`
	Hostname      string `json:"hostname"`
	BackupTime    string `json:"backup_time"`
	ClientVersion string `json:"client_version"`
	OS            string `json:"os"`
	VSSUsed       bool   `json:"vss_used"`
}

func GenerateBackupMeta(backupID, originalPath, hostname string, vssUsed bool) ([]byte, error) {
	meta := BackupMeta{
		BackupID:      backupID,
		OriginalPath:  originalPath,
		Hostname:      hostname,
		BackupTime:    time.Now().UTC().Format(time.RFC3339),
		ClientVersion: appVersion,
		OS:            runtime.GOOS,
		VSSUsed:       vssUsed,
	}
	return json.MarshalIndent(meta, "", "  ")
}
