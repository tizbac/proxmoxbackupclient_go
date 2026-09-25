package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// SettingsBundle is the portable export format for moving PBS servers, the
// theme and Backup Sets to a new machine, instead of re-entering everything
// by hand. BundleVersion lets a future format change detect an old file.
type SettingsBundle struct {
	BundleVersion int            `json:"bundleVersion"`
	ExportedAt    string         `json:"exportedAt"`
	AppVersion    string         `json:"appVersion"`
	Config        *Config        `json:"config,omitempty"`
	ScheduledJobs []ScheduledJob `json:"scheduledJobs,omitempty"`
}

const currentSettingsBundleVersion = 1

// ExportSettings writes a JSON bundle of the current PBS servers, theme and
// Backup Sets to a user-chosen file. includeSecrets controls whether PBS
// tokens/passwords and the SMTP password are written in plain text — the
// frontend defaults this to false, since the exported file itself has no
// other protection (no passphrase/encryption). Returns the chosen path, or
// "" if the save dialog was cancelled.
func (a *App) ExportSettings(includeSecrets bool) (string, error) {
	if a.isServiceProcess {
		return "", fmt.Errorf("export unavailable in service mode")
	}
	if a.config == nil {
		return "", fmt.Errorf("no configuration loaded")
	}

	cfgCopy := *a.config
	if a.config.PBSServers != nil {
		cfgCopy.PBSServers = make(map[string]*PBSServer, len(a.config.PBSServers))
		for id, srv := range a.config.PBSServers {
			srvCopy := *srv
			if !includeSecrets {
				srvCopy.Secret = ""
				srvCopy.Password = ""
			}
			cfgCopy.PBSServers[id] = &srvCopy
		}
	}
	if !includeSecrets {
		cfgCopy.Secret = ""
		cfgCopy.Password = ""
		cfgCopy.SMTPPassword = ""
	}
	// Never export a live session ticket — it's short-lived and per-machine,
	// re-minted from the stored credentials on the next operation anyway.
	cfgCopy.Ticket = ""
	cfgCopy.CSRFToken = ""

	jobs, err := a.GetScheduledJobs()
	if err != nil {
		return "", fmt.Errorf("failed to load scheduled jobs: %w", err)
	}

	bundle := SettingsBundle{
		BundleVersion: currentSettingsBundleVersion,
		ExportedAt:    time.Now().Format(time.RFC3339),
		AppVersion:    appVersion,
		Config:        &cfgCopy,
		ScheduledJobs: jobs,
	}

	data, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to build export: %w", err)
	}

	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Export settings",
		DefaultFilename: fmt.Sprintf("pbsgo-settings-%s.json", time.Now().Format("2006-01-02")),
		Filters: []runtime.FileFilter{
			{DisplayName: "JSON files (*.json)", Pattern: "*.json"},
		},
	})
	if err != nil {
		return "", fmt.Errorf("save dialog failed: %w", err)
	}
	if path == "" {
		return "", nil // cancelled
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return "", fmt.Errorf("failed to write export file: %w", err)
	}

	writeDebugLog(fmt.Sprintf("ExportSettings: wrote %d PBS server(s), %d job(s) to %s (secrets included: %v)",
		len(cfgCopy.PBSServers), len(jobs), path, includeSecrets))
	return path, nil
}

// ImportSettings reads a bundle written by ExportSettings and merges it into
// the current configuration. PBS servers and Backup Sets are upserted by ID
// (an existing entry with the same ID is overwritten, everything else is left
// alone) — never a wholesale replace, so importing a partial export can't
// wipe out unrelated local servers or jobs. A server whose secret was
// excluded at export time (both Secret and Password empty) keeps whatever
// secret is already configured locally for that same server ID, the same
// "empty means keep the current one" rule SaveConfig already applies.
// Returns the chosen path, or "" if the open dialog was cancelled.
func (a *App) ImportSettings() (string, error) {
	if a.isServiceProcess {
		return "", fmt.Errorf("import unavailable in service mode")
	}

	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Import settings",
		Filters: []runtime.FileFilter{
			{DisplayName: "JSON files (*.json)", Pattern: "*.json"},
		},
	})
	if err != nil {
		return "", fmt.Errorf("open dialog failed: %w", err)
	}
	if path == "" {
		return "", nil // cancelled
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read import file: %w", err)
	}

	var bundle SettingsBundle
	if err := json.Unmarshal(stripUTF8BOM(raw), &bundle); err != nil {
		return "", fmt.Errorf("not a valid settings export: %w", err)
	}
	if bundle.Config == nil {
		return "", fmt.Errorf("settings file has no configuration section")
	}

	if a.config == nil {
		a.config = LoadConfig()
	}
	if a.config.PBSServers == nil {
		a.config.PBSServers = make(map[string]*PBSServer)
	}

	serversImported := 0
	for id, srv := range bundle.Config.PBSServers {
		srvCopy := *srv
		if srvCopy.Secret == "" && srvCopy.Password == "" {
			if existing, ok := a.config.PBSServers[id]; ok {
				srvCopy.Secret = existing.Secret
				srvCopy.Password = existing.Password
			}
		}
		a.config.PBSServers[id] = &srvCopy
		serversImported++
	}
	if bundle.Config.DefaultPBSID != "" {
		a.config.DefaultPBSID = bundle.Config.DefaultPBSID
	}
	if bundle.Config.Theme != nil {
		a.config.Theme = bundle.Config.Theme
	}
	if bundle.Config.SMTPHost != "" {
		a.config.SMTPHost = bundle.Config.SMTPHost
		a.config.SMTPPort = bundle.Config.SMTPPort
		a.config.SMTPUsername = bundle.Config.SMTPUsername
		if bundle.Config.SMTPPassword != "" {
			a.config.SMTPPassword = bundle.Config.SMTPPassword
		}
	}

	if err := a.config.Save(); err != nil {
		return "", fmt.Errorf("failed to save imported config: %w", err)
	}

	existingJobs, err := a.GetScheduledJobs()
	if err != nil {
		return "", fmt.Errorf("failed to load existing jobs: %w", err)
	}
	byID := make(map[string]int, len(existingJobs))
	for i, j := range existingJobs {
		byID[j.ID] = i
	}
	jobsImported := 0
	for _, job := range bundle.ScheduledJobs {
		job.NextRun = calculateNextRun(job)
		if i, ok := byID[job.ID]; ok {
			job.Enabled = existingJobs[i].Enabled
			existingJobs[i] = job
		} else {
			existingJobs = append(existingJobs, job)
		}
		jobsImported++
	}

	jobsPath, err := getScheduledJobsPath()
	if err != nil {
		return "", fmt.Errorf("failed to get jobs path: %w", err)
	}
	jobsData, err := json.MarshalIndent(existingJobs, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal jobs: %w", err)
	}
	if err := atomicWriteFile(jobsPath, jobsData, 0600); err != nil {
		return "", fmt.Errorf("failed to write jobs file: %w", err)
	}

	writeDebugLog(fmt.Sprintf("ImportSettings: merged %d PBS server(s), %d job(s) from %s", serversImported, jobsImported, path))
	return path, nil
}
