package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// The routes below let an app without elevation work on the service's
// settings, which only the service can read and write (upstream's strict
// model). Each is served when the service's BackupHandler has the matching
// interface, and answers 501 otherwise.

// ConfigAPI reads the settings document, with no secrets in it (*_set
// markers instead). Changes go through CallAPI, one setting at a time.
type ConfigAPI interface {
	GetFullConfigForAPI() map[string]interface{}
}

// CallAPI runs one of the app's own methods in the service (POST
// /call/{name}): the settings setters (a server, the e-mail settings,
// preferences), Backup Set changes and connection tests. The service runs
// the same code the app runs without a service, with its merge rules (an
// empty secret keeps the stored one) and checks, rather than a second
// implementation of them. args is the JSON array of the method's arguments;
// the answer is its result as JSON. Only names on the service's own list
// are run.
type CallAPI interface {
	CallForAPI(name string, args json.RawMessage) (json.RawMessage, error)
}

// PBSAPI gives the app what one PBS operation needs.
type PBSAPI interface {
	PBSAccessForAPI(req PBSAccessRequest) (*PBSAccess, error)
}

// HistoryAPI reads the run history and adds the app's own entries (restore,
// undelete, roll back).
type HistoryAPI interface {
	GetJobHistoryForAPI() ([]map[string]interface{}, error)
	AppendJobHistoryFromAPI(entry map[string]interface{}) error
}

// MessageAPI reads the Message Log and adds the app's entries to it.
type MessageAPI interface {
	GetMessageLogForAPI() ([]map[string]interface{}, error)
	AppendMessageFromAPI(entry map[string]interface{}) error
}

// JobControlAPI turns Backup Sets on and off and runs one now, in the service.
type JobControlAPI interface {
	SetScheduledJobEnabledForAPI(id string, enabled bool) error
	RunScheduledJobForAPI(id string) error
	GetScheduledJobsFullForAPI() ([]map[string]interface{}, error)
}

// LogAPI shows the service's logs: the list, a part of one (the app's
// LogQuery and LogView, passed through as JSON), and the log of one run.
type LogAPI interface {
	ListLogsForAPI() (json.RawMessage, error)
	ReadLogForAPI(query json.RawMessage) (json.RawMessage, error)
	FindRunLogForAPI(backupID string, at int64) (json.RawMessage, error)
}

// LogBundleAPI packs the service's logs into one zip, for an app that is not
// allowed to read the service's folder and has to hand them to a person.
type LogBundleAPI interface {
	LogBundleForAPI(days int) ([]byte, error)
}

// SettingsAPI exports and imports the settings.
type SettingsAPI interface {
	ExportSettingsForAPI(includeSecrets bool) ([]byte, error)
	ImportSettingsFromAPI(data []byte) (string, error)
}

// KeyAPI stores an encryption key in the service's keys folder.
type KeyAPI interface {
	SaveKeyForAPI(name string, data []byte) (string, error)
}

// errNotServed is the 501 for a route this service has no handler for.
var errNotServed = errors.New("this service does not offer this")

func (s *Server) setupStateRoutes() {
	s.mux.HandleFunc("/config", s.handleConfig)
	s.mux.HandleFunc("/call/", s.handleCall)
	s.mux.HandleFunc("/pbs/access", s.handlePBSAccess)
	s.mux.HandleFunc("/history", s.handleHistory)
	s.mux.HandleFunc("/history/append", s.handleHistoryAppend)
	s.mux.HandleFunc("/messages", s.handleMessages)
	s.mux.HandleFunc("/jobs/full", s.handleJobsFull)
	s.mux.HandleFunc("/jobs/enabled", s.handleJobEnabled)
	s.mux.HandleFunc("/jobs/run/", s.handleJobRun)
	s.mux.HandleFunc(logsPath, s.handleLogs)
	s.mux.HandleFunc(logsReadPath, s.handleLogRead)
	s.mux.HandleFunc(logsRunPath, s.handleRunLog)
	s.mux.HandleFunc(logsBundlePath, s.handleLogBundle)
	s.mux.HandleFunc("/settings/export", s.handleSettingsExport)
	s.mux.HandleFunc("/settings/import", s.handleSettingsImport)
	s.mux.HandleFunc("/keys/save", s.handleKeySave)
}

// decodeBody reads a JSON request body into v, answering 400 when it cannot.
func (s *Server) decodeBody(w http.ResponseWriter, r *http.Request, v interface{}) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		s.writeError(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return false
	}
	return true
}

func (s *Server) method(w http.ResponseWriter, r *http.Request, methods ...string) bool {
	for _, m := range methods {
		if r.Method == m {
			return true
		}
	}
	s.writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
	return false
}

func (s *Server) notServed(w http.ResponseWriter) {
	s.writeError(w, errNotServed.Error(), http.StatusNotImplemented)
}

// handleConfig serves the settings document.
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	c, ok := s.app.(ConfigAPI)
	if !ok {
		s.notServed(w)
		return
	}
	if !s.method(w, r, http.MethodGet) {
		return
	}
	s.writeJSON(w, c.GetFullConfigForAPI(), http.StatusOK)
}

// ErrUnknownCall is what CallForAPI returns for a name not on its list
// (answered 404).
var ErrUnknownCall = errors.New("unknown call")

// handleCall runs one of the app's methods in the service: POST
// /call/{name} with the JSON array of its arguments.
func (s *Server) handleCall(w http.ResponseWriter, r *http.Request) {
	c, ok := s.app.(CallAPI)
	if !ok {
		s.notServed(w)
		return
	}
	if !s.method(w, r, http.MethodPost) {
		return
	}
	name := strings.Trim(strings.TrimPrefix(r.URL.Path, "/call/"), "/")
	var args json.RawMessage
	if !s.decodeBody(w, r, &args) {
		return
	}
	out, err := c.CallForAPI(name, args)
	if errors.Is(err, ErrUnknownCall) {
		s.writeError(w, err.Error()+": "+name, http.StatusNotFound)
		return
	}
	if err != nil {
		s.writeError(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.writeRaw(w, out)
}

func (s *Server) handlePBSAccess(w http.ResponseWriter, r *http.Request) {
	p, ok := s.app.(PBSAPI)
	if !ok {
		s.notServed(w)
		return
	}
	if !s.method(w, r, http.MethodPost) {
		return
	}
	var req PBSAccessRequest
	if !s.decodeBody(w, r, &req) {
		return
	}
	acc, err := p.PBSAccessForAPI(req)
	if err != nil {
		// 502: the request was fine, the server or its login was not.
		s.writeError(w, err.Error(), http.StatusBadGateway)
		return
	}
	s.writeJSON(w, acc, http.StatusOK)
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	h, ok := s.app.(HistoryAPI)
	if !ok {
		s.notServed(w)
		return
	}
	if !s.method(w, r, http.MethodGet) {
		return
	}
	hist, err := h.GetJobHistoryForAPI()
	if err != nil {
		s.writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if hist == nil {
		hist = []map[string]interface{}{}
	}
	s.writeJSON(w, hist, http.StatusOK)
}

func (s *Server) handleHistoryAppend(w http.ResponseWriter, r *http.Request) {
	h, ok := s.app.(HistoryAPI)
	if !ok {
		s.notServed(w)
		return
	}
	if !s.method(w, r, http.MethodPost) {
		return
	}
	var entry map[string]interface{}
	if !s.decodeBody(w, r, &entry) {
		return
	}
	if err := h.AppendJobHistoryFromAPI(entry); err != nil {
		s.writeError(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.writeJSON(w, map[string]interface{}{"success": true}, http.StatusOK)
}

// handleMessages lists the Message Log (GET, newest first) or adds an entry
// (POST).
func (s *Server) handleMessages(w http.ResponseWriter, r *http.Request) {
	m, ok := s.app.(MessageAPI)
	if !ok {
		s.notServed(w)
		return
	}
	switch r.Method {
	case http.MethodGet:
		entries, err := m.GetMessageLogForAPI()
		if err != nil {
			s.writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if entries == nil {
			entries = []map[string]interface{}{}
		}
		s.writeJSON(w, entries, http.StatusOK)
	case http.MethodPost:
		var entry map[string]interface{}
		if !s.decodeBody(w, r, &entry) {
			return
		}
		if err := m.AppendMessageFromAPI(entry); err != nil {
			s.writeError(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.writeJSON(w, map[string]interface{}{"success": true}, http.StatusOK)
	default:
		s.writeError(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleJobsFull lists the Backup Sets with every field (the app's own type).
func (s *Server) handleJobsFull(w http.ResponseWriter, r *http.Request) {
	j, ok := s.app.(JobControlAPI)
	if !ok {
		s.notServed(w)
		return
	}
	if !s.method(w, r, http.MethodGet) {
		return
	}
	jobs, err := j.GetScheduledJobsFullForAPI()
	if err != nil {
		s.writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if jobs == nil {
		jobs = []map[string]interface{}{}
	}
	s.writeJSON(w, jobs, http.StatusOK)
}

func (s *Server) handleJobEnabled(w http.ResponseWriter, r *http.Request) {
	j, ok := s.app.(JobControlAPI)
	if !ok {
		s.notServed(w)
		return
	}
	if !s.method(w, r, http.MethodPost) {
		return
	}
	var req JobEnabledRequest
	if !s.decodeBody(w, r, &req) {
		return
	}
	if err := j.SetScheduledJobEnabledForAPI(req.ID, req.Enabled); err != nil {
		s.writeError(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.writeJSON(w, map[string]interface{}{"success": true}, http.StatusOK)
}

// handleJobRun runs a Backup Set now, in the service (POST /jobs/run/{id}).
// It answers once the run has started; 409 when it cannot start (no such
// set, or it is running already).
func (s *Server) handleJobRun(w http.ResponseWriter, r *http.Request) {
	j, ok := s.app.(JobControlAPI)
	if !ok {
		s.notServed(w)
		return
	}
	if !s.method(w, r, http.MethodPost) {
		return
	}
	id := strings.Trim(strings.TrimPrefix(r.URL.Path, "/jobs/run/"), "/")
	if id == "" {
		s.writeError(w, "Job ID required", http.StatusBadRequest)
		return
	}
	if err := j.RunScheduledJobForAPI(id); err != nil {
		s.writeError(w, err.Error(), http.StatusConflict)
		return
	}
	s.writeJSON(w, map[string]interface{}{"success": true}, http.StatusOK)
}

func (s *Server) handleLogs(w http.ResponseWriter, r *http.Request) {
	l, ok := s.app.(LogAPI)
	if !ok {
		s.notServed(w)
		return
	}
	if !s.method(w, r, http.MethodGet) {
		return
	}
	s.oneLogRequest(w, func() {
		list, err := l.ListLogsForAPI()
		if err != nil {
			s.writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.writeRaw(w, list)
	})
}

func (s *Server) handleLogRead(w http.ResponseWriter, r *http.Request) {
	l, ok := s.app.(LogAPI)
	if !ok {
		s.notServed(w)
		return
	}
	if !s.method(w, r, http.MethodPost) {
		return
	}
	var q json.RawMessage
	if !s.decodeBody(w, r, &q) {
		return
	}
	s.oneLogRequest(w, func() {
		view, err := l.ReadLogForAPI(q)
		if err != nil {
			s.writeError(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.writeRaw(w, view)
	})
}

// handleRunLog finds the log of the run of backupID at time at (Unix ms):
// GET /logs/run?backup_id=...&at=...
func (s *Server) handleRunLog(w http.ResponseWriter, r *http.Request) {
	l, ok := s.app.(LogAPI)
	if !ok {
		s.notServed(w)
		return
	}
	if !s.method(w, r, http.MethodGet) {
		return
	}
	at, _ := strconv.ParseInt(r.URL.Query().Get("at"), 10, 64)
	s.oneLogRequest(w, func() {
		info, err := l.FindRunLogForAPI(r.URL.Query().Get("backup_id"), at)
		if err != nil {
			s.writeError(w, err.Error(), http.StatusNotFound)
			return
		}
		s.writeRaw(w, info)
	})
}

// handleLogBundle answers POST /logs/bundle {"days": n} with {"data": <zip>}.
func (s *Server) handleLogBundle(w http.ResponseWriter, r *http.Request) {
	b, ok := s.app.(LogBundleAPI)
	if !ok {
		s.notServed(w)
		return
	}
	if !s.method(w, r, http.MethodPost) {
		return
	}
	var req LogBundleRequest
	if !s.decodeBody(w, r, &req) {
		return
	}
	if req.Days < 0 || req.Days > 365 {
		s.writeError(w, "days must be between 0 and 365", http.StatusBadRequest)
		return
	}
	s.oneLogRequest(w, func() {
		data, err := b.LogBundleForAPI(req.Days)
		if err != nil {
			s.writeError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.writeJSON(w, LogBundleResponse{Data: data}, http.StatusOK)
	})
}

func (s *Server) handleSettingsExport(w http.ResponseWriter, r *http.Request) {
	st, ok := s.app.(SettingsAPI)
	if !ok {
		s.notServed(w)
		return
	}
	if !s.method(w, r, http.MethodPost) {
		return
	}
	var req SettingsExportRequest
	if !s.decodeBody(w, r, &req) {
		return
	}
	data, err := st.ExportSettingsForAPI(req.IncludeSecrets)
	if err != nil {
		s.writeError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.writeJSON(w, SettingsExportResponse{Data: data}, http.StatusOK)
}

func (s *Server) handleSettingsImport(w http.ResponseWriter, r *http.Request) {
	st, ok := s.app.(SettingsAPI)
	if !ok {
		s.notServed(w)
		return
	}
	if !s.method(w, r, http.MethodPost) {
		return
	}
	var req SettingsImportRequest
	if !s.decodeBody(w, r, &req) {
		return
	}
	summary, err := st.ImportSettingsFromAPI(req.Data)
	if err != nil {
		s.writeError(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.writeJSON(w, SettingsImportResponse{Summary: summary}, http.StatusOK)
}

func (s *Server) handleKeySave(w http.ResponseWriter, r *http.Request) {
	k, ok := s.app.(KeyAPI)
	if !ok {
		s.notServed(w)
		return
	}
	if !s.method(w, r, http.MethodPost) {
		return
	}
	var req KeySaveRequest
	if !s.decodeBody(w, r, &req) {
		return
	}
	path, err := k.SaveKeyForAPI(req.Name, req.Data)
	if err != nil {
		s.writeError(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.writeJSON(w, KeySaveResponse{Path: path}, http.StatusOK)
}

// writeRaw answers with JSON the handler produced itself ("null" as an
// empty object).
func (s *Server) writeRaw(w http.ResponseWriter, raw json.RawMessage) {
	if len(raw) == 0 {
		raw = json.RawMessage("null")
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(raw)
}
