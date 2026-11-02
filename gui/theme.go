package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// The Preferences > Theme choice is each user's own, like the language: one
// user's colours do not change another's. It lives in the user's own folder
// (themeFilePath), not in the machine-wide settings. A user who has not
// chosen yet gets the theme earlier versions saved for the whole computer,
// if any, so nobody's colours change on upgrade; with none, the frontend
// uses the brand's own colours (green for the default brand).

// themeFilePath is the user's theme file. A variable so tests can point it
// elsewhere.
var themeFilePath = func() (string, error) {
	dir, err := userThemeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "theme.json"), nil
}

// SaveTheme saves the user's Preferences > Theme choice.
func (a *App) SaveTheme(theme ThemeSettings) error {
	path, err := themeFilePath()
	if err != nil {
		return fmt.Errorf("failed to save theme: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("failed to save theme: %w", err)
	}
	data, err := json.MarshalIndent(theme, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to save theme: %w", err)
	}
	if err := atomicWriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("failed to save theme: %w", err)
	}
	return nil
}

// GetTheme returns the user's theme, else the one earlier versions saved
// for the whole computer, else a zero-value ThemeSettings (empty Preset):
// the frontend treats that as "nothing saved, use the brand's colours".
func (a *App) GetTheme() ThemeSettings {
	if path, err := themeFilePath(); err == nil {
		if data, err := os.ReadFile(path); err == nil {
			var t ThemeSettings
			if json.Unmarshal(stripUTF8BOM(data), &t) == nil && t.Preset != "" {
				return t
			}
		}
	}
	if a.config != nil && a.config.Theme != nil {
		return *a.config.Theme
	}
	return ThemeSettings{}
}

// SetParallelRestore persists the Preferences > Advanced "parallel restore
// extraction" toggle. Deliberately bypasses SaveConfig/Validate:
// Validate() unconditionally requires a non-empty legacy
// BaseURL, which is genuinely empty for anyone on multi-PBS (real servers
// live in PBSServers instead) — routing this general, connection-unrelated
// preference through SaveConfig failed with "PBS server URL required" for
// exactly that setup (found 2026-09-23, Mick's own multi-PBS config).
func (a *App) SetParallelRestore(enabled bool) error {
	a.config.ParallelRestore = enabled
	if err := a.config.Save(); err != nil {
		return fmt.Errorf("failed to save parallel restore setting: %w", err)
	}
	return nil
}

// SetMaxConcurrentRuns saves Preferences > Backup Options "Maximum Concurrent Backups": how
// many backups and restores may run at the same time (operation_queue.go).
// It takes effect for the next run to start; the service reads it before
// each run. Same SaveConfig/Validate bypass as SetParallelRestore below.
func (a *App) SetMaxConcurrentRuns(n int) error {
	if n < 1 || n > MaxConcurrentRunsLimit {
		return fmt.Errorf("maximum concurrent backups must be between 1 and %d", MaxConcurrentRunsLimit)
	}
	a.config.MaxConcurrentRuns = n
	if err := a.config.Save(); err != nil {
		return fmt.Errorf("failed to save the maximum concurrent backups setting: %w", err)
	}
	writeDebugLog(fmt.Sprintf("Maximum Concurrent Backups set to %d", n))
	// Runs waiting for a free place may start now.
	opMu.Lock()
	admitWaitingLocked()
	opMu.Unlock()
	return nil
}

// SetParallelBackupRead persists the Preferences > Advanced "multi-threaded
// backup reading" toggle and its worker count together (not two separate
// setters) so a change to either always lands as one consistent save — same
// SaveConfig/Validate bypass as SetParallelRestore above, for the same
// multi-PBS reason. workers <= 0 is accepted and just means "use the
// default" (see Config.EffectivePrefetchWorkers).
func (a *App) SetParallelBackupRead(enabled bool, workers int) error {
	a.config.ParallelBackupRead = enabled
	a.config.BackupReadWorkers = workers
	if err := a.config.Save(); err != nil {
		return fmt.Errorf("failed to save backup read-ahead setting: %w", err)
	}
	return nil
}
