package api

import "strings"

// cancelledMessage is the sentinel error text produced by the backup layers
// (machinebackuplib.ErrCancelled) when the user presses Stop.
const cancelledMessage = "backup cancelled by user"

// isCancelled reports whether a job ended because the user pressed Stop rather
// than because something broke.
//
// The api module is deliberately dependency-free (see go.mod: no requires at
// all), so it cannot import machinebackuplib to match the sentinel with
// errors.Is — it matches the message instead. Callers pass errors that still
// carry that text verbatim: the backup layer returns the bare sentinel on
// cancellation, and wrapping uses %w/%v which preserves it.
func isCancelled(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return msg == cancelledMessage ||
		strings.HasSuffix(msg, "Machine backup cancelled by user") ||
		strings.HasSuffix(msg, "Backup annulé par l'utilisateur")
}
