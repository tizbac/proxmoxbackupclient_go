package main

import (
	"fmt"
	"os/exec"
	"runtime"
)

// OpenLogsFolder opens the log directory in the platform's file manager.
//
// It deliberately does not go through the frontend's BrowserOpenURL: Wails
// 2.13 validates every URL and rejects a bare path or a file: URL, so the old
// "View Logs" button silently did nothing.
func (a *App) OpenLogsFolder() error {
	dir := a.GetLogsFolder()
	if dir == "" {
		return fmt.Errorf("log folder is not known")
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		// explorer.exe returns exit status 1 even on success, so only Start matters.
		cmd = exec.Command("explorer.exe", dir)
	case "darwin":
		cmd = exec.Command("open", dir)
	default:
		cmd = exec.Command("xdg-open", dir)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open log folder: %w", err)
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
