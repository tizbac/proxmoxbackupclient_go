package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// The service's public status: the one route that needs no token. The app
// holds the service's token only while its window is in use (it lets go of
// it after a while in the tray, gui/token_drop.go), and any program running
// as the same user could read it from the app's memory while it holds it.
// Without the token the tray still says what goes on: that the service runs,
// how many backups run and wait, and whether one failed. So the answer holds
// nothing else: no Backup Set names, no paths, no servers, no messages.
//
// The peer check (peer.go) applies to it as to every route: the app reads it
// only from the real service.

// PublicStatusPath is the route of the public status (GET only).
const PublicStatusPath = "/status/public"

// QueuedMessagePrefix starts the message of a run that waits for another to
// finish (the app's operation queue): it counts as waiting, not running.
const QueuedMessagePrefix = "Queued: "

// publicOutcomeCache is how long the service keeps the outcome of the last
// runs (PublicStatusAPI) before it reads it again: anyone may ask, and asking
// often must not make the service read its files each time.
const publicOutcomeCache = 5 * time.Second

// PublicStatus is the answer of GET /status/public. Every field is always
// present.
type PublicStatus struct {
	// Running is always true: the service answered.
	Running bool `json:"running"`
	// Version is the service's build version, as in GET /status.
	Version string `json:"version"`
	// BackupsRunning and BackupsWaiting count the service's backups in
	// progress and those queued behind another.
	BackupsRunning int `json:"backups_running"`
	BackupsWaiting int `json:"backups_waiting"`
	// LastRunFailed: the most recent run of at least one Backup Set failed.
	LastRunFailed bool `json:"last_run_failed"`
	// LastRunEnded is when the most recent backup ended (RFC 3339), "" when
	// none is recorded.
	LastRunEnded string `json:"last_run_ended"`
}

// PublicStatusAPI is optionally implemented by the BackupHandler: the outcome
// of the last runs, for GET /status/public. failed: the most recent run of at
// least one Backup Set failed; ended: when the most recent backup ended (RFC
// 3339, "" for none). Without it both stay at their zero values.
type PublicStatusAPI interface {
	LastRunOutcomeForAPI() (failed bool, ended string)
}

// isPublicRequest reports whether r is one of the two requests the token
// check lets through: GET /status/public and GET /servers/public
// (public_servers.go).
func isPublicRequest(r *http.Request) bool {
	return r.Method == http.MethodGet && (r.URL.Path == PublicStatusPath || r.URL.Path == PublicServersPath)
}

// handlePublicStatus serves GET /status/public.
func (s *Server) handlePublicStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	st := PublicStatus{Running: true, Version: s.version}
	s.progressMutex.RLock()
	for _, p := range s.backupProgress {
		if !p.Running || p.Complete {
			continue
		}
		if strings.HasPrefix(p.Message, QueuedMessagePrefix) {
			st.BackupsWaiting++
		} else {
			st.BackupsRunning++
		}
	}
	s.progressMutex.RUnlock()
	st.LastRunFailed, st.LastRunEnded = s.lastRunOutcome()
	s.writeJSON(w, st, http.StatusOK)
}

// lastRunOutcome asks the handler for the outcome of the last runs, at most
// once every publicOutcomeCache; callers in between get the kept answer.
func (s *Server) lastRunOutcome() (bool, string) {
	p, ok := s.app.(PublicStatusAPI)
	if !ok {
		return false, ""
	}
	s.publicMu.Lock()
	defer s.publicMu.Unlock()
	if s.publicAt.IsZero() || time.Since(s.publicAt) >= publicOutcomeCache {
		s.publicFailed, s.publicEnded = p.LastRunOutcomeForAPI()
		s.publicAt = time.Now()
	}
	return s.publicFailed, s.publicEnded
}

// PublicStatus reads GET /status/public. It sends no token: it works whether
// or not the app holds one, and never runs the unauthorized hook. Like every
// request, it records whether the service answers (answer.go), and the peer
// check applies. A service older than this route answers 401: an error.
func (c *Client) PublicStatus() (*PublicStatus, error) {
	client := &http.Client{Timeout: ConnectionTimeout, Transport: c.public}
	resp, err := client.Get(c.baseURL + PublicStatusPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("service returned error: %d", resp.StatusCode)
	}
	var st PublicStatus
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64<<10)).Decode(&st); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &st, nil
}
