package main

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	// MaxLogSize: Rotate when log reaches 10MB
	MaxLogSize = 10 * 1024 * 1024 // 10 MB

	// MaxLogFiles: Keep 5 rotated files (+ 1 current = 6 total)
	MaxLogFiles = 5
)

// RotatingLogger manages a log file with automatic rotation and compression
type RotatingLogger struct {
	path        string
	maxSize     int64
	maxFiles    int
	file        *os.File
	currentSize int64
	mu          sync.Mutex
}

// NewRotatingLogger creates a new rotating logger
func NewRotatingLogger(path string, maxSize int64, maxFiles int) (*RotatingLogger, error) {
	logger := &RotatingLogger{
		path:     path,
		maxSize:  maxSize,
		maxFiles: maxFiles,
	}

	// Get current file size if exists
	if info, err := os.Stat(path); err == nil {
		logger.currentSize = info.Size()
	}

	// Open log file (create if not exists). O_APPEND guarantees writes
	// always land at end-of-file even if someone reads from the same
	// handle; O_RDWR keeps read access available for future tooling.
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}
	logger.file = file

	return logger, nil
}

// Write writes a log message and rotates if needed
func (l *RotatingLogger) Write(message string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Check if rotation is needed
	if l.currentSize >= l.maxSize {
		if err := l.rotate(); err != nil {
			// Rotation failed but logger is still usable (truncate strategy)
			fmt.Fprintf(os.Stderr, "Log rotation failed: %v (continuing with current file)\n", err)
		}
	}

	if l.file == nil {
		return fmt.Errorf("log file is nil, cannot write")
	}
	n, err := l.file.WriteString(message)
	if err != nil {
		return fmt.Errorf("failed to write to log: %w", err)
	}

	// Do NOT Sync() here. On Windows, Sync maps to FlushFileBuffers,
	// which can block for seconds (or indefinitely under Defender
	// realtime scan, locked files, slow disks). Since we hold l.mu the
	// whole time, a blocked Sync freezes every subsequent log write
	// while the backup itself keeps running — producing a "silent
	// client, progressing PBS" state that's very hard to diagnose.
	// The OS page cache will flush within a few seconds on its own;
	// explicit Sync happens at Close(), rotation, and compression time
	// where durability actually matters.
	l.currentSize += int64(n)
	return nil
}

// rotate rotates the log file. Both the POSIX and Windows code paths
// close the current handle, move the old file aside, and reopen a
// fresh one — simple and symmetric. Windows adds a short retry-on-
// share-violation backoff after Close to cope with antivirus holding
// transient handles.
func (l *RotatingLogger) rotate() error {
	timestamp := time.Now().Format("20060102-150405")
	rotatedPath := fmt.Sprintf("%s.%s", l.path, timestamp)

	if runtime.GOOS == "windows" {
		return l.rotateWindows(rotatedPath)
	}
	return l.rotatePosix(rotatedPath)
}

// rotatePosix uses rename (safe on Linux/Mac where open files can be renamed)
func (l *RotatingLogger) rotatePosix(rotatedPath string) error {
	// Close, rename, reopen
	_ = l.file.Close()

	if err := os.Rename(l.path, rotatedPath); err != nil {
		// Reopen the original file to keep logging
		l.reopenOrDie()
		return fmt.Errorf("failed to rename log file: %w", err)
	}

	// Start background compression
	go l.compressAndCleanup(rotatedPath)

	// Open new log file — same mode as NewRotatingLogger.
	file, err := os.OpenFile(l.path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0600)
	if err != nil {
		l.reopenOrDie()
		return fmt.Errorf("failed to create new log file: %w", err)
	}

	l.file = file
	l.currentSize = 0
	return nil
}

// rotateWindows rotates by closing the current handle, renaming the
// file, and reopening a fresh one.
//
// The previous copy+truncate approach was broken: O_APPEND under Go on
// Windows maps to FILE_APPEND_DATA only (not GENERIC_WRITE), and
// SetEndOfFile — used by os.File.Truncate — requires GENERIC_WRITE. So
// every Truncate call failed with ERROR_ACCESS_DENIED, rotate() kept
// returning errors, currentSize was never reset, and every subsequent
// log line re-triggered rotation. In prod we observed bursts of 10+
// rotation files produced in < 20 s with byte-identical partial
// copies. Compression goroutines never got their .gz either because
// rotate bailed before the `go l.compressAndCleanup(...)` line.
//
// Close→Rename→Open also sidesteps the io.Copy-from-write-only-handle
// bug: no copy is done at all, the file is just atomically moved.
func (l *RotatingLogger) rotateWindows(rotatedPath string) error {
	_ = l.file.Sync()
	_ = l.file.Close()
	l.file = nil

	// Short backoff: after Close(), antivirus / Windows Search can
	// briefly hold a transient handle on the file, which causes
	// os.Rename to fail with ERROR_SHARING_VIOLATION. A few retries
	// with small sleeps cover that race without blocking the caller
	// for long.
	var renameErr error
	for _, d := range []time.Duration{0, 50 * time.Millisecond, 200 * time.Millisecond, 500 * time.Millisecond} {
		if d > 0 {
			time.Sleep(d)
		}
		renameErr = os.Rename(l.path, rotatedPath)
		if renameErr == nil {
			break
		}
	}
	if renameErr != nil {
		l.reopenOrDie()
		return fmt.Errorf("failed to rename log file for rotation: %w", renameErr)
	}

	file, err := os.OpenFile(l.path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0600)
	if err != nil {
		l.reopenOrDie()
		return fmt.Errorf("failed to reopen log file after rotation: %w", err)
	}
	l.file = file
	l.currentSize = 0

	go l.compressAndCleanup(rotatedPath)
	return nil
}

// reopenOrDie tries to reopen the log file after a failed rotation.
// If it can't, sets l.file to nil (writes will go to stderr).
func (l *RotatingLogger) reopenOrDie() {
	file, err := os.OpenFile(l.path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0600)
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: Cannot reopen log file %s: %v\n", l.path, err)
		l.file = nil
		return
	}
	l.file = file
	if info, err := l.file.Stat(); err == nil {
		l.currentSize = info.Size()
	}
}

// compressAndCleanup compresses a rotated log file and cleans up old logs
func (l *RotatingLogger) compressAndCleanup(rotatedPath string) {
	if err := compressLogFile(rotatedPath); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to compress log %s: %v\n", rotatedPath, err)
	}
	if err := cleanupOldLogs(l.path, l.maxFiles); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to cleanup old logs: %v\n", err)
	}
}

// Close flushes outstanding writes to disk and closes the log file.
// Since Write() no longer fsyncs on every line, this is the durability
// boundary for the logger — on a clean shutdown (service stop, end of
// run), everything we wrote makes it to stable storage.
func (l *RotatingLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file == nil {
		return nil
	}
	_ = l.file.Sync()
	return l.file.Close()
}

// Flush forces outstanding writes to disk without closing the file.
// Call this from a slow periodic ticker if you need durability between
// rotations — not from the hot path, since Sync can block for seconds
// on Windows.
func (l *RotatingLogger) Flush() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return nil
	}
	return l.file.Sync()
}

// compressLogFile compresses a log file with gzip and removes the original
func compressLogFile(path string) error {
	src, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}

	gzPath := path + ".gz"
	dst, err := os.Create(gzPath)
	if err != nil {
		_ = src.Close()
		return fmt.Errorf("failed to create compressed file: %w", err)
	}

	gzWriter := gzip.NewWriter(dst)

	_, copyErr := io.Copy(gzWriter, src)
	closeGzErr := gzWriter.Close()
	// Force the gzip footer + compressed body to disk before we delete
	// the source. Without this a hard crash here leaves a .gz with no
	// footer (decodes as "unexpected end of file") and no source to
	// recover from.
	syncErr := dst.Sync()
	_ = dst.Close()
	_ = src.Close()

	if copyErr != nil {
		_ = os.Remove(gzPath)
		return fmt.Errorf("failed to compress: %w", copyErr)
	}
	if closeGzErr != nil {
		_ = os.Remove(gzPath)
		return fmt.Errorf("failed to close gzip writer: %w", closeGzErr)
	}
	if syncErr != nil {
		_ = os.Remove(gzPath)
		return fmt.Errorf("failed to fsync compressed log: %w", syncErr)
	}

	// Remove original - on Windows this can fail, retry once after a short delay
	if err := os.Remove(path); err != nil {
		time.Sleep(500 * time.Millisecond)
		if err := os.Remove(path); err != nil {
			// Not fatal - the .gz exists, uncompressed file is just wasted space
			fmt.Fprintf(os.Stderr, "Warning: could not remove %s after compression: %v\n", path, err)
		}
	}

	return nil
}

// sanitizeForFilename replaces characters that are unsafe or awkward in filenames.
// Used to embed a backup-id into a log filename.
func sanitizeForFilename(s string) string {
	if s == "" {
		return "unnamed"
	}
	var b []byte
	for _, c := range []byte(s) {
		switch {
		case c >= 'A' && c <= 'Z', c >= 'a' && c <= 'z', c >= '0' && c <= '9':
			b = append(b, c)
		case c == '_' || c == '-' || c == '.':
			b = append(b, c)
		default:
			b = append(b, '_')
		}
	}
	if len(b) == 0 {
		return "unnamed"
	}
	return string(b)
}

// perRunLogRe matches a per-run uncompressed log file left behind when the
// process died before EndBackupRunLog's compression ran to completion.
var perRunLogRe = regexp.MustCompile(`^backup-\d{8}-\d{6}-.+\.log$`)

// rotatedLeftoverRe matches an intermediate rotation file whose compression
// goroutine was killed before it finished.
var rotatedLeftoverRe = regexp.MustCompile(`\.log\.\d{8}-\d{6}$`)

// RecoverOrphanLogs gzips any uncompressed per-run log or intermediate
// rotation file found in dir. Intended to run at service startup, before
// any backup has had a chance to begin. It is safe to call when no orphans
// exist (no-op) and cheap: the scan is bounded by the rotation cap.
//
// Rationale: EndBackupRunLog and rotateWindows both hand off compression
// to a goroutine. If the service is killed (reboot, task-scheduler kill,
// crash) between "compression started" and "gzip footer flushed", the .gz
// on disk is truncated. Leaving a full .log alongside a partial .gz lets
// us recover here on the next start.
func RecoverOrphanLogs(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !perRunLogRe.MatchString(name) && !rotatedLeftoverRe.MatchString(name) {
			continue
		}
		path := filepath.Join(dir, name)
		if err := compressLogFile(path); err != nil {
			fmt.Fprintf(os.Stderr, "orphan log recovery failed for %s: %v\n", name, err)
		}
	}
}

// cleanupOldLogs removes old rotated log files, keeping only maxFiles
func cleanupOldLogs(basePath string, maxFiles int) error {
	dir := filepath.Dir(basePath)
	baseName := filepath.Base(basePath)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read log directory: %w", err)
	}

	var rotatedLogs []os.DirEntry
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, baseName+".") && name != baseName {
			rotatedLogs = append(rotatedLogs, entry)
		}
	}

	if len(rotatedLogs) <= maxFiles {
		return nil
	}

	sort.Slice(rotatedLogs, func(i, j int) bool {
		return rotatedLogs[i].Name() < rotatedLogs[j].Name()
	})

	filesToRemove := len(rotatedLogs) - maxFiles
	for i := 0; i < filesToRemove; i++ {
		filePath := filepath.Join(dir, rotatedLogs[i].Name())
		if err := os.Remove(filePath); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to remove old log %s: %v\n", filePath, err)
		}
	}

	return nil
}
