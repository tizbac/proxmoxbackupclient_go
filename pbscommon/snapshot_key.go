package pbscommon

import (
	"encoding/json"
	"fmt"
	"strings"
)

// SnapshotKeyError says an encrypted archive cannot be read with the key
// given: the snapshot was encrypted with another key, or no key was given.
// Without this check the restore failed on its first chunk with "unable to
// decrypt data: cipher: message authentication failed" (or "chunk ... is
// encrypted but no encryption key was provided"), which does not say which
// key is needed.
type SnapshotKeyError struct {
	Archive     string
	SnapshotKey string // fingerprint the snapshot's manifest names, "" if none
	GivenKey    string // fingerprint of the key given, "" when no key was given
}

func (e *SnapshotKeyError) Error() string {
	switch {
	case e.GivenKey != "":
		return fmt.Sprintf("this snapshot was encrypted with another key: it needs the key with fingerprint %s, and the key in use has fingerprint %s", e.SnapshotKey, e.GivenKey)
	case e.SnapshotKey != "":
		return fmt.Sprintf("this snapshot is encrypted: it needs the encryption key with fingerprint %s, and no key is in use", e.SnapshotKey)
	default:
		return "this snapshot is encrypted: it needs its encryption key, and no key is in use"
	}
}

// CheckArchiveKey reads the snapshot's manifest (index.json.blob, which is
// never encrypted) in this reader session and returns a *SnapshotKeyError
// when archiveName is encrypted and pbs.Crypt cannot read it. It returns nil
// when the archive is not encrypted, when the key matches, and when the
// manifest cannot be read or says nothing either way: the restore then goes
// ahead and fails, if it must, on the first chunk as before.
func (pbs *PBSClient) CheckArchiveKey(archiveName string) error {
	raw, err := pbs.DownloadBlob(ManifestBlobName)
	if err != nil {
		return nil
	}
	var m BackupManifest
	if json.Unmarshal(raw, &m) != nil {
		return nil
	}
	return archiveKeyError(&m, archiveName, pbs.Crypt)
}

// archiveKeyError is CheckArchiveKey's decision, given the manifest.
func archiveKeyError(m *BackupManifest, archiveName string, crypt *CryptConfig) error {
	encrypted := false
	for _, f := range m.Files {
		if f.Filename == archiveName {
			encrypted = f.CryptMode == CryptModeEncrypt
			break
		}
	}
	if !encrypted {
		return nil
	}
	want := ""
	if m.Unprotected.KeyFingerprint != nil {
		want = strings.ToLower(strings.TrimSpace(*m.Unprotected.KeyFingerprint))
	}
	if crypt == nil {
		return &SnapshotKeyError{Archive: archiveName, SnapshotKey: want}
	}
	have := crypt.Fingerprint()
	if want == "" || want == have {
		// An encrypted snapshot whose manifest names no key cannot be
		// checked here; its chunks will say whether the key is right.
		return nil
	}
	return &SnapshotKeyError{Archive: archiveName, SnapshotKey: want, GivenKey: have}
}
