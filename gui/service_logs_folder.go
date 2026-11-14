package main

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
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
	// Listed: the Logs page already lists the service's logs (the service
	// gives them, or this process reads the folder), so nothing needs
	// unlocking to view them.
	Listed bool `json:"listed"`
	// Copy: the button opens a copy of the logs, not the folder itself (see
	// OpenServiceLogsFolder).
	Copy bool `json:"copy"`
}

// GetServiceLogsStatus tells the window how the service's logs can be had
// here: from the service, from its folder, or only by restarting the app as
// an administrator (the folder is locked and the service gives none).
func (a *App) GetServiceLogsStatus() ServiceLogsStatus {
	folder, state := serviceLogsAccess()
	_, served := a.serviceLogList()
	return ServiceLogsStatus{
		Folder: folder, State: state,
		Listed: served || state == "readable",
		Copy:   (state == "readable" && fileManagerLacksOurRights()) || (state == "locked" && served),
	}
}

const (
	// serviceLogsCopyName is the folder, in the user's own log folder, that
	// holds the copy of the service's logs OpenServiceLogsFolder makes.
	serviceLogsCopyName = "service-logs-copy"
	// serviceLogsCopyDays is how far back the copy goes when the service
	// gives the logs (it packs the newest first, up to logBundleMaxBytes).
	serviceLogsCopyDays = 365
	// serviceLogsCopyMaxBytes caps what a bundle from the service may unpack to.
	serviceLogsCopyMaxBytes = 2 * logBundleMaxBytes
)

// OpenServiceLogsFolder opens the service's logs in the file manager: the
// folder itself where the file manager may read it, a copy of the logs
// otherwise.
//
// The folder holds the service's settings too and stays closed to everyone
// but the service and an administrator. So a person who may not read it gets
// the logs from the service (it gives them to any user of the computer,
// api/public_logs.go), unpacked into their own log folder. Only when the
// service gives none (it is stopped) is the answer SERVICE_LOGS_NEED_ADMIN,
// and the window offers to restart the app as an administrator
// (RequestElevation).
//
// An app restarted as administrator on Windows reads the folder, and File
// Explorer, which is not restarted, does not: Explorer's own way round that
// is to give the user permanent full access to the service's settings
// folder. So there too the logs are copied and the copy is opened.
func (a *App) OpenServiceLogsFolder() error {
	folder, state := serviceLogsAccess()
	copyDir := filepath.Join(logDir, serviceLogsCopyName)
	switch state {
	case "readable":
		if fileManagerLacksOurRights() {
			if _, err := copyServiceLogs(folder, copyDir); err != nil {
				return fmt.Errorf("copy the service's logs: %w", err)
			}
			return openFolderInFileManager(copyDir)
		}
		return openFolderInFileManager(folder)
	case "locked":
		if !a.asksServiceForLogs() {
			return fmt.Errorf("SERVICE_LOGS_NEED_ADMIN")
		}
		data, err := a.apiClient.LogBundle(serviceLogsCopyDays)
		if err != nil {
			writeDebugLog(fmt.Sprintf("service logs: the service gave none: %v", err))
			return fmt.Errorf("SERVICE_LOGS_NEED_ADMIN")
		}
		if _, err := unpackServiceLogs(data, copyDir, serviceLogsCopyMaxBytes); err != nil {
			return fmt.Errorf("copy the service's logs: %w", err)
		}
		return openFolderInFileManager(copyDir)
	}
	return fmt.Errorf("the service has no log folder on this computer")
}

// newLogsCopyFolder makes dst as a new, empty folder of this user's, in place
// of what an earlier copy left there.
func newLogsCopyFolder(dst string) error {
	if err := os.RemoveAll(dst); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	// Mkdir, not MkdirAll: dst must be the new folder made here.
	return os.Mkdir(dst, 0o700)
}

// unpackServiceLogs writes the log files of a zip the service gave (LogBundle)
// to dst, in place of what an earlier copy left, and returns dst. Only log
// files are written, each under its own plain name, and no more than maxBytes
// in all: what the zip says decides neither where a file goes nor how much
// is written.
func unpackServiceLogs(data []byte, dst string, maxBytes int64) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	// ErrInsecurePath comes with a usable reader: such names are passed over below.
	if err != nil && (!errors.Is(err, zip.ErrInsecurePath) || zr == nil) {
		return "", err
	}
	if err := newLogsCopyFolder(dst); err != nil {
		return "", err
	}
	left := maxBytes
	for _, f := range zr.File {
		name := f.Name
		if left <= 0 {
			break
		}
		if !f.Mode().IsRegular() || name != filepath.Base(name) || !safeLogName.MatchString(name) {
			continue
		}
		if _, _, _, ok := classifyLog(name); !ok {
			continue
		}
		n, err := unpackLogFile(f, filepath.Join(dst, name), left)
		left -= n
		if err != nil && !errors.Is(err, fs.ErrExist) {
			return "", err
		}
	}
	return dst, nil
}

// unpackLogFile writes at most limit bytes of the zip entry f to the new file
// to, keeping the log's time, and returns how many it wrote.
func unpackLogFile(f *zip.File, to string, limit int64) (int64, error) {
	in, err := f.Open()
	if err != nil {
		return 0, err
	}
	defer func() { _ = in.Close() }()
	out, err := os.OpenFile(to, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600) // #nosec G304 -- a checked log file name in the folder made just before
	if err != nil {
		return 0, err
	}
	n, err := io.Copy(out, io.LimitReader(in, limit))
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return n, err
	}
	if !f.Modified.IsZero() {
		_ = os.Chtimes(to, f.Modified, f.Modified)
	}
	return n, nil
}

// copyServiceLogs copies the log files in src (and nothing else that is
// there: the service's settings and keys stay where they are) to dst, in
// place of what an earlier copy left, and returns dst.
func copyServiceLogs(src, dst string) (string, error) {
	entries, err := os.ReadDir(src)
	if err != nil {
		return "", err
	}
	if err := newLogsCopyFolder(dst); err != nil {
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
