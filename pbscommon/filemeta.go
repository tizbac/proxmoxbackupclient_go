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
	// Streams are the file's (or folder's) alternate data streams on NTFS,
	// such as Zone.Identifier; EAs its NTFS extended attributes. Windows
	// only, added 2026-10-14; older readers ignore them.
	Streams []StreamEntry `json:"t,omitempty"`
	EAs     []EAEntry     `json:"e,omitempty"`
}

// StreamEntry is one alternate data stream. Skipped is empty when its
// content is in Data (an empty stream has no Data); otherwise it says why the
// content was not recorded (larger than
// MaxStreamBytes, over the backup's MaxStreamBytesTotal, unreadable, or
// dropped to keep the metadata blob under PBS's blob limit).
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

const (
	// MaxStreamBytes is the largest alternate data stream recorded with its
	// content. Nearly all are a few hundred bytes (Zone.Identifier).
	MaxStreamBytes = 1 << 20
	// MaxStreamBytesTotal bounds the stream content one archive records, so
	// the metadata blob stays well inside PBS's blob size limit.
	MaxStreamBytesTotal = 8 << 20
	// MaxFileMetaBlobBytes is the size the serialized metadata blob must stay
	// under (PBS refuses blobs over 16 MiB).
	MaxFileMetaBlobBytes = 15 << 20
)

// SerializeCapped is Serialize, but if the blob would be larger than limit
// the recorded stream contents are dropped (each marked Skipped) and it is
// serialized again, so permissions and attributes are never lost to a large
// stream. dropped counts the streams whose content was dropped.
func (m *CombinedBackupFileMeta) SerializeCapped(limit int) (blob []byte, dropped int, err error) {
	blob, err = m.Serialize()
	if err != nil || len(blob) <= limit || m == nil {
		return blob, 0, err
	}
	for _, a := range m.Archives {
		if a == nil {
			continue
		}
		for i := range a.Entries {
			for j := range a.Entries[i].Streams {
				s := &a.Entries[i].Streams[j]
				if s.Skipped == "" {
					s.Data, s.Skipped = nil, "metadata record too large"
					dropped++
				}
			}
		}
	}
	if dropped == 0 {
		return blob, 0, nil
	}
	blob, err = m.Serialize()
	return blob, dropped, err
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
