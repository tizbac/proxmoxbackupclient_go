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
	// SaveSet stores a Backup Set document (the service's own checks apply):
	// a new set when isNew, otherwise a change of the set with that id.
	SaveSet(doc map[string]interface{}, isNew bool) error
	DeleteSet(id string) error
	History() ([]HistoryEntry, error)
	Messages() ([]MessageEntry, error)
	Servers() ([]Server, error)
	// RunLog returns the log lines of the run of backupID at the given time;
	// hideChunks leaves out the "New chunk" and "Reuse chunk" lines.
	RunLog(backupID string, at time.Time, hideChunks bool) ([]string, error)
	// Restore: the backup groups of a server, a group's snapshots, a snapshot's
	// file catalog, and the extraction of ticked paths (catalog paths; none
	// means everything) into dest. serverID "" is the default server.
	RestoreGroups(serverID string) ([]Group, error)
	RestoreSnapshots(serverID string, g Group) ([]SnapshotInfo, error)
	RestoreCatalog(serverID string, s SnapshotInfo) ([]CatalogItem, error)
	RestoreFiles(serverID string, s SnapshotInfo, includes []string, dest string, overwrite bool, progress func(file string)) (int, error)
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
	Default                                          bool
	Login                                            string
	Encrypted                                        bool
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

// scheduleText summarises when a set runs, from the fields the service sends:
// the Schedules list (gui/schedules.go), or the older single-trigger fields.
func scheduleText(m map[string]interface{}) string {
	if str(m, "scheduleMode") == "manual" || (str(m, "scheduleMode") == "" && str(m, "triggerMode") == "manual") {
		return "manual"
	}
	var parts []string
	if arr, ok := m["schedules"].([]interface{}); ok {
		for _, s := range arr {
			sm, _ := s.(map[string]interface{})
			if sm == nil || sm["enabled"] == false {
				continue
			}
			parts = append(parts, oneSchedule(sm))
		}
		if len(arr) > 0 && len(parts) == 0 {
			return "all schedules off"
		}
	}
	if len(parts) == 0 {
		parts = append(parts, legacySchedule(m))
	}
	return strings.Join(parts, ", ")
}

func plural(n int, unit string) string {
	if n <= 1 {
		return unit
	}
	return fmt.Sprintf("%d %ss", n, unit)
}

func every(m map[string]interface{}) int {
	n, _ := m["every"].(float64)
	if n < 1 {
		return 1
	}
	return int(n)
}

// oneSchedule is one entry of the Schedules list in words.
func oneSchedule(m map[string]interface{}) string {
	n := every(m)
	at := str(m, "at")
	if at != "" {
		at = " " + at
	}
	days := strList(m, "weekdays")
	onDays := ""
	if len(days) > 0 && len(days) < 7 {
		onDays = " " + strings.Join(days, ",")
	}
	window := ""
	if s, e := str(m, "windowStart"), str(m, "windowEnd"); s != "" && e != "" {
		window = " " + s + "-" + e
	}
	switch str(m, "unit") {
	case "minutes":
		return fmt.Sprintf("every %s%s%s", plural(n, "min"), window, onDays)
	case "hours":
		if window == "" {
			return fmt.Sprintf("every %s at :%02.0f%s", plural(n, "hour"), num(m, "minute"), onDays)
		}
		return fmt.Sprintf("every %s%s%s", plural(n, "hour"), window, onDays)
	case "days":
		if n == 1 && onDays == "" {
			return "daily" + at
		}
		if n == 1 {
			return strings.TrimSpace(onDays) + at
		}
		return fmt.Sprintf("every %s%s%s", plural(n, "day"), at, onDays)
	case "weeks":
		if n == 1 {
			return "weekly" + onDays + at
		}
		return fmt.Sprintf("every %s%s%s", plural(n, "week"), onDays, at)
	case "months":
		switch str(m, "monthMode") {
		case "last":
			return "monthly, last day" + at
		case "nth":
			return fmt.Sprintf("monthly, nth %s%s", str(m, "nthWeekday"), at)
		}
		return fmt.Sprintf("monthly, day %s%s", str(m, "monthDay"), at)
	case "years":
		return fmt.Sprintf("yearly, %s/%s%s", str(m, "yearDay"), str(m, "yearMonth"), at)
	}
	return "on schedule"
}

// legacySchedule reads the single-trigger fields of a set saved before the
// Schedules list.
func legacySchedule(m map[string]interface{}) string {
	days := strList(m, "daysOfWeek")
	d := ""
	if len(days) > 0 && len(days) < 7 {
		d = " " + strings.Join(days, ",")
	}
	switch str(m, "triggerMode") {
	case "interval":
		return fmt.Sprintf("every %s min%s", str(m, "intervalMinutes"), d)
	case "manual":
		return "manual"
	}
	if t := str(m, "scheduleTime"); t != "" {
		return "daily " + t + d
	}
	return "no schedule"
}

func num(m map[string]interface{}, k string) float64 {
	f, _ := m[k].(float64)
	return f
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
	sched := scheduleText(m)
	return Set{
		ID: str(m, "id"), Name: str(m, "name"), Type: typ, BackupID: str(m, "backupId"),
		ServerID: str(m, "pbsServerId"), Enabled: boolean(m, "enabled"),
		Manual:   sched == "manual",
		Schedule: sched, LastRun: parseTime(str(m, "lastRun")), NextRun: parseTime(str(m, "nextRun")),
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
