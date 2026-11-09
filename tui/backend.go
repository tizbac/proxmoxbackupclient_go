package tui

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/salvanza/salvanza_client/gui/api"
)

// Backend is what the screens need from the Salvanza service. The real one
// talks to the local service API (gui/api, documented in its INTEGRATION.md);
// the demo one invents moving data so the screens can be tried and tested
// without a service.
type Backend interface {
	Status() (*api.StatusResponse, error)
	Sets() ([]Set, error)
	Runs() ([]api.BackupProgress, error)
	RunSet(id string) error
	Control(action, jobID string) error // "pause", "resume" or "stop"
	SetEnabled(id string, on bool) error
	History() ([]HistoryEntry, error)
	Messages() ([]MessageEntry, error)
	Servers() ([]Server, error)
	Name() string
}

// Set is a Backup Set as the screens show it.
type Set struct {
	ID, Name, Type, BackupID, ServerID string
	Enabled                            bool
	Manual                             bool
	Schedule                           string
	LastRun, NextRun                   time.Time
	Sources                            []string
	Raw                                map[string]interface{}
}

// HistoryEntry is one finished run.
type HistoryEntry struct {
	ID, Name, Status, Message, BackupID string
	At                                  time.Time
	Sources                             []string
	Encryption                          string
}

// MessageEntry is one Message Log line.
type MessageEntry struct {
	At             time.Time
	Level, Message string
	Source         string
}

// Server is a configured PBS server (never its login).
type Server struct {
	ID, Name, URL, Datastore, Namespace, Fingerprint string
	Default                                           bool
	Login                                             string
	Encrypted                                         bool
}

var errNotOffered = errors.New("this service does not offer that")

func str(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			switch t := v.(type) {
			case string:
				if t != "" {
					return t
				}
			case float64:
				return fmt.Sprintf("%g", t)
			case bool:
				return fmt.Sprintf("%t", t)
			}
		}
	}
	return ""
}

func boolean(m map[string]interface{}, key string) bool {
	b, _ := m[key].(bool)
	return b
}

func strList(m map[string]interface{}, key string) []string {
	arr, _ := m[key].([]interface{})
	out := make([]string, 0, len(arr))
	for _, v := range arr {
		if s, ok := v.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02T15:04:05", "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Local()
		}
	}
	return time.Time{}
}

// scheduleText summarises when a set runs, from the fields the service sends.
func scheduleText(m map[string]interface{}) string {
	mode := str(m, "scheduleMode")
	trig := str(m, "triggerMode")
	if mode == "manual" || trig == "manual" {
		return "manual"
	}
	var parts []string
	if arr, ok := m["schedules"].([]interface{}); ok {
		for _, s := range arr {
			sm, _ := s.(map[string]interface{})
			if sm == nil || (sm["enabled"] == false) {
				continue
			}
			parts = append(parts, oneSchedule(sm))
		}
	}
	if len(parts) == 0 {
		parts = append(parts, oneSchedule(m))
	}
	return strings.Join(parts, ", ")
}

func oneSchedule(m map[string]interface{}) string {
	trig := str(m, "triggerMode", "mode")
	days := strList(m, "daysOfWeek")
	d := ""
	if len(days) > 0 && len(days) < 7 {
		d = " " + strings.Join(days, ",")
	}
	if trig == "interval" {
		return fmt.Sprintf("every %s min%s", str(m, "intervalMinutes"), d)
	}
	t := str(m, "scheduleTime", "time")
	if t == "" {
		return "on schedule"
	}
	return "daily " + t + d
}

func toSet(m map[string]interface{}) Set {
	typ := str(m, "backupType")
	if typ == "directory" || typ == "" {
		typ = "folder"
	} else if typ == "machine" {
		typ = "machine"
	}
	src := strList(m, "backupDirs")
	if len(src) == 0 {
		src = strList(m, "driveLetters")
	}
	mode := str(m, "scheduleMode")
	return Set{
		ID: str(m, "id"), Name: str(m, "name"), Type: typ, BackupID: str(m, "backupId"),
		ServerID: str(m, "pbsServerId"), Enabled: boolean(m, "enabled"),
		Manual:  mode == "manual" || str(m, "triggerMode") == "manual",
		Schedule: scheduleText(m), LastRun: parseTime(str(m, "lastRun")), NextRun: parseTime(str(m, "nextRun")),
		Sources: src, Raw: m,
	}
}

func toHistory(m map[string]interface{}) HistoryEntry {
	return HistoryEntry{
		ID: str(m, "id"), Name: str(m, "name"), Status: str(m, "status"), Message: str(m, "message"),
		BackupID: str(m, "backupId"), At: parseTime(str(m, "timestamp")), Sources: strList(m, "backupDirs"),
		Encryption: str(m, "encryption"),
	}
}

func toMessage(m map[string]interface{}) MessageEntry {
	return MessageEntry{
		At: parseTime(str(m, "timestamp", "time", "at")), Level: str(m, "level", "type", "severity"),
		Message: str(m, "message", "text", "msg"), Source: str(m, "source", "category"),
	}
}
