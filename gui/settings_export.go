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
	// SourceHost is the name of the computer the bundle was exported from.
	// A bundle deployed to many computers by the installer
	// (applySettingsFile) uses it to give each computer its own backup IDs.
	SourceHost string `json:"sourceHost,omitempty"`
}

const currentSettingsBundleVersion = 1

// ExportSettings writes a JSON bundle of the current PBS servers, theme and
// Backup Sets to a user-chosen file. includeSecrets controls whether PBS
// tokens/passwords, remembered key passphrases and the SMTP password are
// written in plain text (never sealed: the bundle is for another computer) — the
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
				srvCopy.EncryptionKeyPassphrase = ""
			}
			cfgCopy.PBSServers[id] = &srvCopy
		}
	}
	if !includeSecrets {
		cfgCopy.Secret = ""
		cfgCopy.Password = ""
		cfgCopy.SMTPPassword = ""
		cfgCopy.EncryptionKeyPassphrase = ""
	}
	// Never export a live session ticket — it's short-lived and per-machine,
	// re-minted from the stored credentials on the next operation anyway.
	cfgCopy.Ticket = ""
	cfgCopy.CSRFToken = ""

	jobs, err := a.GetScheduledJobs()
	if err != nil {
		return "", fmt.Errorf("failed to load scheduled jobs: %w", err)
	}

	host, _ := os.Hostname()
	bundle := SettingsBundle{
		BundleVersion: currentSettingsBundleVersion,
		ExportedAt:    time.Now().Format(time.RFC3339),
		AppVersion:    appVersion,
		Config:        &cfgCopy,
		ScheduledJobs: jobs,
		SourceHost:    host,
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

	bundle, err := parseSettingsBundle(raw)
	if err != nil {
		return "", err
	}

	// Importing Backup Sets changes the job list, so it is subject to the same
	// administrator requirement as adding or editing a set by hand.
	if len(bundle.ScheduledJobs) > 0 {
		if err := a.checkJobsChangeAllowed(); err != nil {
			return "", err
		}
	}

	serversImported, jobsImported, err := a.mergeSettingsBundle(bundle, false)
	if err != nil {
		return "", err
	}
	writeDebugLog(fmt.Sprintf("ImportSettings: merged %d PBS server(s), %d job(s) from %s", serversImported, jobsImported, path))
	return path, nil
}

// parseSettingsBundle reads a file written by ExportSettings.
func parseSettingsBundle(raw []byte) (SettingsBundle, error) {
	var bundle SettingsBundle
	if err := json.Unmarshal(stripUTF8BOM(raw), &bundle); err != nil {
		return bundle, fmt.Errorf("not a valid settings export: %w", err)
	}
	if bundle.Config == nil {
		return bundle, fmt.Errorf("settings file has no configuration section")
	}
	for id, srv := range bundle.Config.PBSServers {
		if srv == nil {
			return bundle, fmt.Errorf("settings file: PBS server %q is empty", id)
		}
	}
	return bundle, nil
}

// mergeSettingsBundle merges a bundle into the current configuration, as
// described at ImportSettings. forDeployment is one bundle set up on many
// computers by the installer (applySettingsFile). There:
//   - a backup ID made from the exporting computer's name is made from this
//     computer's (deployedBackupID), so the computers do not all back up into
//     the same groups;
//   - the bundle's default server only becomes this computer's default when
//     it has none (sets without a server use the default);
//   - the sets' run state (last run, full-read count) is the exporting
//     computer's, so it is cleared.
func (a *App) mergeSettingsBundle(bundle SettingsBundle, forDeployment bool) (serversImported, jobsImported int, err error) {
	if a.config == nil {
		a.config = LoadConfig()
	}
	if a.config.PBSServers == nil {
		a.config.PBSServers = make(map[string]*PBSServer)
	}

	for id, srv := range bundle.Config.PBSServers {
		srvCopy := *srv
		if existing, ok := a.config.PBSServers[id]; ok && existing != nil {
			if srvCopy.Secret == "" && srvCopy.Password == "" {
				srvCopy.Secret = existing.Secret
				srvCopy.Password = existing.Password
			}
			// A file exported without passwords has no key passphrase either:
			// keep the one this computer remembers.
			if srvCopy.EncryptionKeyPassphrase == "" {
				srvCopy.EncryptionKeyPassphrase = existing.EncryptionKeyPassphrase
			}
		}
		a.config.PBSServers[id] = &srvCopy
		serversImported++
	}
	if bundle.Config.DefaultPBSID != "" && (!forDeployment || a.config.DefaultPBSID == "") {
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
		return 0, 0, fmt.Errorf("failed to save imported config: %w", err)
	}

	existingJobs, err := a.GetScheduledJobs()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to load existing jobs: %w", err)
	}
	byID := make(map[string]int, len(existingJobs))
	for i, j := range existingJobs {
		byID[j.ID] = i
	}
	thisHost, _ := os.Hostname()
	for _, job := range bundle.ScheduledJobs {
		if forDeployment {
			if id := deployedBackupID(job.BackupID, bundle.SourceHost, thisHost); id != job.BackupID {
				writeDebugLog(fmt.Sprintf("[Deploy] Backup Set %q: backup ID %q becomes %q on this computer", job.Name, job.BackupID, id))
				job.BackupID = id
			}
			job.LastRun, job.NextRun, job.NextRunBy = "", "", ""
			job.MetadataRunsSinceFullRead = 0
		}
		normalizeJobSchedules(&job)
		setNextRun(&job, time.Now())
		if i, ok := byID[job.ID]; ok {
			job.Enabled = existingJobs[i].Enabled
			if forDeployment {
				// Deployed again: the set's own runs on this computer.
				job.LastRun = existingJobs[i].LastRun
				job.MetadataRunsSinceFullRead = existingJobs[i].MetadataRunsSinceFullRead
			}
			existingJobs[i] = job
		} else {
			existingJobs = append(existingJobs, job)
		}
		jobsImported++
	}

	jobsPath, err := getScheduledJobsPath()
	if err != nil {
		return serversImported, 0, fmt.Errorf("failed to get jobs path: %w", err)
	}
	jobsData, err := json.MarshalIndent(existingJobs, "", "  ")
	if err != nil {
		return serversImported, 0, fmt.Errorf("failed to marshal jobs: %w", err)
	}
	if err := atomicWriteFile(jobsPath, jobsData, 0600); err != nil {
		return serversImported, 0, fmt.Errorf("failed to write jobs file: %w", err)
	}
	return serversImported, jobsImported, nil
}
