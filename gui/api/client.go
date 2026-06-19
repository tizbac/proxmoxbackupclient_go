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

// NewClient creates a new API client
func NewClient(tokenPath string) *Client {
	return &Client{
		baseURL: DefaultServiceURL,
		httpClient: &http.Client{
			Timeout:   RequestTimeout,
			Transport: &tokenTransport{tokenPath: tokenPath, base: http.DefaultTransport},
		},
	}
}

// IsServiceAvailable checks if the local service is running
func (c *Client) IsServiceAvailable() bool {
	client := &http.Client{
		Timeout:   ConnectionTimeout,
		Transport: c.httpClient.Transport, // reuse the token-injecting transport
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
