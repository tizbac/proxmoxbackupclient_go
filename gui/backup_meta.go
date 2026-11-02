package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"

	"pbscommon"
)

const (
	BackupMetaFilename = pbscommon.BackupMetaFilename
	BackupAclsFilename = pbscommon.FileMetaBlobName
	FileMetaFormatVers = pbscommon.FileMetaVersion
)

type (
	FileMetaEntry          = pbscommon.FileMetaEntry
	BackupFileMeta         = pbscommon.BackupFileMeta
	CombinedBackupFileMeta = pbscommon.CombinedBackupFileMeta
)

// SerializeFileMeta gzips a single archive's metadata.
func SerializeFileMeta(meta *BackupFileMeta) ([]byte, error) {
	return pbscommon.SerializeFileMeta(meta)
}

// downloadCombinedBackupFileMeta fetches and decodes the NTFS ACL/attributes
// blob (BackupAclsFilename) for the snapshot the given client is currently
// connected to. Returns (nil, nil) — not an error — when the blob is simply
// absent, which is the normal case for a snapshot backed up before this
// feature existed, or one from a non-Windows source with nothing to capture:
// restore must proceed without ACL/attribute application either way, never
// fail because of it.
func downloadCombinedBackupFileMeta(client *pbscommon.PBSClient) (*CombinedBackupFileMeta, error) {
	raw, err := client.DownloadBlob(BackupAclsFilename)
	if err != nil {
		writeBackupLog(fmt.Sprintf("NTFS ACL/attributes blob not available for this snapshot (restoring without it): %v", err))
		return nil, nil
	}
	gz, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	payload, err := io.ReadAll(gz)
	if err != nil {
		return nil, err
	}
	var meta CombinedBackupFileMeta
	if err := json.Unmarshal(payload, &meta); err != nil {
		return nil, err
	}
	return &meta, nil
}

// buildFileMetaIndex turns a BackupFileMeta's Entries slice into a
// path->entry lookup, built once per archive instead of scanning the
// (potentially large) slice for every restored file.
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
type BackupMeta = pbscommon.BackupMeta

func GenerateBackupMeta(backupID, originalPath, hostname string, vssUsed bool) ([]byte, error) {
	return pbscommon.GenerateBackupMeta(backupID, originalPath, hostname, appVersion, vssUsed)
}
