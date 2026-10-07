package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeHandler is a minimal BackupHandler: it records what the routes hand it,
// so the HTTP surface (the service half of the GUI↔service contract) can be
// tested without a running service or a PBS server.
type fakeHandler struct {
	jobs []map[string]interface{}

	saveErr   error
	updateErr error
	deleteErr error
	runErr    error

	savedJob   map[string]interface{}
	updatedJob map[string]interface{}
	deletedID  string
	ranID      string
}

func (f *fakeHandler) StartBackup(string, []string, []string, []string, string, bool, string) error {
	return nil
}
func (f *fakeHandler) GetConfigWithHostname() map[string]interface{} {
	return map[string]interface{}{"hostname": "testhost"}
}
func (f *fakeHandler) GetScheduledJobsForAPI() []map[string]interface{} {
	return f.jobs
}
func (f *fakeHandler) SaveScheduledJobFromMap(job map[string]interface{}) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.savedJob = job
	return nil
}
func (f *fakeHandler) UpdateScheduledJobFromMap(job map[string]interface{}) error {
	if f.updateErr != nil {
		return f.updateErr
	}
	f.updatedJob = job
	return nil
}
func (f *fakeHandler) DeleteScheduledJobFromMap(id string) error {
	if f.deleteErr != nil {
		return f.deleteErr
	}
	f.deletedID = id
	return nil
}
func (f *fakeHandler) RunScheduledJobForAPI(id string) error {
	if f.runErr != nil {
		return f.runErr
	}
	f.ranID = id
	return nil
}
func (f *fakeHandler) PinServerFingerprint(string, string) error { return nil }
func (f *fakeHandler) StartMachineBackup(string, []string, string, bool, string) error {
	return nil
}
func (f *fakeHandler) GetFullConfigForAPI() map[string]interface{} {
	return map[string]interface{}{"pbs_servers": map[string]interface{}{}}
}
func (f *fakeHandler) SaveFullConfigFromAPI(map[string]interface{}) error       { return nil }
func (f *fakeHandler) TestPBSServerForAPI(string, map[string]interface{}) error { return nil }
func (f *fakeHandler) MintPBSTicketForAPI(string) (*PBSTicket, error) {
	return &PBSTicket{}, nil
}
func (f *fakeHandler) GetJobHistoryForAPI() ([]map[string]interface{}, error) {
	return []map[string]interface{}{}, nil
}

const testToken = "test-local-token"

// newTestServer starts the real routes+auth on a random port and returns the
// server, its URL and a token-carrying Client for it.
func newTestServer(t *testing.T, h BackupHandler) (*Server, string, *Client) {
	t.Helper()
	s := NewServer("127.0.0.1:0", h, testToken, "test")
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)

	// A token file rather than TokenOverride (a package global): each test
	// passes its own to the client, so nothing leaks between tests.
	tokenPath := filepath.Join(t.TempDir(), "api-token")
	if err := os.WriteFile(tokenPath, []byte(testToken+"\n"), 0600); err != nil {
		t.Fatalf("write token: %v", err)
	}
	c := NewClient(tokenPath)
	c.baseURL = ts.URL
	return s, ts.URL, c
}

// doAuthed sends the shared local-API token, like every real client does.
func doAuthed(t *testing.T, method, url, body string) *http.Response {
	t.Helper()
	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set(tokenHeader, testToken)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request %s %s: %v", method, url, err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

// Every route is reachable only with the shared token: the service is
// privileged, and a browser-originated request must be refused outright.
func TestHandlerRequiresLocalToken(t *testing.T) {
	_, url, _ := newTestServer(t, &fakeHandler{})

	tests := []struct {
		name   string
		mutate func(*http.Request)
		want   int
	}{
		{"no token", func(r *http.Request) {}, http.StatusUnauthorized},
		{"wrong token", func(r *http.Request) { r.Header.Set(tokenHeader, "nope") }, http.StatusUnauthorized},
		{"browser origin", func(r *http.Request) {
			r.Header.Set(tokenHeader, testToken)
			r.Header.Set("Origin", "https://evil.example")
		}, http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, url+"/jobs/full", nil)
			if err != nil {
				t.Fatalf("new request: %v", err)
			}
			tt.mutate(req)
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("request: %v", err)
			}
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != tt.want {
				t.Errorf("status = %d, want %d", resp.StatusCode, tt.want)
			}
		})
	}

	// Sanity: with the token the same route answers 200.
	if got := doAuthed(t, http.MethodGet, url+"/jobs/full", "").StatusCode; got != http.StatusOK {
		t.Errorf("authenticated status = %d, want %d", got, http.StatusOK)
	}
}

// POST /jobs/{id}/run starts a job and reports the id to the handler.
func TestJobRunRoute(t *testing.T) {
	h := &fakeHandler{}
	_, url, _ := newTestServer(t, h)

	resp := doAuthed(t, http.MethodPost, url+"/jobs/run/job-42", "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if h.ranID != "job-42" {
		t.Errorf("handler got id %q, want job-42", h.ranID)
	}

	var out map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if out["success"] != true {
		t.Errorf("response = %v, want success", out)
	}
}

// A job that cannot be started (unknown id, already running) is a conflict, and
// the service's message is what the caller sees.
func TestJobRunReportsFailure(t *testing.T) {
	h := &fakeHandler{runErr: fmt.Errorf("job \"Nightly\" est deja en cours d'execution")}
	_, url, _ := newTestServer(t, h)

	resp := doAuthed(t, http.MethodPost, url+"/jobs/run/job-42", "")
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d, want 409", resp.StatusCode)
	}
	var errResp ErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if !strings.Contains(errResp.Error, "deja en cours") {
		t.Errorf("error = %q, want the service's message", errResp.Error)
	}
	if h.ranID != "" {
		t.Errorf("handler started a job it refused: %q", h.ranID)
	}
}

func TestJobRunValidatesMethodAndID(t *testing.T) {
	_, url, _ := newTestServer(t, &fakeHandler{})

	if got := doAuthed(t, http.MethodGet, url+"/jobs/run/job-42", "").StatusCode; got != http.StatusMethodNotAllowed {
		t.Errorf("GET status = %d, want 405", got)
	}
	if got := doAuthed(t, http.MethodPost, url+"/jobs/run/", "").StatusCode; got != http.StatusBadRequest {
		t.Errorf("missing id status = %d, want 400", got)
	}
}

// The full job list must carry every scheduling parameter — the fields the
// GUI's scheduler UI reads and the backup needs.
func TestJobsFullKeepsEveryJobField(t *testing.T) {
	h := &fakeHandler{jobs: []map[string]interface{}{{
		"id":           "job-1",
		"name":         "Nightly",
		"scheduleTime": "02:30",
		"runAtStartup": true,
		"backupDirs":   []interface{}{"/data"},
		"driveLetters": []interface{}{"C:"},
		"backupId":     "host-01",
		"useVSS":       true,
		"backupType":   "machine",
		"excludeList":  []interface{}{"*.tmp"},
		"compression":  "better",
		"lastRun":      "2026-01-01T00:00:00Z",
		"nextRun":      "2026-01-02T02:30:00Z",
		"enabled":      true,
		// Legacy aliases (older GUI builds read those).
		"backup_type": "machine",
		"schedule":    "02:30",
		"last_run":    "2026-01-01T00:00:00Z",
		"next_run":    "2026-01-02T02:30:00Z",
	}}}
	_, url, client := newTestServer(t, h)

	apiJobs, err := client.GetScheduledJobs()
	if err != nil {
		t.Fatalf("GetScheduledJobs() error = %v", err)
	}
	if len(apiJobs) != 1 {
		t.Fatalf("got %d jobs, want 1", len(apiJobs))
	}
	job := apiJobs[0]

	for _, key := range []string{"scheduleTime", "runAtStartup", "driveLetters", "compression", "backupDirs"} {
		if _, ok := job[key]; !ok {
			t.Errorf("job map is missing %q", key)
		}
	}
	if job["compression"] != "better" {
		t.Errorf("compression = %v, want better", job["compression"])
	}
	if job["runAtStartup"] != true {
		t.Errorf("runAtStartup = %v, want true", job["runAtStartup"])
	}

	// The summary endpoint keeps its documented shape.
	resp := doAuthed(t, http.MethodGet, url+"/jobs", "")
	var summary JobsResponse
	if err := json.NewDecoder(resp.Body).Decode(&summary); err != nil {
		t.Fatalf("decode /jobs: %v", err)
	}
	if len(summary.Jobs) != 1 {
		t.Fatalf("got %d summary jobs, want 1", len(summary.Jobs))
	}
	if summary.Jobs[0].BackupType != "machine" || summary.Jobs[0].Schedule != "02:30" {
		t.Errorf("summary = %+v, want backup_type/machine + schedule/02:30", summary.Jobs[0])
	}
}

// A job POSTed to /jobs/create must arrive at the service with all of its
// parameters (the route is a pure pass-through of the GUI's map).
func TestJobCreateForwardsEveryField(t *testing.T) {
	h := &fakeHandler{}
	_, _, client := newTestServer(t, h)

	job := map[string]interface{}{
		"id":           "job-1",
		"name":         "Nightly",
		"scheduleTime": "02:30",
		"runAtStartup": true,
		"backupDirs":   []string{"/data"},
		"driveLetters": []string{"C:"},
		"backupId":     "host-01",
		"useVSS":       true,
		"backupType":   "machine",
		"excludeList":  []string{"*.tmp"},
		"compression":  "better",
		"enabled":      true,
	}
	if err := client.SaveScheduledJob(job); err != nil {
		t.Fatalf("SaveScheduledJob() error = %v", err)
	}
	if h.savedJob == nil {
		t.Fatalf("the service never received the job")
	}
	for _, key := range []string{"scheduleTime", "runAtStartup", "driveLetters", "compression", "backupDirs", "backupId", "backupType"} {
		if _, ok := h.savedJob[key]; !ok {
			t.Errorf("job map lost %q on the way to the service", key)
		}
	}

	if err := client.UpdateScheduledJob(job); err != nil {
		t.Fatalf("UpdateScheduledJob() error = %v", err)
	}
	if h.updatedJob == nil {
		t.Fatalf("the service never received the update")
	}
	if err := client.DeleteScheduledJob("job-1"); err != nil {
		t.Fatalf("DeleteScheduledJob() error = %v", err)
	}
	if h.deletedID != "job-1" {
		t.Errorf("deleted id = %q, want job-1", h.deletedID)
	}
}

// The GUI's client reports the service's own message when a run is refused.
func TestClientRunScheduledJobSurfacesServiceError(t *testing.T) {
	h := &fakeHandler{runErr: fmt.Errorf("job planifie \"nope\" introuvable")}
	_, _, client := newTestServer(t, h)

	err := client.RunScheduledJob("nope")
	if err == nil {
		t.Fatalf("RunScheduledJob() succeeded for an unknown job")
	}
	if !strings.Contains(err.Error(), "introuvable") {
		t.Errorf("error = %q, want the service's message", err)
	}
}

// The client must not hang forever on a service that never answers.
func TestClientRunScheduledJobTimeout(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		time.Sleep(500 * time.Millisecond)
	}))
	t.Cleanup(ts.Close)

	tokenPath := filepath.Join(t.TempDir(), "api-token")
	if err := os.WriteFile(tokenPath, []byte(testToken), 0600); err != nil {
		t.Fatalf("write token: %v", err)
	}
	c := NewClient(tokenPath)
	c.baseURL = ts.URL
	c.httpClient.Timeout = 50 * time.Millisecond

	if err := c.RunScheduledJob("job-1"); err == nil {
		t.Fatalf("RunScheduledJob() succeeded against an unresponsive service")
	}
}
