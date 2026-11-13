package pbscommon

import "fmt"

// A snapshot says, in its manifest, which version of this client wrote it and
// what a client needs to be able to read it. Version 2.0 and older wrote
// nothing of the kind, and nothing told a restore that a snapshot was written
// in a form it could not read until a chunk failed to decode: 2.0 fails on a
// snapshot with compressed encrypted chunks (first written by 2.1) with
// "compressed encrypted blobs (zstd block format) are not supported".
//
// The note is in the manifest's "unprotected" section, which PBS stores as it
// is given (the rest of the manifest is rebuilt from the fields PBS knows, see
// BackupManifest) and which is outside the signature, so it can be read
// without the encryption key. Being unsigned, it is advice for a message, not
// something a restore's safety rests on.

// Snapshot formats, in the order they came. A snapshot names the newest one
// it may use, and a client reads every format up to ReadableSnapshotFormat.
const (
	// SnapshotFormatBase is what every version reads; it is not written out.
	SnapshotFormatBase = 0
	// SnapshotFormatEncrCompr is an encrypted snapshot whose chunks and blobs
	// may be compressed before they are encrypted (ENCR_COMPR), as the
	// official client writes them. Named on every encrypted snapshot,
	// whatever the compression setting and whether or not the key is set to
	// compress (CryptConfig.WithCompression): a chunk PBS already holds is
	// not sent again, so a backup that writes none compressed can still
	// refer to compressed chunks an earlier backup stored.
	SnapshotFormatEncrCompr = 1

	// ReadableSnapshotFormat is the newest format this build restores.
	ReadableSnapshotFormat = SnapshotFormatEncrCompr
)

// snapshotFormatMinVersion is the first version of this client that reads
// each format, for the manifest note and the messages.
var snapshotFormatMinVersion = map[int]string{
	SnapshotFormatEncrCompr: "2.1.0",
}

// SnapshotWriter is the manifest's note of the client that wrote a snapshot.
type SnapshotWriter struct {
	// Version is the version of this client that wrote the snapshot.
	Version string `json:"version,omitempty"`
	// Format is the newest snapshot format the snapshot may use.
	Format int `json:"format,omitempty"`
	// MinVersion is the oldest version of this client that reads Format, so
	// a client too old to know the format can still say what is needed.
	MinVersion string `json:"min-version,omitempty"`
}

// clientVersion is this program's version, for SnapshotWriter.Version.
var clientVersion string

// SetClientVersion tells the package the version of the program it is built
// into, once at start. Unset (tests, a build with no version) reads "dev".
func SetClientVersion(v string) { clientVersion = v }

// snapshotWriter is the note for a snapshot this client is about to finish.
func (pbs *PBSClient) snapshotWriter() *SnapshotWriter {
	w := &SnapshotWriter{Version: clientVersion}
	if w.Version == "" {
		w.Version = "dev"
	}
	if pbs.Crypt != nil {
		w.Format = SnapshotFormatEncrCompr
		w.MinVersion = snapshotFormatMinVersion[w.Format]
	}
	return w
}

// SnapshotFormatError says a snapshot was written in a format newer than
// this build reads.
type SnapshotFormatError struct {
	Archive    string
	WrittenBy  string // the version that wrote the snapshot, "" if not named
	MinVersion string // the oldest version that reads it, "" if not named
}

func (e *SnapshotFormatError) Error() string {
	by := "a newer version of this client"
	if e.WrittenBy != "" {
		by = "version " + e.WrittenBy + " of this client"
	}
	need := "a newer version"
	if e.MinVersion != "" {
		need = "version " + e.MinVersion + " or later"
	}
	return fmt.Sprintf("this snapshot was written by %s in a format this version cannot read: restoring it needs %s", by, need)
}

// archiveFormatError is CheckArchiveKey's decision on the snapshot's format,
// given the manifest: nil when the manifest names no writer (an older
// version, or another client) or a format this build reads.
func archiveFormatError(m *BackupManifest, archiveName string) error {
	w := m.Unprotected.Writer
	if w == nil || w.Format <= ReadableSnapshotFormat {
		return nil
	}
	return &SnapshotFormatError{Archive: archiveName, WrittenBy: w.Version, MinVersion: w.MinVersion}
}
