package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// ErrUnauthorized is a request the service refused for want of a valid
// token: the app is not connected (no elevation yet, or it was declined).
var ErrUnauthorized = errors.New("not connected to the background service")

// call sends one request with in as its JSON body (nil for none) and decodes
// the answer into out (nil to ignore it). The service's error message is
// returned as the error.
func (c *Client) call(method, path string, in, out interface{}) error {
	return c.callWith(c.httpClient, method, path, in, out)
}

// callWith is call through the HTTP client hc.
func (c *Client) callWith(hc *http.Client, method, path string, in, out interface{}) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return fmt.Errorf("encode request: %w", err)
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.baseURL+path, body)
	if err != nil {
		return err
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := hc.Do(req)
	if err != nil {
		return fmt.Errorf("the background service did not answer: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read answer: %w", err)
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return ErrUnauthorized
	}
	if resp.StatusCode != http.StatusOK {
		var e ErrorResponse
		if json.Unmarshal(raw, &e) == nil && e.Error != "" {
			return errors.New(e.Error)
		}
		return fmt.Errorf("the background service answered %d", resp.StatusCode)
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("decode answer: %w", err)
	}
	return nil
}

// GetFullConfig returns the settings document (no secrets).
func (c *Client) GetFullConfig() (map[string]interface{}, error) {
	var doc map[string]interface{}
	err := c.call(http.MethodGet, "/config", nil, &doc)
	return doc, err
}

// Call runs the app method name in the service with args, decoding its
// result into out (nil to ignore it).
func (c *Client) Call(name string, out interface{}, args ...interface{}) error {
	if args == nil {
		args = []interface{}{}
	}
	return c.call(http.MethodPost, "/call/"+url.PathEscape(name), args, out)
}

// PBSAccess returns what one PBS operation on server id needs.
func (c *Client) PBSAccess(id string, withKey bool) (*PBSAccess, error) {
	var acc PBSAccess
	if err := c.call(http.MethodPost, "/pbs/access", PBSAccessRequest{ID: id, WithKey: withKey}, &acc); err != nil {
		return nil, err
	}
	return &acc, nil
}

// GetJobHistory returns the run history.
func (c *Client) GetJobHistory() ([]map[string]interface{}, error) {
	var h []map[string]interface{}
	err := c.call(http.MethodGet, "/history", nil, &h)
	return h, err
}

// AppendJobHistory adds an entry the app made (a restore, an undelete).
func (c *Client) AppendJobHistory(entry interface{}) error {
	return c.call(http.MethodPost, "/history/append", entry, nil)
}

// GetMessageLog returns the Message Log, newest first.
func (c *Client) GetMessageLog() ([]map[string]interface{}, error) {
	var m []map[string]interface{}
	err := c.call(http.MethodGet, "/messages", nil, &m)
	return m, err
}

// AppendMessage adds an entry to the Message Log.
func (c *Client) AppendMessage(entry interface{}) error {
	return c.call(http.MethodPost, "/messages", entry, nil)
}

// GetScheduledJobsFull lists the Backup Sets with every field.
func (c *Client) GetScheduledJobsFull() ([]map[string]interface{}, error) {
	var jobs []map[string]interface{}
	err := c.call(http.MethodGet, "/jobs/full", nil, &jobs)
	return jobs, err
}

// SetJobEnabled turns a Backup Set on or off.
func (c *Client) SetJobEnabled(id string, enabled bool) error {
	return c.call(http.MethodPost, "/jobs/enabled", JobEnabledRequest{ID: id, Enabled: enabled}, nil)
}

// RunJob runs a Backup Set now, in the service.
func (c *Client) RunJob(id string) error {
	return c.call(http.MethodPost, "/jobs/run/"+url.PathEscape(id), nil, nil)
}

// logClient is the HTTP client of the four log requests. The service answers
// them without the token (public_logs.go), so they never run the unauthorized
// hook: reading a log must not bring up an elevation prompt. The token goes
// along when the app holds one, as with ProbeStatus. A service older than
// that answers 401 to an app without the token: ErrUnauthorized.
func (c *Client) logClient(timeout time.Duration) *http.Client {
	return &http.Client{Timeout: timeout, Transport: c.probe}
}

// ListLogs lists the service's logs (the app's []LogFileInfo).
func (c *Client) ListLogs(out interface{}) error {
	return c.callWith(c.logClient(RequestTimeout), http.MethodGet, logsPath, nil, out)
}

// ReadLog reads part of a log (query: the app's LogQuery; out: its LogView).
func (c *Client) ReadLog(query, out interface{}) error {
	return c.callWith(c.logClient(RequestTimeout), http.MethodPost, logsReadPath, query, out)
}

// FindRunLog finds the log of backupID's run at at (Unix ms).
func (c *Client) FindRunLog(backupID string, at int64, out interface{}) error {
	q := url.Values{"backup_id": {backupID}, "at": {strconv.FormatInt(at, 10)}}
	return c.callWith(c.logClient(RequestTimeout), http.MethodGet, logsRunPath+"?"+q.Encode(), nil, out)
}

// LogBundle returns the service's logs of the last days (0: its default) as
// a zip. It gets as long as the service gives itself to answer, and a little
// more for a request that waits behind another log request.
func (c *Client) LogBundle(days int) ([]byte, error) {
	var resp LogBundleResponse
	err := c.callWith(c.logClient(logAnswerTimeout+RequestTimeout), http.MethodPost, logsBundlePath, LogBundleRequest{Days: days}, &resp)
	return resp.Data, err
}

// ExportSettings returns a settings export file's contents.
func (c *Client) ExportSettings(includeSecrets bool) ([]byte, error) {
	var resp SettingsExportResponse
	err := c.call(http.MethodPost, "/settings/export", SettingsExportRequest{IncludeSecrets: includeSecrets}, &resp)
	return resp.Data, err
}

// ImportSettings merges an export file and returns the summary.
func (c *Client) ImportSettings(data []byte) (string, error) {
	var resp SettingsImportResponse
	err := c.call(http.MethodPost, "/settings/import", SettingsImportRequest{Data: data}, &resp)
	return resp.Summary, err
}

// SaveKey stores a key file in the service's keys folder and returns its path.
func (c *Client) SaveKey(name string, data []byte) (string, error) {
	var resp KeySaveResponse
	err := c.call(http.MethodPost, "/keys/save", KeySaveRequest{Name: name, Data: data}, &resp)
	return resp.Path, err
}
