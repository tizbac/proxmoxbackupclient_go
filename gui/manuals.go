package main

import (
	"embed"
	"fmt"
	"os"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// The user manuals, built into the app so Help works offline and always
// matches this version. go:embed cannot reach outside the gui module, so
// gui/docs mirrors the repo's docs/ folder for these four files (same layout,
// so their relative links resolve the same way). docs/ is the copy to edit;
// TestEmbeddedManualsMatchDocs fails when the two differ, and `make manuals`
// copies them across.
//
//go:embed docs/manual/GUI.md docs/manual/CLI.md docs/manual/BMR.md docs/RESTORE_GUIDE.md
var manualFS embed.FS

type manualInfo struct {
	path string // inside manualFS, and inside the repo
	file string // default name for Save a copy
}

var manuals = map[string]manualInfo{
	"gui":     {"docs/manual/GUI.md", "Proxmox-Backup-Client-GUI-manual.md"},
	"cli":     {"docs/manual/CLI.md", "Proxmox-Backup-Client-command-line-manual.md"},
	"bmr":     {"docs/manual/BMR.md", "Proxmox-Backup-Client-bare-metal-restore-manual.md"},
	"restore": {"docs/RESTORE_GUIDE.md", "Proxmox-Backup-Client-restore-scenarios.md"},
}

// GetManual returns one manual's Markdown ("gui", "cli", "bmr" or "restore").
func (a *App) GetManual(id string) (string, error) {
	m, ok := manuals[id]
	if !ok {
		return "", fmt.Errorf("unknown manual %q", id)
	}
	b, err := manualFS.ReadFile(m.path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// SaveManual saves one manual to a file the user picks, for reading or
// printing away from the app (the bare-metal restore manual on a phone, say).
// Returns the chosen path, or "" if the dialog was cancelled.
func (a *App) SaveManual(id string) (string, error) {
	if a.isServiceProcess {
		return "", fmt.Errorf("unavailable in service mode")
	}
	m, ok := manuals[id]
	if !ok {
		return "", fmt.Errorf("unknown manual %q", id)
	}
	b, err := manualFS.ReadFile(m.path)
	if err != nil {
		return "", err
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Save a copy of the manual",
		DefaultFilename: m.file,
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
	if err := os.WriteFile(path, b, 0644); err != nil {
		return "", fmt.Errorf("failed to write the manual: %w", err)
	}
	return path, nil
}
