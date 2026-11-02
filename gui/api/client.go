package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
	probe      http.RoundTripper // ProbeStatus's: the token, never the 401 hook
	public     http.RoundTripper // PublicStatus's: no token, no 401 hook
	answer     answerState       // whether the service answers (answer.go)
}

// NewClient creates a new API client. Its requests carry the token
// (tokenTransport) and, on a 401, get a fresh one through the unauthorized
// hook and are sent once more (reauthTransport). Each new connection to the
// service's address is checked first: it must be the background service
// that answers (serviceTransport, peer.go). Every request also records
// whether anything answered (answerTransport).
func NewClient(tokenPath string) *Client {
	c := &Client{baseURL: DefaultServiceURL}
	token := &tokenTransport{tokenPath: tokenPath, base: serviceTransport}
	c.probe = &answerTransport{c: c, base: token}
	c.public = &answerTransport{c: c, base: serviceTransport}
	c.httpClient = &http.Client{
		Timeout: RequestTimeout,
		Transport: &answerTransport{c: c, base: &reauthTransport{
			base:      token,
			tokenPath: tokenPath,
		}},
	}
	return c
}

// NewClientAt is NewClient for a service listening at baseURL (tests).
func NewClientAt(baseURL, tokenPath string) *Client {
	c := NewClient(tokenPath)
	c.baseURL = baseURL
	return c
}

// IsServiceAvailable reports whether the local service is running and takes
// this app's token.
func (c *Client) IsServiceAvailable() bool {
	code, _ := c.ProbeStatus()
	return code == http.StatusOK
}

// ProbeStatus asks /status and returns 200 (running, token taken), 401
// (running, no token or a replaced one) or 0 (not reachable, or something
// other than the service answers: peer.go). It never runs the unauthorized
// hook, so probing never asks for elevation.
func (c *Client) ProbeStatus() (int, error) {
	client := &http.Client{Timeout: ConnectionTimeout, Transport: c.probe}
	resp, err := client.Get(c.baseURL + "/status")
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode, nil
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

// GetRuns lists the service's runs: in progress and recently finished.
func (c *Client) GetRuns() (*RunsResponse, error) {
	resp, err := c.httpClient.Get(c.baseURL + "/backup/runs")
	if err != nil {
		return nil, fmt.Errorf("failed to get runs: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("service returned error: %d", resp.StatusCode)
	}
	var runs RunsResponse
	if err := json.NewDecoder(resp.Body).Decode(&runs); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &runs, nil
}

// ControlRun asks the service to "pause", "resume" or "stop" its backup
// jobID ("" for every backup it is running), and returns that run's pause
// state afterwards.
func (c *Client) ControlRun(action, jobID string) (*RunControlResponse, error) {
	u := c.baseURL + "/backup/" + action
	if jobID != "" {
		u += "?job=" + url.QueryEscape(jobID)
	}
	resp, err := c.httpClient.Post(u, "application/json", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to reach the service: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		var e ErrorResponse
		if json.NewDecoder(resp.Body).Decode(&e) == nil && e.Error != "" {
			return nil, fmt.Errorf("%s", e.Error)
		}
		return nil, fmt.Errorf("service returned error: %d", resp.StatusCode)
	}
	var st RunControlResponse
	if err := json.NewDecoder(resp.Body).Decode(&st); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return &st, nil
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
