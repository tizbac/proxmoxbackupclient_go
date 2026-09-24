package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// MessageLogEntry is one user-facing event line in the Message Log page —
// a short, readable audit trail ("backup completed", "cannot access file
// X"), distinct from the much more verbose debug/backup logs on disk that
// only support transcripts use.
type MessageLogEntry struct {
	Timestamp string `json:"timestamp"` // ISO format
	Source    string `json:"source"`    // "Backup" | "Restore" | "App"
	Level     string `json:"level"`     // "info" | "warning" | "error"
	Message   string `json:"message"`

	// JobName is the scheduled/manual job's own name, when this entry came
	// from one (empty for restore entries, which have no job). Kept separate
	// from MessageKey/MessageParams — see below — since a job's name is a
	// user-assigned label, not backend-generated text, and so is never
	// itself translated; the frontend renders "{jobName}: {translated}"
	// when both are present.
	JobName string `json:"job_name,omitempty"`

	// MessageKey/MessageParams — see BackupStatus's fields of the same name
	// in backup_status.go and msgcodes.go's doc comment. Empty on every
	// entry persisted before this field existed; the frontend falls back to
	// Message for those.
	MessageKey    MessageKey `json:"message_key,omitempty"`
	MessageParams msgParams  `json:"message_params,omitempty"`
}

// maxMessageLogEntries matches the cap shown in the UI ("1,000 message
// cap"). Oldest entries are dropped once the log is full.
const maxMessageLogEntries = 1000

var messageLogMutex sync.Mutex

func getMessageLogPath() (string, error) {
	configDir, err := getConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "message_log.json"), nil
}

// LogMessage appends one entry to the persistent Message Log. Failures are
// logged to the debug log and otherwise swallowed — a logging call must
// never be able to fail the backup/restore operation it's describing.
//
// jobName, key and params are optional (pass "", "" and nil for a plain,
// unkeyed entry — e.g. a general App-source message with no translation
// available yet); see MessageLogEntry's doc comment.
func LogMessage(source, level, message, jobName string, key MessageKey, params msgParams) {
	messageLogMutex.Lock()
	defer messageLogMutex.Unlock()

	logPath, err := getMessageLogPath()
	if err != nil {
		writeDebugLog(fmt.Sprintf("LogMessage: cannot resolve path: %v", err))
		return
	}

	var entries []MessageLogEntry
	if data, rerr := os.ReadFile(logPath); rerr == nil {
		_ = json.Unmarshal(data, &entries) // a corrupt file just starts a fresh log, not a crash
	}

	entries = append(entries, MessageLogEntry{
		Timestamp:     time.Now().Format(time.RFC3339),
		Source:        source,
		Level:         level,
		Message:       message,
		JobName:       jobName,
		MessageKey:    key,
		MessageParams: params,
	})

	if len(entries) > maxMessageLogEntries {
		entries = entries[len(entries)-maxMessageLogEntries:]
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		writeDebugLog(fmt.Sprintf("LogMessage: marshal failed: %v", err))
		return
	}
	if err := atomicWriteFile(logPath, data, 0600); err != nil {
		writeDebugLog(fmt.Sprintf("LogMessage: write failed: %v", err))
	}
}

// GetMessageLog returns the Message Log, newest entry first (the order the
// UI displays it in).
func (a *App) GetMessageLog() ([]MessageLogEntry, error) {
	logPath, err := getMessageLogPath()
	if err != nil {
		return []MessageLogEntry{}, err
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []MessageLogEntry{}, nil
		}
		return nil, err
	}

	var entries []MessageLogEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}

	// Reverse in place — stored oldest-first (append-friendly), displayed newest-first.
	for i, j := 0, len(entries)-1; i < j; i, j = i+1, j-1 {
		entries[i], entries[j] = entries[j], entries[i]
	}

	return entries, nil
}
