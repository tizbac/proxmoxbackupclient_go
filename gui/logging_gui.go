//go:build !service
// +build !service

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

var (
	backupLogger  *RotatingLogger
	serviceLogger *RotatingLogger
	logDir        string

	// currentBackupLogger is set for the duration of a backup run so logs for
	// that run land in a dedicated file. writeBackupLog uses this if set.
	currentBackupLogger   *RotatingLogger
	currentBackupLoggerMu sync.RWMutex
)

func init() {
	// Setup log directory for GUI
	if runtime.GOOS == "windows" {
		programData := os.Getenv("ProgramData")
		if programData == "" {
			programData = "C:\\ProgramData"
		}
		logDir = filepath.Join(programData, "NimbusBackup")
	} else {
		logDir = "/var/log/nimbusbackup"
	}
	// #nosec G703 -- ProgramData is a trusted Windows system environment variable
	_ = os.MkdirAll(logDir, 0700)

	// Initialize rotating loggers
	var err error
	backupLogger, err = NewRotatingLogger(
		filepath.Join(logDir, "backup-gui.log"),
		MaxLogSize,
		MaxLogFiles,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create backup logger: %v\n", err)
	}

	serviceLogger, err = NewRotatingLogger(
		filepath.Join(logDir, "service-gui.log"),
		MaxLogSize,
		MaxLogFiles,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create service logger: %v\n", err)
	}

	// Compress any per-run or rotated log files left behind by a previous
	// crash/kill so operators get a full, inspectable .gz.
	RecoverOrphanLogs(logDir)
}

// GetServiceLogPath returns the path to the service log file
func GetServiceLogPath() string {
	return filepath.Join(logDir, "service-gui.log")
}

// GetBackupLogPath returns the path to the backup log file
func GetBackupLogPath() string {
	return filepath.Join(logDir, "backup-gui.log")
}

// StartBackupRunLog creates a dedicated log file for the current backup run and
// installs it as the active backup logger. Returns the new logger so the caller
// can close/compress it at the end of the run.
// If the backup produces > MaxLogSize bytes of log, the file is rotated internally.
func StartBackupRunLog(backupID string) *RotatingLogger {
	timestamp := time.Now().Format("20060102-150405")
	name := fmt.Sprintf("backup-%s-%s.log", timestamp, sanitizeForFilename(backupID))
	path := filepath.Join(logDir, name)

	logger, err := NewRotatingLogger(path, MaxLogSize, MaxLogFiles)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create per-run backup logger %s: %v\n", path, err)
		return nil
	}

	currentBackupLoggerMu.Lock()
	currentBackupLogger = logger
	currentBackupLoggerMu.Unlock()
	return logger
}

// EndBackupRunLog closes the per-run backup logger and compresses the log file.
// Safe to call with a nil logger.
func EndBackupRunLog(logger *RotatingLogger) {
	if logger == nil {
		return
	}

	currentBackupLoggerMu.Lock()
	if currentBackupLogger == logger {
		currentBackupLogger = nil
	}
	currentBackupLoggerMu.Unlock()

	path := logger.path
	_ = logger.Close()

	// Synchronous compression: after a multi-hour backup, an extra second
	// to flush a gzip footer is acceptable, and it guarantees that a
	// service stop/kill right after EndBackupRunLog returns can't leave a
	// truncated .log.gz on disk.
	if err := compressLogFile(path); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to compress backup run log %s: %v\n", path, err)
	}
}

// writeDebugLog writes to service log (scheduler, general operations)
func writeDebugLog(message string) {
	writeLogToLogger(serviceLogger, "SERVICE", message)
}

// writeBackupLog writes to backup log (backup operations).
// Prefers the per-run logger if one is active, else falls back to the global.
func writeBackupLog(message string) {
	currentBackupLoggerMu.RLock()
	perRun := currentBackupLogger
	currentBackupLoggerMu.RUnlock()

	if perRun != nil {
		writeLogToLogger(perRun, "BACKUP", message)
		return
	}
	writeLogToLogger(backupLogger, "BACKUP", message)
}

func writeLogToLogger(logger *RotatingLogger, prefix string, message string) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	logLine := fmt.Sprintf("[%s] [%s] %s\n", prefix, timestamp, message)

	// Fallback to stderr if logger is not initialized
	if logger == nil {
		fmt.Fprint(os.Stderr, logLine)
		return
	}

	// Write to rotating logger
	if err := logger.Write(logLine); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write log: %v\n", err)
		fmt.Fprint(os.Stderr, logLine)
	}

	// Also write to stderr for console output
	fmt.Fprint(os.Stderr, logLine)
}
