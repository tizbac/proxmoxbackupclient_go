package retry

import (
	"strings"
	"time"
)

// What to do when a backup's connection to PBS fails, shared by the app and
// the command-line tools. A single request that fails on its own (a timeout,
// a 503) is retried in place with Do and DefaultRetryable. A lost connection
// is different: PBS ends the backup session with it, so the backup starts
// again with a new session (SessionLost, Reconnectable and the waits below).

// SessionLost reports errors that make the current PBS backup session
// unusable. They mean the HTTP/2 connection was lost: the session on the
// server is gone, and every later request in it fails too. The only recovery
// is a fresh Connect() with a new backup time, which runs the backup again.
func SessionLost(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "PBS session cannot be resumed") ||
		strings.Contains(s, "connection lost") ||
		// The other uploads in flight when the connection goes may fail
		// with this instead, and whichever fails first is the error a
		// backup returns.
		strings.Contains(s, "client connection force closed") ||
		strings.Contains(s, "unexpected EOF") ||
		strings.Contains(s, "writer '") && strings.Contains(s, "not registered")
}

// Reconnectable says whether a machine backup that failed with err is worth
// starting again: the connection or session was lost, the server could not
// be reached, or PBS still held the backup group's lock from the lost
// session. Anything else (a snapshot or disk error, a refused token) fails at
// once.
func Reconnectable(err error) bool {
	if SessionLost(err) || DefaultRetryable(err) {
		return true
	}
	s := strings.ToLower(err.Error())
	// Not plain "lock": "block" contains it, and a disk read error must not retry.
	for _, m := range []string{"no route to host", "network is unreachable", "dial tcp", "locked", "acquire lock", "already running"} {
		if strings.Contains(s, m) {
			return true
		}
	}
	return false
}

// ReconnectWaits are the waits before each new attempt at a machine backup
// whose connection to PBS was lost: four more attempts over about half an
// hour. A run on a mobile link lost its session part way (2026-10-24) and a
// manual retry 13 minutes later went through.
var ReconnectWaits = []time.Duration{2 * time.Minute, 5 * time.Minute, 10 * time.Minute, 15 * time.Minute}

// A folder backup whose session was lost runs again once, from scratch,
// after FolderRerunWait, which lets PBS release the backup group's lock.
// Chunk deduplication makes the second run cheap: only the chunks new since
// the lost one are sent again.
const (
	FolderRerunAttempts = 2
	FolderRerunWait     = 25 * time.Minute
)
