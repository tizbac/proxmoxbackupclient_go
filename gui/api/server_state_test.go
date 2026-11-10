package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
)

// stateHandler is a service App that also holds settings, history and the
// Message Log, as the real service does.
type stateHandler struct {
	fakeHandler
	mu       sync.Mutex
	doc      map[string]interface{}
	history  []map[string]interface{}
	messages []map[string]interface{}
	enabled  map[string]bool
	ran      []string
	access   PBSAccessRequest
	keys     map[string][]byte
}

func (h *stateHandler) GetFullConfigForAPI() map[string]interface{} {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.doc
}

// CallForAPI knows SetDefaultPBSServer(id) and Add(a, b), as the service's
// list would.
func (h *stateHandler) CallForAPI(name string, args json.RawMessage) (json.RawMessage, error) {
	switch name {
	case "SetDefaultPBSServer":
		var a []string
		if err := json.Unmarshal(args, &a); err != nil || len(a) != 1 {
			return nil, errors.New("SetDefaultPBSServer takes one ID")
		}
		if a[0] == "refuse" {
			return nil, errors.New("settings refused")
		}
		h.mu.Lock()
		h.doc["default_pbs_id"] = a[0]
		h.mu.Unlock()
		return nil, nil
	case "Add":
		var a []int
		if err := json.Unmarshal(args, &a); err != nil || len(a) != 2 {
			return nil, errors.New("Add takes two numbers")
		}
		return json.Marshal(a[0] + a[1])
	}
	return nil, ErrUnknownCall
}

func (h *stateHandler) PBSAccessForAPI(req PBSAccessRequest) (*PBSAccess, error) {
	h.access = req
	if req.ID == "missing" {
		return nil, errors.New("PBS server 'missing' not found")
	}
	acc := &PBSAccess{BaseURL: "https://pbs:8007", Datastore: "ds", AuthID: "u@pbs!t", Secret: "s3cret"}
	if req.WithKey {
		acc.KeyFileData = []byte(`{"data":"x"}`)
	}
	return acc, nil
}

func (h *stateHandler) GetJobHistoryForAPI() ([]map[string]interface{}, error) {
	return h.history, nil
}

func (h *stateHandler) AppendJobHistoryFromAPI(e map[string]interface{}) error {
	h.history = append(h.history, e)
	return nil
}

func (h *stateHandler) GetMessageLogForAPI() ([]map[string]interface{}, error) {
	return h.messages, nil
}

func (h *stateHandler) AppendMessageFromAPI(e map[string]interface{}) error {
	h.messages = append(h.messages, e)
	return nil
}

func (h *stateHandler) SetScheduledJobEnabledForAPI(id string, enabled bool) error {
	if h.enabled == nil {
		h.enabled = map[string]bool{}
	}
	h.enabled[id] = enabled
	return nil
}

func (h *stateHandler) RunScheduledJobForAPI(id string) error {
	if id == "busy" {
		return errors.New("already running")
	}
	h.ran = append(h.ran, id)
	return nil
}

func (h *stateHandler) GetScheduledJobsFullForAPI() ([]map[string]interface{}, error) {
	return []map[string]interface{}{{"id": "a", "name": "Documents"}}, nil
}

func (h *stateHandler) ListLogsForAPI() (json.RawMessage, error) {
	return json.RawMessage(`[{"name":"service.log"}]`), nil
}

func (h *stateHandler) ReadLogForAPI(q json.RawMessage) (json.RawMessage, error) {
	return json.RawMessage(`{"query":` + string(q) + `}`), nil
}

func (h *stateHandler) FindRunLogForAPI(backupID string, at int64) (json.RawMessage, error) {
	if backupID == "" {
		return nil, errors.New("no such run")
	}
	b, _ := json.Marshal(map[string]interface{}{"name": backupID, "at": at})
	return b, nil
}

func (h *stateHandler) LogBundleForAPI(days int) ([]byte, error) {
	if days == 13 {
		return nil, errors.New("no logs to pack")
	}
	return []byte(fmt.Sprintf("zip-of-%d-days", days)), nil
}

func (h *stateHandler) ExportSettingsForAPI(includeSecrets bool) ([]byte, error) {
	if includeSecrets {
		return []byte("with-secrets"), nil
	}
	return []byte("plain"), nil
}

func (h *stateHandler) ImportSettingsFromAPI(data []byte) (string, error) {
	return "imported " + string(data), nil
}

func (h *stateHandler) SaveKeyForAPI(name string, data []byte) (string, error) {
	if h.keys == nil {
		h.keys = map[string][]byte{}
	}
	h.keys[name] = data
	return "/keys/" + name, nil
}

func newStateService(t *testing.T) (*stateHandler, *Client) {
	t.Helper()
	h := &stateHandler{doc: map[string]interface{}{"default_pbs_id": "pbs1"}}
	_, c := newTestServiceWith(t, h)
	return h, c
}

func TestConfigAndCall(t *testing.T) {
	h, c := newStateService(t)
	doc, err := c.GetFullConfig()
	if err != nil || doc["default_pbs_id"] != "pbs1" {
		t.Fatalf("GetFullConfig = %v, %v", doc, err)
	}
	if err := c.Call("SetDefaultPBSServer", nil, "pbs2"); err != nil || h.doc["default_pbs_id"] != "pbs2" {
		t.Fatalf("Call SetDefaultPBSServer: %v (held %v)", err, h.doc)
	}
	if err := c.Call("SetDefaultPBSServer", nil, "refuse"); err == nil || !strings.Contains(err.Error(), "settings refused") {
		t.Errorf("a refused call gave %v, want the service's reason", err)
	}
	var sum int
	if err := c.Call("Add", &sum, 2, 3); err != nil || sum != 5 {
		t.Errorf("Call Add = %d, %v", sum, err)
	}
	if err := c.Call("DeleteEverything", nil); err == nil || !strings.Contains(err.Error(), "unknown call") {
		t.Errorf("an unlisted name gave %v, want unknown call", err)
	}
}

func TestPBSAccess(t *testing.T) {
	h, c := newStateService(t)
	acc, err := c.PBSAccess("pbs1", true)
	if err != nil || acc.Secret != "s3cret" || string(acc.KeyFileData) != `{"data":"x"}` || !h.access.WithKey {
		t.Fatalf("PBSAccess = %+v, %v", acc, err)
	}
	if _, err := c.PBSAccess("missing", false); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("PBSAccess(missing) = %v", err)
	}
}

func TestHistoryAndMessages(t *testing.T) {
	_, c := newStateService(t)
	if err := c.AppendJobHistory(map[string]interface{}{"job_name": "Restore"}); err != nil {
		t.Fatal(err)
	}
	h, err := c.GetJobHistory()
	if err != nil || len(h) != 1 || h[0]["job_name"] != "Restore" {
		t.Fatalf("history %v, %v", h, err)
	}
	if err := c.AppendMessage(map[string]interface{}{"message": "hello"}); err != nil {
		t.Fatal(err)
	}
	m, err := c.GetMessageLog()
	if err != nil || len(m) != 1 || m[0]["message"] != "hello" {
		t.Fatalf("messages %v, %v", m, err)
	}
}

func TestJobControl(t *testing.T) {
	h, c := newStateService(t)
	if err := c.SetJobEnabled("a", false); err != nil || h.enabled["a"] {
		t.Fatalf("SetJobEnabled: %v, %v", err, h.enabled)
	}
	if err := c.RunJob("a"); err != nil || len(h.ran) != 1 {
		t.Fatalf("RunJob: %v, %v", err, h.ran)
	}
	if err := c.RunJob("busy"); err == nil || !strings.Contains(err.Error(), "already running") {
		t.Errorf("RunJob(busy) = %v", err)
	}
	jobs, err := c.GetScheduledJobsFull()
	if err != nil || len(jobs) != 1 || jobs[0]["name"] != "Documents" {
		t.Errorf("GetScheduledJobsFull = %v, %v", jobs, err)
	}
}

func TestLogBundle(t *testing.T) {
	_, c := newStateService(t)
	data, err := c.LogBundle(7)
	if err != nil || string(data) != "zip-of-7-days" {
		t.Fatalf("LogBundle(7) = %q, %v", data, err)
	}
	if data, err = c.LogBundle(0); err != nil || string(data) != "zip-of-0-days" {
		t.Errorf("LogBundle(0) = %q, %v", data, err)
	}
	if _, err = c.LogBundle(13); err == nil || !strings.Contains(err.Error(), "no logs to pack") {
		t.Errorf("a failure of the service was not passed on: %v", err)
	}
	for _, days := range []int{-1, 366, 100000} {
		if _, err = c.LogBundle(days); err == nil {
			t.Errorf("LogBundle(%d) was accepted", days)
		}
	}
}

func TestLogsSettingsKeys(t *testing.T) {
	h, c := newStateService(t)
	var list []map[string]interface{}
	if err := c.ListLogs(&list); err != nil || len(list) != 1 {
		t.Fatalf("ListLogs %v, %v", list, err)
	}
	var view map[string]interface{}
	if err := c.ReadLog(map[string]interface{}{"name": "service.log"}, &view); err != nil {
		t.Fatal(err)
	}
	if q, _ := view["query"].(map[string]interface{}); q["name"] != "service.log" {
		t.Errorf("ReadLog view %v", view)
	}
	var run map[string]interface{}
	if err := c.FindRunLog("pc-docs", 42, &run); err != nil || run["name"] != "pc-docs" || run["at"] != float64(42) {
		t.Errorf("FindRunLog %v, %v", run, err)
	}
	if err := c.FindRunLog("", 0, &run); err == nil {
		t.Error("FindRunLog with no run gave no error")
	}
	if data, err := c.ExportSettings(true); err != nil || string(data) != "with-secrets" {
		t.Errorf("ExportSettings %q, %v", data, err)
	}
	if s, err := c.ImportSettings([]byte("x")); err != nil || s != "imported x" {
		t.Errorf("ImportSettings %q, %v", s, err)
	}
	if p, err := c.SaveKey("k.json", []byte("{}")); err != nil || p != "/keys/k.json" || string(h.keys["k.json"]) != "{}" {
		t.Errorf("SaveKey %q, %v", p, err)
	}
}

// A service without these handlers answers 501, which the app reports.
func TestStateRoutesNotServed(t *testing.T) {
	_, c := newTestService(t, &fakeHandler{})
	if _, err := c.GetFullConfig(); err == nil || !strings.Contains(err.Error(), errNotServed.Error()) {
		t.Errorf("GetFullConfig on a plain service = %v, want not offered", err)
	}
}
