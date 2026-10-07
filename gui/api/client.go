package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	DefaultServiceURL = "http://127.0.0.1:18765"
	ConnectionTimeout = 5 * time.Second  // Increased for service startup
	RequestTimeout    = 30 * time.Second // Backup returns immediately, safe
)

// Client handles communication with the local service
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new API client. The transport chain injects the shared
// token and, when a 401 hook is installed, re-authenticates a request the
// service rejected because the token was rotated.
func NewClient(tokenPath string) *Client {
	return &Client{
		baseURL: DefaultServiceURL,
		httpClient: &http.Client{
			Timeout: RequestTimeout,
			Transport: &reauthTransport{
				base:      &tokenTransport{tokenPath: tokenPath, base: http.DefaultTransport},
				tokenPath: tokenPath,
			},
		},
	}
}

// IsServiceAvailable checks if the local service is running
func (c *Client) IsServiceAvailable() bool {
	client := &http.Client{
		Timeout:   ConnectionTimeout,
		Transport: c.httpClient.Transport, // token injection + rotation retry
	}

	resp, err := client.Get(c.baseURL + "/status")
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()

	return resp.StatusCode == http.StatusOK
}

// GetStatus retrieves the service status
func (c *Client) GetStatus() (*StatusResponse, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/status")
	if err != nil {
		return nil, fmt.Errorf("failed to connect to service: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("service returned error: %d", resp.StatusCode)
	}

	var status StatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &status, nil
}

// StartBackup sends a backup request to the service
func (c *Client) StartBackup(req *BackupRequest) (*BackupResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	resp, err := c.httpClient.Post(
		c.baseURL+"/backup",
		"application/json",
		bytes.NewBuffer(body),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to send backup request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if err := json.Unmarshal(respBody, &errResp); err == nil {
			return nil, fmt.Errorf("backup failed: %s", errResp.Error)
		}
		return nil, fmt.Errorf("backup failed with status %d", resp.StatusCode)
	}

	var backupResp BackupResponse
	if err := json.Unmarshal(respBody, &backupResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &backupResp, nil
}

// StartMachineBackup sends a machine backup request to the service
func (c *Client) StartMachineBackup(req *BackupRequest) (*BackupResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	resp, err := c.httpClient.Post(
		c.baseURL+"/backup/machine",
		"application/json",
		bytes.NewBuffer(body),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to send machine backup request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if err := json.Unmarshal(respBody, &errResp); err == nil {
			return nil, fmt.Errorf("machine backup failed: %s", errResp.Error)
		}
		return nil, fmt.Errorf("machine backup failed with status %d", resp.StatusCode)
	}

	var backupResp BackupResponse
	if err := json.Unmarshal(respBody, &backupResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &backupResp, nil
}

// GetBackupStatus retrieves the current status of a backup job
func (c *Client) GetBackupStatus(jobID string) (*BackupProgress, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/backup/status/" + jobID)
	if err != nil {
		return nil, fmt.Errorf("failed to get backup status: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("backup job not found")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("service returned error: %d", resp.StatusCode)
	}

	var progress BackupProgress
	if err := json.NewDecoder(resp.Body).Decode(&progress); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &progress, nil
}

// ListBackupJobs returns a list of all running/completed backup jobs
func (c *Client) ListBackupJobs() ([]*BackupProgress, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/backup/jobs")
	if err != nil {
		return nil, fmt.Errorf("failed to list backup jobs: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("service returned error: %d", resp.StatusCode)
	}

	var jobs []*BackupProgress
	if err := json.NewDecoder(resp.Body).Decode(&jobs); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return jobs, nil
}

// CancelBackup cancels a running backup job by ID
func (c *Client) CancelBackup(jobID string) error {
	resp, err := c.httpClient.Post(c.baseURL+"/backup/cancel/"+jobID, "application/json", nil)
	if err != nil {
		return fmt.Errorf("failed to cancel backup: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("backup job not found")
	}

	if resp.StatusCode == http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("cannot cancel backup: %s", string(body))
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("service returned error: %d", resp.StatusCode)
	}

	return nil
}

// GetJobs retrieves the list of configured jobs
func (c *Client) GetJobs() (*JobsResponse, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/jobs")
	if err != nil {
		return nil, fmt.Errorf("failed to get jobs: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("service returned error: %d", resp.StatusCode)
	}

	var jobs JobsResponse
	if err := json.NewDecoder(resp.Body).Decode(&jobs); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &jobs, nil
}

// CreateJob creates a new scheduled job
func (c *Client) CreateJob(job map[string]interface{}) error {
	body, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to encode job: %w", err)
	}

	resp, err := c.httpClient.Post(
		c.baseURL+"/jobs/create",
		"application/json",
		bytes.NewBuffer(body),
	)
	if err != nil {
		return fmt.Errorf("failed to create job: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create job: %s", string(respBody))
	}

	return nil
}

// PinFingerprint asks the service to persist a pinned certificate fingerprint for
// a PBS server. The service is the single privileged writer of config.json, so the
// unprivileged GUI delegates this write instead of failing to overwrite the file.
func (c *Client) PinFingerprint(id, fingerprint string) error {
	body, err := json.Marshal(map[string]string{"id": id, "fingerprint": fingerprint})
	if err != nil {
		return fmt.Errorf("failed to encode request: %w", err)
	}

	resp, err := c.httpClient.Post(
		c.baseURL+"/pbs/fingerprint",
		"application/json",
		bytes.NewBuffer(body),
	)
	if err != nil {
		return fmt.Errorf("failed to send fingerprint request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to pin fingerprint: %s", string(respBody))
	}

	return nil
}

// UpdateJob updates an existing scheduled job
func (c *Client) UpdateJob(job map[string]interface{}) error {
	body, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to encode job: %w", err)
	}

	resp, err := c.httpClient.Post(
		c.baseURL+"/jobs/update",
		"application/json",
		bytes.NewBuffer(body),
	)
	if err != nil {
		return fmt.Errorf("failed to update job: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update job: %s", string(respBody))
	}

	return nil
}

// DeleteJob deletes a scheduled job by ID
func (c *Client) DeleteJob(jobID string) error {
	req, err := http.NewRequest(http.MethodDelete, c.baseURL+"/jobs/delete/"+jobID, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete job: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete job: %s", string(respBody))
	}

	return nil
}

// ProbeStatus performs a GET /status and returns the raw HTTP status code
// without turning it into an error:
//   - 200: service is running AND the client's token is accepted
//   - 401: service is running but the token is missing or invalid
//   - 0:   service unreachable (connection refused / timeout)
//
// The GUI uses the 401 case to trigger an elevated one-time token fetch.
func (c *Client) ProbeStatus() (int, error) {
	client := &http.Client{
		Timeout:   ConnectionTimeout,
		Transport: c.httpClient.Transport, // token injection + rotation retry
	}
	resp, err := client.Get(c.baseURL + "/status")
	if err != nil {
		return 0, nil // unreachable: not an error for probing purposes
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.StatusCode, nil
}

// GetFullConfig fetches the sanitized full configuration document from the
// service (secrets are replaced by *_set markers; the GUI never sees them).
func (c *Client) GetFullConfig() (map[string]interface{}, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/config")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch config from service: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read config response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch config: %s", string(respBody))
	}

	var doc map[string]interface{}
	if err := json.Unmarshal(respBody, &doc); err != nil {
		return nil, fmt.Errorf("failed to decode config: %w", err)
	}
	return doc, nil
}

// SaveFullConfig pushes a whole sanitized configuration document to the service.
// Empty secret/password fields mean "keep the existing value" on the service
// side (the GUI never receives or resends real secrets).
func (c *Client) SaveFullConfig(doc map[string]interface{}) error {
	body, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}

	resp, err := c.httpClient.Post(
		c.baseURL+"/config",
		"application/json",
		bytes.NewBuffer(body),
	)
	if err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to save config: %s", string(respBody))
	}
	return nil
}

// TestPBSServer asks the service to test connectivity to a PBS server. The
// draft (non-empty fields only) is merged over the stored entry, so unsaved
// form edits can be tested without persisting them.
func (c *Client) TestPBSServer(id string, draft map[string]interface{}) error {
	body, err := json.Marshal(map[string]interface{}{"id": id, "draft": draft})
	if err != nil {
		return fmt.Errorf("failed to encode test request: %w", err)
	}

	resp, err := c.httpClient.Post(
		c.baseURL+"/pbs/test",
		"application/json",
		bytes.NewBuffer(body),
	)
	if err != nil {
		return fmt.Errorf("failed to test PBS server: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s", string(respBody))
	}
	return nil
}

// MintPBSTicket asks the service to mint a short-lived PBS session ticket for
// the given server (by ID, or the default server when empty). The GUI uses the
// returned ticket + non-sensitive connection parameters for restore/listing
// operations; it never handles PBS credentials.
func (c *Client) MintPBSTicket(id string) (*PBSTicket, error) {
	body, err := json.Marshal(map[string]string{"id": id})
	if err != nil {
		return nil, fmt.Errorf("failed to encode ticket request: %w", err)
	}

	resp, err := c.httpClient.Post(
		c.baseURL+"/pbs/ticket",
		"application/json",
		bytes.NewBuffer(body),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to mint PBS ticket: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read ticket response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to mint PBS ticket: %s", string(respBody))
	}

	var ticket PBSTicket
	if err := json.Unmarshal(respBody, &ticket); err != nil {
		return nil, fmt.Errorf("failed to decode ticket: %w", err)
	}
	return &ticket, nil
}

// GetJobHistory fetches the job history (backups executed by the service's
// scheduler) for the GUI's history view.
func (c *Client) GetJobHistory() ([]map[string]interface{}, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/history")
	if err != nil {
		return nil, fmt.Errorf("failed to get job history: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read history response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get job history: %s", string(respBody))
	}

	var history []map[string]interface{}
	if err := json.Unmarshal(respBody, &history); err != nil {
		return nil, fmt.Errorf("failed to decode history: %w", err)
	}
	return history, nil
}

// GetScheduledJobs retrieves all scheduled jobs with FULL data (as returned by
// the service's GetScheduledJobsForAPI). Returns []map for the caller to
// convert to its local ScheduledJob type (avoids circular import).
func (c *Client) GetScheduledJobs() ([]map[string]interface{}, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/jobs/full")
	if err != nil {
		return nil, fmt.Errorf("failed to get jobs: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("service returned error: %d", resp.StatusCode)
	}

	var apiJobs []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&apiJobs); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	if apiJobs == nil {
		return []map[string]interface{}{}, nil
	}
	return apiJobs, nil
}

// SaveScheduledJob creates a new scheduled job with full data (map form)
func (c *Client) SaveScheduledJob(job map[string]interface{}) error {
	return c.CreateJob(job)
}

// UpdateScheduledJob updates an existing scheduled job with full data (map form)
func (c *Client) UpdateScheduledJob(job map[string]interface{}) error {
	return c.UpdateJob(job)
}

// DeleteScheduledJob deletes a scheduled job by ID
func (c *Client) DeleteScheduledJob(jobID string) error {
	req, err := http.NewRequest(http.MethodDelete, c.baseURL+"/jobs/delete/"+jobID, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete job: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete job: %s", string(respBody))
	}

	return nil
}

// RunScheduledJob asks the service to start a stored scheduled job NOW,
// ignoring its schedule (manual "run now"). The service owns the jobs file
// and the history, so the run — and its bookkeeping — happen there.
func (c *Client) RunScheduledJob(jobID string) error {
	resp, err := c.httpClient.Post(c.baseURL+"/jobs/run/"+jobID, "application/json", nil)
	if err != nil {
		return fmt.Errorf("failed to run job: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		// Surface the service's message (unknown job / already running) as-is.
		var errResp ErrorResponse
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Error != "" {
			return fmt.Errorf("%s", errResp.Error)
		}
		return fmt.Errorf("failed to run job: %s", string(respBody))
	}

	return nil
}
