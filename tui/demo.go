package tui

import (
	"fmt"
	"sync"
	"time"

	"github.com/salvanza/salvanza_client/gui/api"
)

// demoBackend invents a small, living installation: sets, a running backup
// that moves on, history, messages and servers. It lets the screens be tried,
// screenshotted and tested without a service (salvanza-cli tui -demo).
type demoBackend struct {
	mu     sync.Mutex
	sets   []Set
	runs   []api.BackupProgress
	hist   []HistoryEntry
	msgs   []MessageEntry
	start  time.Time
	nextID int
}

func newDemoBackend() *demoBackend {
	now := time.Now()
	at := func(h, m int, dayOffset int) time.Time {
		return time.Date(now.Year(), now.Month(), now.Day()+dayOffset, h, m, 0, 0, now.Location())
	}
	d := &demoBackend{start: now}
	d.sets = []Set{
		{ID: "s1", Name: "Documents", Type: "folder", BackupID: "DESKTOP-01", Enabled: true, Schedule: "daily 18:00",
			LastRun: at(2, 10, 0), NextRun: at(18, 0, 0), Sources: []string{`C:\Users\mick\Documents`, `C:\Users\mick\Desktop`}, ServerID: "main"},
		{ID: "s2", Name: "Machine image", Type: "machine", BackupID: "DESKTOP-01-disk", Enabled: true, Schedule: "daily 01:00 Sun",
			LastRun: at(1, 7, -1), NextRun: at(1, 0, 6), Sources: []string{`\\.\PhysicalDrive0`}, ServerID: "main"},
		{ID: "s3", Name: "Photos", Type: "folder", BackupID: "DESKTOP-01-photos", Enabled: true, Schedule: "daily 02:00",
			LastRun: at(2, 0, -1), NextRun: at(2, 0, 1), Sources: []string{`D:\Photos`}, ServerID: "offsite"},
		{ID: "s4", Name: "Web server", Type: "folder", BackupID: "DESKTOP-01-web", Enabled: false, Schedule: "manual", Manual: true,
			LastRun: at(3, 0, -6), Sources: []string{`C:\inetpub`}, ServerID: "main"},
	}
	d.hist = []HistoryEntry{
		{ID: "h1", Name: "Photos", Status: "failed", Message: "2 files could not be read: in use", At: at(2, 0, -1), BackupID: "DESKTOP-01-photos", Encryption: "encrypted"},
		{ID: "h2", Name: "Machine image", Status: "success", Message: "1 snapshot, 38.2 GB, 91% of chunks already on the server", At: at(1, 7, -1), BackupID: "DESKTOP-01-disk", Encryption: "encrypted"},
		{ID: "h3", Name: "Documents", Status: "success", Message: "412 MB in 3,204 files, 18 changed", At: at(2, 10, 0), BackupID: "DESKTOP-01", Encryption: "encrypted"},
		{ID: "h4", Name: "Documents", Status: "success", Message: "409 MB in 3,198 files, 6 changed", At: at(18, 0, -1), BackupID: "DESKTOP-01", Encryption: "encrypted"},
		{ID: "h5", Name: "Web server", Status: "failed", Message: "no route to host: pbs.example.com", At: at(3, 0, -6), BackupID: "DESKTOP-01-web"},
	}
	d.msgs = []MessageEntry{
		{At: at(2, 10, 0), Level: "info", Message: "Documents: finished, 18 files changed"},
		{At: at(2, 0, -1), Level: "warning", Message: "Photos: 2 files skipped, in use"},
		{At: at(1, 7, -1), Level: "info", Message: "Machine image: snapshot uploaded"},
		{At: at(3, 0, -6), Level: "error", Message: "Web server: no route to host"},
	}
	return d
}

func (d *demoBackend) Name() string { return "demo" }

func (d *demoBackend) Status() (*api.StatusResponse, error) {
	d.tick()
	d.mu.Lock()
	defer d.mu.Unlock()
	active := 0
	for _, r := range d.runs {
		if r.Running {
			active++
		}
	}
	return &api.StatusResponse{Running: true, Version: "2.0.0", ActiveJobs: active}, nil
}

func (d *demoBackend) Sets() ([]Set, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]Set(nil), d.sets...), nil
}

// tick moves the running demo backups on.
func (d *demoBackend) tick() {
	d.mu.Lock()
	defer d.mu.Unlock()
	for i := range d.runs {
		r := &d.runs[i]
		if !r.Running || r.Paused {
			continue
		}
		r.Progress += 3.5
		files := int(r.Progress * 32)
		r.Stats = map[string]interface{}{
			"percent": r.Progress, "filesDone": float64(files), "filesTotal": 3204.0,
			"bytesDone": float64(files) * 128000, "bytesTotal": 412e6, "sentBytes": float64(files) * 28000,
			"newChunks": float64(files / 5), "reusedChunks": float64(files),
		}
		r.File = fmt.Sprintf(`C:\Users\mick\Documents\Projects\notes-%03d.docx`, files%999)
		r.Message = "Uploading"
		if r.Progress >= 100 {
			r.Progress, r.Running, r.Complete, r.Success = 100, false, true, true
			r.Message = "Backup finished"
			d.hist = append([]HistoryEntry{{ID: r.JobID, Name: r.Name, Status: "success", Message: "demo run finished", At: time.Now(), Encryption: "encrypted"}}, d.hist...)
		}
	}
}

func (d *demoBackend) Runs() ([]api.BackupProgress, error) {
	d.tick()
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]api.BackupProgress(nil), d.runs...), nil
}

func (d *demoBackend) RunSet(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, s := range d.sets {
		if s.ID != id {
			continue
		}
		for _, r := range d.runs {
			if r.Name == s.Name && r.Running {
				return fmt.Errorf("%s is running already", s.Name)
			}
		}
		d.nextID++
		d.runs = append(d.runs, api.BackupProgress{
			JobID: fmt.Sprintf("demo-%d", d.nextID), Name: s.Name, Running: true, Progress: 1,
			StartTime: time.Now().Format(time.RFC3339), Message: "Starting",
		})
		return nil
	}
	return fmt.Errorf("no such set")
}

func (d *demoBackend) Control(action, jobID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	for i := range d.runs {
		r := &d.runs[i]
		if r.JobID != jobID || !r.Running {
			continue
		}
		switch action {
		case "pause":
			r.Paused = true
		case "resume":
			r.Paused = false
		case "stop":
			r.Running, r.Complete, r.Success, r.Error = false, true, false, "stopped by the user"
		}
		return nil
	}
	return fmt.Errorf("no backup is running")
}

func (d *demoBackend) SetEnabled(id string, on bool) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	for i := range d.sets {
		if d.sets[i].ID == id {
			d.sets[i].Enabled = on
			return nil
		}
	}
	return fmt.Errorf("no such set")
}

func (d *demoBackend) History() ([]HistoryEntry, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]HistoryEntry(nil), d.hist...), nil
}

func (d *demoBackend) Messages() ([]MessageEntry, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]MessageEntry(nil), d.msgs...), nil
}

func (d *demoBackend) Servers() ([]Server, error) {
	return []Server{
		{ID: "main", Name: "Main PBS", URL: "https://pbs.example.com:8007", Datastore: "backup", Namespace: "desktop", Fingerprint: "07:49:87:ae:76:0f:a8:e7:97:d8:1c:f0:44:1a:65:5d:2c:f3:10:49:00:ea:2c:53:e4:0b:cc:09:95:29:c5:22", Default: true, Login: "token", Encrypted: true},
		{ID: "offsite", Name: "Offsite PBS", URL: "https://offsite.example.net:8007", Datastore: "photos", Login: "token", Encrypted: true},
	}, nil
}
