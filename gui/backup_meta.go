package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"runtime"
	"time"

	"pbscommon"
)

const (
	BackupMetaFilename = ".proxmox_backup_client_meta.json"
	// BackupAclsFilename is the PBS blob name. It must match the PBS
	// file-name regex: bare basename, no leading dot, and end in ".blob".
	// The payload is still gzipped JSON — the ".blob" suffix is the PBS
	// container extension, the ".json.gz" inside describes the content.
	BackupAclsFilename = "proxmox-client-acls.json.gz.blob"
	FileMetaFormatVers = 1
)

// FileMetaEntry is the per-file/dir NTFS (Windows) or extended-attribute
// (Linux) metadata captured before the PXAR walk. Stored in a gzipped JSON
// side-car inside the archive root (as a VirtualFile).
//
// Field names use short keys to keep the serialized size small: for a tree of
// 500k files and 50 unique SDDLs, the whole file is typically 5-15 MB gzipped.
type FileMetaEntry struct {
	Path    string `json:"p"` // archive-relative path using forward slashes
	IsDir   bool   `json:"d,omitempty"`
	SDDLIdx int    `json:"s"`           // Windows: index into BackupFileMeta.SDDLs (dedup)
	Attrs   uint32 `json:"a"`           // Windows file attributes bitmask
	Reparse uint32 `json:"r,omitempty"` // reparse tag (0 if not a reparse point)
	// Created is the Windows creation time as a FILETIME (100 ns ticks since
	// 1601-01-01 UTC), 0 when not captured. pxar has no field for it (Linux
	// has no settable creation time), so it travels here. Optional: older
	// readers ignore the key, and an older snapshot without it restores as
	// before.
	Created int64 `json:"c,omitempty"`
	// Xattrs is Linux-only: every extended attribute captured on this file,
	// name -> raw value (encoding/json base64-encodes []byte automatically).
	// POSIX ACLs are included here under their real kernel xattr names
	// (system.posix_acl_access, system.posix_acl_default) rather than
	// decoded — the kernel already defines their binary layout as a stable
	// ABI, so copying the raw bytes back via Setxattr on restore is exactly
	// as correct as (and far simpler than) parsing and re-encoding them.
	// No dedup: unlike Windows SDDLs, xattr values vary too much per-file
	// for a dictionary to earn its complexity, and the blob is already
	// gzip-compressed, which absorbs most of the repetition anyway.
	Xattrs map[string][]byte `json:"x,omitempty"`
	// Streams are the file's (or folder's) alternate data streams on NTFS,
	// such as Zone.Identifier; EAs its NTFS extended attributes. Windows
	// only and optional: older readers ignore them.
	Streams []StreamEntry `json:"t,omitempty"`
	EAs     []EAEntry     `json:"e,omitempty"`
}

// StreamEntry is one alternate data stream. Skipped is empty when its content
// is in Data (an empty stream has no Data); otherwise it says why the content
// was not recorded (larger than maxStreamBytes, over the archive's
// maxStreamBytesTotal, unreadable, or dropped to keep the side file under the
// PBS blob limit) and the restore leaves the stream out.
type StreamEntry struct {
	Name    string `json:"n"`
	Size    int64  `json:"z"`
	Data    []byte `json:"d,omitempty"`
	Skipped string `json:"k,omitempty"`
}

// EAEntry is one NTFS extended attribute (FILE_FULL_EA_INFORMATION).
type EAEntry struct {
	Name  string `json:"n"`
	Flags uint8  `json:"f,omitempty"`
	Value []byte `json:"v,omitempty"`
}

// BackupFileMeta is the root of the ACL side-car. SDDLs are deduplicated into
// a string array and each entry references by index.
type BackupFileMeta struct {
	Version   int             `json:"version"`
	Root      string          `json:"root"`     // filesystem root that was walked
	Captured  string          `json:"captured"` // RFC3339 timestamp
	Host      string          `json:"host"`
	Collected int             `json:"collected"` // count of entries
	Errors    int             `json:"errors"`    // count of files that failed ACL lookup
	SDDLs     []string        `json:"sddl"`      // dedup dictionary
	Entries   []FileMetaEntry `json:"entries"`
}

// SerializeFileMeta streams the metadata through gzip to minimize peak memory.
func SerializeFileMeta(meta *BackupFileMeta) ([]byte, error) {
	if meta == nil {
		return nil, nil
	}
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	enc := json.NewEncoder(gz)
	if err := enc.Encode(meta); err != nil {
		_ = gz.Close()
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// downloadBackupFileMeta fetches and decodes the NTFS ACL/attributes blob
// (BackupAclsFilename) for the snapshot the given client is currently
// connected to. Returns (nil, nil) — not an error — when the blob is simply
// absent, which is the normal case for a snapshot backed up before this
// feature existed, or one with nothing to capture: restore must proceed
// without ACL/attribute application either way, never fail because of it.
func downloadBackupFileMeta(client *pbscommon.PBSClient) (*BackupFileMeta, error) {
	raw, err := client.DownloadBlob(BackupAclsFilename)
	if err != nil {
		writeBackupLog(fmt.Sprintf("NTFS ACL/attributes blob not available for this snapshot (restoring without it): %v", err))
		return nil, nil
	}
	gz, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	defer func() { _ = gz.Close() }()
	payload, err := io.ReadAll(gz)
	if err != nil {
		return nil, err
	}
	var meta BackupFileMeta
	if err := json.Unmarshal(payload, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

// buildFileMetaIndex turns a BackupFileMeta's Entries slice into a
// path->entry lookup, built once instead of scanning the (potentially large)
// slice for every restored file.
func buildFileMetaIndex(meta *BackupFileMeta) map[string]FileMetaEntry {
	if meta == nil {
		return nil
	}
	idx := make(map[string]FileMetaEntry, len(meta.Entries))
	for _, e := range meta.Entries {
		idx[e.Path] = e
	}
	return idx
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
