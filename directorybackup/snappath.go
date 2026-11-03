package main

import "strings"

// unmapSnapshotPathString replaces the snapshot location of a directory (on
// Windows a path under the VSS folder) with the directory the user asked for,
// so messages name something the user recognises.
func unmapSnapshotPathString(s, snapshotDir, originalDir string) string {
	if snapshotDir == "" || snapshotDir == originalDir {
		return s
	}
	// A drive or file system root ("C:\", "/") ends with its separator and
	// the snapshot's path does not, so a child would come out as C:\\Users.
	if n := len(originalDir); n > 0 && (originalDir[n-1] == '\\' || originalDir[n-1] == '/') {
		s = strings.ReplaceAll(s, snapshotDir+originalDir[n-1:], originalDir)
	}
	return strings.ReplaceAll(s, snapshotDir, originalDir)
}

type unmappedError struct {
	msg string
	err error
}

func (e *unmappedError) Error() string { return e.msg }
func (e *unmappedError) Unwrap() error { return e.err }

func unmapSnapshotPath(err error, snapshotDir, originalDir string) error {
	if err == nil || snapshotDir == "" || snapshotDir == originalDir {
		return err
	}
	msg := err.Error()
	mapped := unmapSnapshotPathString(msg, snapshotDir, originalDir)
	if mapped == msg {
		return err
	}
	return &unmappedError{msg: mapped, err: err}
}
