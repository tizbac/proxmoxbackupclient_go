package main

import (
	"fmt"
	"os"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// SaveLogs asks where to save and writes one zip with the app's logs and the
// service's (see buildLogBundle). It returns the path, or "" when the save
// dialog was cancelled. The file is owner-only: the logs name this computer's
// folders and servers.
func (a *App) SaveLogs() (string, error) {
	if a.isServiceProcess {
		return "", fmt.Errorf("saving logs is not available in service mode")
	}
	data, err := a.buildLogBundle(logBundleDays)
	if err != nil {
		return "", fmt.Errorf("collecting the logs: %w", err)
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Save logs",
		DefaultFilename: fmt.Sprintf("%s-logs-%s.zip", product().FileBase, time.Now().Format("20060102-150405")),
		Filters: []runtime.FileFilter{
			{DisplayName: "Zip files (*.zip)", Pattern: "*.zip"},
		},
	})
	if err != nil {
		return "", fmt.Errorf("save dialog failed: %w", err)
	}
	if path == "" {
		return "", nil // cancelled
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return "", fmt.Errorf("failed to write the logs file: %w", err)
	}
	writeDebugLog(fmt.Sprintf("SaveLogs: wrote %s (%d bytes)", path, len(data)))
	return path, nil
}
