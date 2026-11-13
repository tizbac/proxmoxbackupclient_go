package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
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
	// Copy: the button opens a copy of the logs, not the folder itself (see
	// fileManagerLacksOurRights).
	Copy bool `json:"copy"`
}

// GetServiceLogsStatus tells the window whether the service's logs can be
// opened here, or need the app restarted as an administrator.
func (a *App) GetServiceLogsStatus() ServiceLogsStatus {
	folder, state := serviceLogsAccess()
	return ServiceLogsStatus{
		Folder: folder, State: state,
		Listed: a.logsThroughService() || state == "readable",
		Copy:   state == "readable" && fileManagerLacksOurRights(),
	}
}

// serviceLogsCopyName is the folder, in the user's own log folder, that holds
// the copy of the service's logs OpenServiceLogsFolder makes.
const serviceLogsCopyName = "service-logs-copy"

// OpenServiceLogsFolder opens the service's log folder in the file manager. A
// process that may not read it gets SERVICE_LOGS_NEED_ADMIN, and the window
// then offers to restart the app as an administrator (RequestElevation).
//
// Where the file manager does not have this process's rights (an app
// restarted as administrator on Windows: File Explorer is not), it could not
// show the folder, and Explorer's own way round that is to give the user
// permanent full access to the service's settings folder. So there the logs
// are copied to the user's own log folder and the copy is opened.
func (a *App) OpenServiceLogsFolder() error {
	folder, state := serviceLogsAccess()
	switch state {
	case "readable":
		if fileManagerLacksOurRights() {
			copyDir, err := copyServiceLogs(folder, filepath.Join(logDir, serviceLogsCopyName))
			if err != nil {
				return fmt.Errorf("copy the service's logs: %w", err)
			}
			return openFolderInFileManager(copyDir)
		}
		return openFolderInFileManager(folder)
	case "locked":
		return fmt.Errorf("SERVICE_LOGS_NEED_ADMIN")
	}
	return fmt.Errorf("the service has no log folder on this computer")
}

// copyServiceLogs copies the log files in src (and nothing else that is
// there: the service's settings and keys stay where they are) to dst, in
// place of what an earlier copy left, and returns dst.
func copyServiceLogs(src, dst string) (string, error) {
	entries, err := os.ReadDir(src)
	if err != nil {
		return "", err
	}
	if err := os.RemoveAll(dst); err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return "", err
	}
	// Mkdir, not MkdirAll: dst must be the new folder made here.
	if err := os.Mkdir(dst, 0o700); err != nil {
		return "", err
	}
	for _, e := range entries {
		if !e.Type().IsRegular() {
			continue
		}
		if _, _, _, ok := classifyLog(e.Name()); !ok {
			continue
		}
		if err := copyLogFile(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
			return "", err
		}
	}
	return dst, nil
}

// copyLogFile copies one log, keeping its time so the copies sort as the
// originals do. A log the service is still writing is copied as far as it goes.
func copyLogFile(from, to string) error {
	in, err := os.Open(from) // #nosec G304 -- a log file name listed from the service's own folder
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(to, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600) // #nosec G304 -- in the folder made just above
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	if info, err := in.Stat(); err == nil {
		_ = os.Chtimes(to, info.ModTime(), info.ModTime())
	}
	return nil
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
