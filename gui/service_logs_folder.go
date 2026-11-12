package main

import (
	"fmt"
	"os/exec"
	"runtime"
)

// ServiceLogsStatus is what the window needs to offer the service's logs.
type ServiceLogsStatus struct {
	Folder string `json:"folder"`
	// State: "readable" (this process may open the folder), "locked" (it is
	// there, and only an administrator may read it), "none" (no service folder).
	State string `json:"state"`
	// Listed: the Logs page already lists the service's logs (through the
	// service, or by reading the folder), so nothing needs unlocking to view them.
	Listed bool `json:"listed"`
}

// GetServiceLogsStatus tells the window whether the service's logs can be
// opened here, or need the app restarted as an administrator.
func (a *App) GetServiceLogsStatus() ServiceLogsStatus {
	folder, state := serviceLogsAccess()
	return ServiceLogsStatus{Folder: folder, State: state, Listed: a.logsThroughService() || state == "readable"}
}

// OpenServiceLogsFolder opens the service's log folder in the file manager. A
// process that may not read it gets SERVICE_LOGS_NEED_ADMIN, and the window
// then offers to restart the app as an administrator (RequestElevation).
func (a *App) OpenServiceLogsFolder() error {
	folder, state := serviceLogsAccess()
	switch state {
	case "readable":
		return openFolderInFileManager(folder)
	case "locked":
		return fmt.Errorf("SERVICE_LOGS_NEED_ADMIN")
	}
	return fmt.Errorf("the service has no log folder on this computer")
}

// openFolderInFileManager starts the platform's file manager on dir. A
// variable so a test can see what would be opened.
var openFolderInFileManager = func(dir string) error {
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
