package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"security"
)

// API wrapper methods for HTTP API compatibility
// These methods convert map[string]interface{} to typed structs

// SaveScheduledJobFromMap is an API wrapper that accepts map[string]interface{}
func (a *App) SaveScheduledJobFromMap(jobData map[string]interface{}) error {
	// Convert map to JSON then unmarshal to ScheduledJob
	jsonData, err := json.Marshal(jobData)
	if err != nil {
		return fmt.Errorf("failed to marshal job data: %w", err)
	}

	var job ScheduledJob
	if err := json.Unmarshal(jsonData, &job); err != nil {
		return fmt.Errorf("failed to unmarshal job data: %w", err)
	}

	return a.SaveScheduledJob(job)
}

// UpdateScheduledJobFromMap is an API wrapper that accepts map[string]interface{}
func (a *App) UpdateScheduledJobFromMap(jobData map[string]interface{}) error {
	jsonData, err := json.Marshal(jobData)
	if err != nil {
		return fmt.Errorf("failed to marshal job data: %w", err)
	}

	var job ScheduledJob
	if err := json.Unmarshal(jsonData, &job); err != nil {
		return fmt.Errorf("failed to unmarshal job data: %w", err)
	}

	return a.UpdateScheduledJob(job)
}

// DeleteScheduledJobFromMap is an API wrapper (same signature, just for consistency)
func (a *App) DeleteScheduledJobFromMap(jobID string) error {
	return a.DeleteScheduledJob(jobID)
}

// pinFingerprintLocal writes the pinned certificate fingerprint to the PBS server
// identified by id, in THIS process's config. It is the privileged write: in
// service mode the service (which owns config.json under ProgramData) performs it
// on the GUI's behalf via the local API, because the unprivileged GUI cannot
// overwrite a service-owned config file — TOFU pinning would otherwise never
// persist and the connection test would keep reporting the server offline.
func (a *App) pinFingerprintLocal(id, fingerprint string) error {
	if err := security.ValidateFingerprint(fingerprint); err != nil {
		return fmt.Errorf("empreinte certificat invalide: %w", err)
	}
	pbs, err := a.config.GetPBSServer(id)
	if err != nil {
		return err
	}
	pbs.CertFingerprint = fingerprint
	if err := a.config.UpdatePBSServer(pbs); err != nil {
		writeDebugLog(fmt.Sprintf("pinFingerprintLocal: could not persist fingerprint for %q: %v", id, err))
		return fmt.Errorf("impossible d'enregistrer l'empreinte (config.json non accessible en écriture ?): %w", err)
	}
	// Read the file back so the log states unambiguously whether the fingerprint
	// reached disk — this separates a write/permission failure from a TLS-apply bug
	// when diagnosing why a pinned certificate still tests offline.
	normalize := func(s string) string { return strings.ToLower(strings.ReplaceAll(s, ":", "")) }
	if verify := LoadConfig(); verify != nil {
		if s, gerr := verify.GetPBSServer(id); gerr == nil && s != nil && normalize(s.CertFingerprint) == normalize(fingerprint) {
			writeDebugLog(fmt.Sprintf("pinFingerprintLocal: fingerprint for %q persisted to disk OK", id))
		} else {
			onDisk := ""
			if s != nil {
				onDisk = s.CertFingerprint
			}
			writeDebugLog(fmt.Sprintf("pinFingerprintLocal: WARNING fingerprint for %q not on disk after save (on-disk=%q) — config.json unwritable or overwritten", id, onDisk))
		}
	}
	return nil
}

// PinServerFingerprint is the local-API entrypoint the service exposes so the
// unprivileged GUI can pin a fingerprint through the privileged service process,
// keeping the service the single writer of config.json. It performs the write in
// this (service) process.
func (a *App) PinServerFingerprint(id, fingerprint string) error {
	writeDebugLog(fmt.Sprintf("PinServerFingerprint(%s) called (service-side write)", id))
	return a.pinFingerprintLocal(id, fingerprint)
}
