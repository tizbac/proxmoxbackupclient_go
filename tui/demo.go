package tui

import (
	"fmt"
	"strings"
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

	servers  []Server
	prefs    Prefs
	prefsSet bool
}

func newDemoBackend() *demoBackend {
	now := time.Now()
	at := func(h, m int, dayOffset int) time.Time {
		return time.Date(now.Year(), now.Month(), now.Day()+dayOffset, h, m, 0, 0, now.Location())
	}
	d := &demoBackend{start: now}
	// The sets are documents as the service sends them, so the editor has real fields to work on.
	doc := func(id, name, typ, backupID, server string, enabled bool, unit, atTime string, weekdays []interface{}, dirs []interface{}, last, next time.Time) Set {
		m := map[string]interface{}{
			"id": id, "name": name, "backupType": typ, "backupId": backupID, "pbsServerId": server, "enabled": enabled,
			"lastRun": last.UTC().Format(time.RFC3339), "useVSS": true, "excludeList": []interface{}{},
		}
		if typ == "machine" {
			m["driveLetters"] = dirs
		} else {
			m["backupDirs"] = dirs
			m["changeDetectionMode"] = "legacy"
		}
		if unit == "" {
			m["scheduleMode"], m["triggerMode"] = "manual", "manual"
		} else {
			sc := map[string]interface{}{"id": "s1", "enabled": true, "unit": unit, "every": 1.0, "at": atTime}
			if weekdays != nil {
				sc["weekdays"] = weekdays
			}
			m["scheduleMode"], m["schedules"] = "scheduled", []interface{}{sc}
			m["nextRun"] = next.UTC().Format(time.RFC3339)
		}
		return toSet(m)
	}
	d.sets = []Set{
		doc("s1", "Documents", "directory", "DESKTOP-01", "main", true, "days", "18:00", nil,
			[]interface{}{`C:\Users\mick\Documents`, `C:\Users\mick\Desktop`}, at(2, 10, 0), at(18, 0, 0)),
		doc("s2", "Machine image", "machine", "DESKTOP-01-disk", "main", true, "weeks", "01:00", []interface{}{"Sun"},
			[]interface{}{`\\.\PhysicalDrive0`}, at(1, 7, -1), at(1, 0, 6)),
		doc("s3", "Photos", "directory", "DESKTOP-01-photos", "offsite", true, "days", "02:00", nil,
			[]interface{}{`D:\Photos`}, at(2, 0, -1), at(2, 0, 1)),
		doc("s4", "Web server", "directory", "DESKTOP-01-web", "main", false, "", "", nil,
			[]interface{}{`C:\inetpub`}, at(3, 0, -6), time.Time{}),
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

func (d *demoBackend) StartFolderBackup(req api.BackupRequest) error {
	if len(req.BackupDirs) == 0 {
		return fmt.Errorf("no folder to back up")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	d.nextID++
	d.runs = append(d.runs, api.BackupProgress{
		JobID: fmt.Sprintf("demo-%d", d.nextID), Name: "One-off: " + req.BackupDirs[0], Running: true, Progress: 1,
		StartTime: time.Now().Format(time.RFC3339), Message: "Starting",
	})
	return nil
}

func (d *demoBackend) SaveSet(doc map[string]interface{}, isNew bool) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	name := str(doc, "name")
	if name == "" {
		return fmt.Errorf("a Backup Set needs a name")
	}
	for _, s := range d.sets {
		if strings.EqualFold(s.Name, name) && s.ID != str(doc, "id") {
			return fmt.Errorf("a Backup Set named %q exists already", name)
		}
	}
	if !boolean(doc, "enabled") && isNew {
		doc["enabled"] = true
	}
	set := toSet(doc)
	if isNew {
		d.sets = append(d.sets, set)
		return nil
	}
	for i := range d.sets {
		if d.sets[i].ID == set.ID {
			set.LastRun = d.sets[i].LastRun
			d.sets[i] = set
			return nil
		}
	}
	return fmt.Errorf("no such Backup Set")
}

func (d *demoBackend) DeleteSet(id string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	for i := range d.sets {
		if d.sets[i].ID == id {
			d.sets = append(d.sets[:i], d.sets[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("no such Backup Set")
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

func (d *demoBackend) RunLog(backupID string, at time.Time, hideChunks bool) ([]string, error) {
	ts := at.Format("2006-01-02 15:04:05")
	lines := []string{
		ts + " Starting backup of C:\\Users\\mick\\Documents",
		ts + " Creating VSS Snapshot... Snapshot created",
		ts + " Connected to Main PBS, backup group host/" + backupID,
	}
	if !hideChunks {
		for i := 0; i < 12; i++ {
			lines = append(lines, fmt.Sprintf("%s New chunk %d (4194304 bytes)", ts, 1000+i), fmt.Sprintf("%s Reuse chunk %d", ts, 2000+i))
		}
	}
	return append(lines, ts+" File metadata: 3204 entries, 0 errors", ts+" Backup completed in 55s: 412 MB backed up (18 new, 3186 reused chunks)"), nil
}

func (d *demoBackend) RestoreGroups(serverID string) ([]Group, error) {
	now := time.Now()
	return []Group{
		{Type: "host", ID: "DESKTOP-01", Count: 14, Latest: now.Add(-9 * time.Hour)},
		{Type: "host", ID: "DESKTOP-01-photos", Count: 30, Latest: now.Add(-33 * time.Hour)},
		{Type: "host", ID: "DESKTOP-01-disk", Count: 6, Latest: now.Add(-40 * time.Hour)},
	}, nil
}

func (d *demoBackend) RestoreSnapshots(serverID string, g Group) ([]SnapshotInfo, error) {
	out := make([]SnapshotInfo, 0, 6)
	for i := 0; i < 6; i++ {
		out = append(out, SnapshotInfo{
			Type: g.Type, BackupID: g.ID, Time: g.Latest.Add(-time.Duration(i) * 24 * time.Hour),
			Archives: []string{"backup.pxar.didx"}, Size: 412e6 + int64(i)*1e6, HasCatalog: true,
		})
	}
	if g.ID == "DESKTOP-01-disk" {
		out[0].Archives = nil
	}
	if g.ID == "DESKTOP-01-photos" {
		for i := range out {
			out[i].Encrypted = true
		}
	}
	return out, nil
}

const demoPassphrase = "demo-pass"

func (d *demoBackend) RestoreCatalog(serverID string, s SnapshotInfo, passphrase string) ([]CatalogItem, error) {
	if s.Encrypted {
		if passphrase == "" {
			return nil, ErrNeedPassphrase
		}
		if passphrase != demoPassphrase {
			return nil, fmt.Errorf("the key could not be unlocked, is the passphrase right?")
		}
	}
	mod := s.Time
	a := "backup.pxar.didx"
	items := []CatalogItem{
		{Path: a, Dir: true},
		{Path: a + "/Documents", Dir: true},
		{Path: a + "/Documents/Finance", Dir: true},
		{Path: a + "/Documents/Finance/invoice-2026-10.pdf", Size: 184e3, Mod: mod},
		{Path: a + "/Documents/Finance/invoice-2026-09.pdf", Size: 176e3, Mod: mod},
		{Path: a + "/Documents/Finance/budget.xlsx", Size: 64e3, Mod: mod},
		{Path: a + "/Documents/Projects", Dir: true},
		{Path: a + "/Documents/Projects/notes-001.docx", Size: 22e3, Mod: mod},
		{Path: a + "/Documents/Projects/notes-002.docx", Size: 24e3, Mod: mod},
		{Path: a + "/Pictures", Dir: true},
		{Path: a + "/Pictures/holiday.jpg", Size: 4.2e6, Mod: mod},
		{Path: a + "/todo.txt", Size: 800, Mod: mod},
	}
	return items, nil
}

func (d *demoBackend) RestoreFiles(serverID string, s SnapshotInfo, includes []string, dest string, overwrite bool, passphrase string, progress func(file string)) (int, error) {
	if s.Encrypted && passphrase != demoPassphrase {
		return 0, ErrNeedPassphrase
	}
	n := len(includes)
	if n == 0 {
		n = 9
	}
	for i := 0; i < n; i++ {
		if progress != nil {
			progress(fmt.Sprintf("%s/file-%02d", dest, i))
		}
		time.Sleep(60 * time.Millisecond)
	}
	return n, nil
}

func demoServer(id, name, url, store, ns, fp, authID string, def bool) Server {
	raw := map[string]interface{}{
		"id": id, "name": name, "baseurl": url, "datastore": store, "namespace": ns, "certfingerprint": fp,
		"authid": authID, "secret_set": true, "encryption_key_file": "keys/key.json",
	}
	return Server{
		ID: id, Name: name, URL: url, Datastore: store, Namespace: ns, Fingerprint: fp, Default: def, Login: "token",
		Encrypted: true, AuthID: authID, Raw: raw,
	}
}

func (d *demoBackend) Servers() ([]Server, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.servers == nil {
		d.servers = []Server{
			demoServer("main", "Main PBS", "https://pbs.example.com:8007", "backup", "desktop",
				"07:49:87:AE:76:0F:A8:E7:97:D8:1C:F0:44:1A:65:5D:2C:F3:10:49:00:EA:2C:53:E4:0B:CC:09:95:29:C5:22", "backup@pbs!desktop", true),
			demoServer("offsite", "Offsite PBS", "https://offsite.example.net:8007", "photos", "", "", "backup@pbs!photos", false),
		}
	}
	return append([]Server(nil), d.servers...), nil
}

func (d *demoBackend) SaveServer(doc map[string]interface{}, isNew bool) error {
	if _, err := d.Servers(); err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	id := str(doc, "id")
	if str(doc, "name") == "" || str(doc, "baseurl") == "" || str(doc, "datastore") == "" {
		return fmt.Errorf("a server needs a name, an address and a datastore")
	}
	for i, s := range d.servers {
		if s.ID == id {
			if isNew {
				return fmt.Errorf("a server with the id %q exists already", id)
			}
			d.servers[i] = demoServer(id, str(doc, "name"), str(doc, "baseurl"), str(doc, "datastore"), str(doc, "namespace"),
				str(doc, "certfingerprint"), str(doc, "authid"), s.Default)
			d.servers[i].Raw = doc
			return nil
		}
	}
	if !isNew {
		return fmt.Errorf("no such server")
	}
	s := demoServer(id, str(doc, "name"), str(doc, "baseurl"), str(doc, "datastore"), str(doc, "namespace"),
		str(doc, "certfingerprint"), str(doc, "authid"), len(d.servers) == 0)
	s.Raw = doc
	d.servers = append(d.servers, s)
	return nil
}

func (d *demoBackend) DeleteServer(id string) error {
	if _, err := d.Servers(); err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	for i := range d.servers {
		if d.servers[i].ID == id {
			if d.servers[i].Default && len(d.servers) > 1 {
				return fmt.Errorf("this is the default server: make another one the default first")
			}
			d.servers = append(d.servers[:i], d.servers[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("no such server")
}

func (d *demoBackend) SetDefaultServer(id string) error {
	if _, err := d.Servers(); err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	found := false
	for i := range d.servers {
		d.servers[i].Default = d.servers[i].ID == id
		found = found || d.servers[i].ID == id
	}
	if !found {
		return fmt.Errorf("no such server")
	}
	return nil
}

func (d *demoBackend) TestServer(id string) error {
	time.Sleep(200 * time.Millisecond)
	if id == "offsite" {
		return fmt.Errorf("connection test failed: no route to host")
	}
	return nil
}

func (d *demoBackend) PinFingerprint(id, fingerprint string) error {
	if _, err := d.Servers(); err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	for i := range d.servers {
		if d.servers[i].ID == id {
			d.servers[i].Fingerprint = fingerprint
			return nil
		}
	}
	return fmt.Errorf("no such server")
}

func (d *demoBackend) Prefs() (Prefs, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.prefsSet {
		d.prefs = Prefs{Compression: "default", SMTPHost: "smtp.example.com", SMTPPort: "587", SMTPUser: "alerts@example.com",
			SMTPPasswordSet: true, MailFrom: "backup@example.com", MaxRuns: 2}
		d.prefsSet = true
	}
	return d.prefs, nil
}

func (d *demoBackend) SavePrefs(old, now Prefs) error {
	if now.UploadMBps < 0 || now.UploadMBps > 10000 {
		return fmt.Errorf("speed limits must be between 0 and 10000 MB/s")
	}
	if now.ChunkTimeout != 0 && (now.ChunkTimeout < 30 || now.ChunkTimeout > 1800) {
		return fmt.Errorf("the chunk timeout must be the default (0) or between 30 and 1800 seconds")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if now.SMTPPassword != "" {
		now.SMTPPasswordSet = true
	}
	now.SMTPPassword = ""
	d.prefs, d.prefsSet = now, true
	return nil
}

func (d *demoBackend) SendTestMail(to string) error {
	time.Sleep(200 * time.Millisecond)
	if !strings.Contains(to, "@") {
		return fmt.Errorf("that is not an e-mail address")
	}
	return nil
}

func (d *demoBackend) LogFiles() ([]LogFile, error) {
	now := time.Now()
	return []LogFile{
		{Name: "backup-20261109-021048-DESKTOP-01.log.gz", Kind: "run", BackupID: "DESKTOP-01", Started: now.Add(-9 * time.Hour), Modified: now.Add(-9 * time.Hour), Size: 5552, Compressed: true},
		{Name: "service.log", Kind: "service", Started: now.Add(-30 * time.Hour), Modified: now.Add(-5 * time.Minute), Size: 183000, Service: true},
		{Name: "backup-20261108-020000-DESKTOP-01-photos.log.gz", Kind: "run", BackupID: "DESKTOP-01-photos", Started: now.Add(-33 * time.Hour), Modified: now.Add(-33 * time.Hour), Size: 3563, Compressed: true},
	}, nil
}

func (d *demoBackend) ReadLogFile(f LogFile, filter string, hideChunks bool) ([]string, error) {
	ts := f.Started.Format("2006-01-02 15:04:05")
	lines := []string{ts + " Starting " + f.Name, ts + " Connected to Main PBS", ts + " warning: 2 files skipped, in use"}
	for i := 0; i < 8 && !hideChunks; i++ {
		lines = append(lines, fmt.Sprintf("%s New chunk %d", ts, i))
	}
	lines = append(lines, ts+" Backup completed in 55s")
	if filter == "" {
		return lines, nil
	}
	var out []string
	for _, l := range lines {
		if strings.Contains(strings.ToLower(l), strings.ToLower(filter)) {
			out = append(out, l)
		}
	}
	return out, nil
}
