package main

import (
	_ "embed"
	"fmt"
	"os"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// bmrGuideMarkdown is the Bare Metal Restore guide shown by the "View Bare
// Metal Restore Guide" screen and offered by "Download Bare Metal Restore
// Guide…". Embedded directly in the binary (rather than fetched from GitHub,
// the way "Download Bare Metal Restore ISO…" works) so it's available
// completely offline — the one moment you're most likely to need this guide
// is exactly when internet access can't be assumed. Source lives at
// gui/docs/BARE_METAL_RESTORE.md rather than the repo-root docs/ folder
// because go:embed patterns can't reach outside this module's own directory
// tree; this file is the canonical, accurate copy of this guide.
//
//go:embed docs/BARE_METAL_RESTORE.md
var bmrGuideMarkdown string

// GetBMRGuideContent returns the Bare Metal Restore guide's full Markdown
// text, for in-app rendering.
func (a *App) GetBMRGuideContent() string {
	return bmrGuideMarkdown
}

// DownloadBMRGuide saves the Bare Metal Restore guide to a user-chosen file,
// mirroring ExportSettings' save-dialog pattern in settings_export.go.
// Returns the chosen path, or "" if the dialog was cancelled.
func (a *App) DownloadBMRGuide() (string, error) {
	if a.isServiceProcess {
		return "", fmt.Errorf("unavailable in service mode")
	}

	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Save Bare Metal Restore Guide",
		DefaultFilename: "Bare-Metal-Restore-Guide.md",
		Filters: []runtime.FileFilter{
			{DisplayName: "Markdown (*.md)", Pattern: "*.md"},
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
		},
	})
	if err != nil {
		return "", fmt.Errorf("save dialog failed: %w", err)
	}
	if path == "" {
		return "", nil // cancelled
	}

	if err := os.WriteFile(path, []byte(bmrGuideMarkdown), 0644); err != nil {
		return "", fmt.Errorf("failed to write guide file: %w", err)
	}
	return path, nil
}
