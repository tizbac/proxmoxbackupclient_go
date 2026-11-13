package api

import (
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// fakeHandler is a service App: it records the backups it is asked for and
// answers Pause, Resume and Stop.
type fakeHandler struct {
	mu          sync.Mutex
	machineDevs []string
	dirRuns     int
	backupIDs   []string
	runKeys     []string
	tracked     map[string]string // run key -> job ID
	paused      bool
	pausedJobs  map[string]bool
	stopped     []string // the job each Stop named ("" = all)
	stopErr     error
	diskIDs     map[string][]string        // run key -> disk identities (SetRunDiskIDs)
	skips       map[string][]PartitionSkip // run key -> partitions left out (SetRunSkipPartitions)
}

func (f *fakeHandler) SetRunSkipPartitions(runKey string, skips []PartitionSkip) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.skips == nil {
		f.skips = map[string][]PartitionSkip{}
	}
	f.skips[runKey] = skips
}

func (f *fakeHandler) SetRunDiskIDs(runKey string, diskIDs []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.diskIDs == nil {
		f.diskIDs = map[string][]string{}
	}
	f.diskIDs[runKey] = diskIDs
}

func (f *fakeHandler) StartBackup(backupType string, backupDirs, driveLetters, excludeList []string, backupID string, useVSS bool, compression, pbsServerID, comment, postActionsKey string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.dirRuns++
	f.backupIDs = append(f.backupIDs, backupID)
	f.runKeys = append(f.runKeys, postActionsKey)
	return nil
}
func (f *fakeHandler) StartMachineBackup(backupType string, backupDevices []string, backupID string, useVSS bool, compression, pbsServerID, comment, postActionsKey string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.machineDevs = append(f.machineDevs, backupDevices...)
	f.runKeys = append(f.runKeys, postActionsKey)
	return nil
}
func (f *fakeHandler) GetConfigWithHostname() map[string]interface{}          { return map[string]interface{}{} }
func (f *fakeHandler) GetScheduledJobsForAPI() []map[string]interface{}       { return nil }
func (f *fakeHandler) SaveScheduledJobFromMap(map[string]interface{}) error   { return nil }
func (f *fakeHandler) UpdateScheduledJobFromMap(map[string]interface{}) error { return nil }
func (f *fakeHandler) DeleteScheduledJobFromMap(string) error                 { return nil }
func (f *fakeHandler) PinServerFingerprint(string, string) error              { return nil }

func (f *fakeHandler) TrackRequestRun(jobID, backupType, changeDetectionMode string, readFreeSpace bool) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	key := "key-" + jobID
	if f.tracked == nil {
		f.tracked = map[string]string{}
	}
	f.tracked[key] = jobID
	return key
}

// Pause and Resume with no job act on every run (f.paused); with one, on
// that run only (f.pausedJobs).
func (f *fakeHandler) PauseRun(jobID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if jobID == "" {
		f.paused = true
		return nil
	}
	if f.pausedJobs == nil {
		f.pausedJobs = map[string]bool{}
	}
	f.pausedJobs[jobID] = true
	return nil
}
func (f *fakeHandler) ResumeRun(jobID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if jobID == "" {
		f.paused = false
		f.pausedJobs = nil
		return nil
	}
	delete(f.pausedJobs, jobID)
	return nil
}
func (f *fakeHandler) StopRun(jobID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopped = append(f.stopped, jobID)
	return f.stopErr
}
func (f *fakeHandler) RunPauseState(jobID string) (bool, int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.paused || f.pausedJobs[jobID] {
		return true, 1700000000000
	}
	return false, 0
}

// newTestService serves a Server with fake behind the real auth, and returns
// a Client that talks to it with the token.
func newTestService(t *testing.T, fake *fakeHandler) (*Server, *Client) {
	t.Helper()
	return newTestServiceWith(t, fake)
}

// newTestServiceWith is newTestService for any service App.
func newTestServiceWith(t *testing.T, handler BackupHandler) (*Server, *Client) {
	t.Helper()
	tokenPath := filepath.Join(t.TempDir(), "api-token")
	tok, err := EnsureToken(tokenPath)
	if err != nil {
		t.Fatal(err)
	}
	s := NewServer("127.0.0.1:0", handler, tok, "test")
	hs := httptest.NewServer(s.authMiddleware(s.mux))
	t.Cleanup(hs.Close)
	c := NewClient(tokenPath)
	c.baseURL = hs.URL
	return s, c
}

func findRun(t *testing.T, c *Client, id string) BackupProgress {
	t.Helper()
	runs, err := c.GetRuns()
	if err != nil {
		t.Fatalf("GetRuns: %v", err)
	}
	for _, r := range runs.Runs {
		if r.JobID == id {
			return r
		}
	}
	t.Fatalf("run %s not listed in %+v", id, runs.Runs)
	return BackupProgress{}
}

// A run the service starts on its schedule is listed with its set's name and
// progress, and keeps the first outcome reported for it.
func TestScheduledRunListedAndFinishedOnce(t *testing.T) {
	s, c := newTestService(t, &fakeHandler{})
	id := s.BeginRun("Fileserver - Data")
	s.ReportProgress(id, 42, "Backing up F:\\Data")

	r := findRun(t, c, id)
	if !r.Running || !r.Scheduled || r.Name != "Fileserver - Data" || r.Progress != 42 || r.Message != "Backing up F:\\Data" {
		t.Fatalf("listed as %+v", r)
	}
	st, err := c.GetStatus()
	if err != nil || st.ActiveJobs != 1 {
		t.Fatalf("status active jobs = %v, %v; want 1", st, err)
	}

	s.FinishRun(id, false, "Backup cancelled")
	s.FinishRun(id, true, "Backup completed") // the scheduler's own late close
	r = findRun(t, c, id)
	if r.Running || !r.Complete || r.Success || r.Message != "Backup cancelled" {
		t.Fatalf("after finish: %+v", r)
	}
	s.ReportProgress(id, 99, "late") // ignored once complete
	if r = findRun(t, c, id); r.Progress == 99 {
		t.Fatalf("progress changed after completion: %+v", r)
	}
}

// A run's live statistics and current file reach a GUI through both the run
// list and the run's status, and stop changing once it is complete.
func TestRunStatsListed(t *testing.T) {
	s, c := newTestService(t, &fakeHandler{})
	id := s.BeginRun("Set")
	s.ReportStats(id, map[string]interface{}{"filesDone": 1200, "newChunks": 7, "phase": "data"})
	s.ReportFile(id, `F:\Data\a.txt`)

	r := findRun(t, c, id)
	if r.Stats["filesDone"] != float64(1200) || r.Stats["newChunks"] != float64(7) || r.Stats["phase"] != "data" || r.File != `F:\Data\a.txt` {
		t.Fatalf("listed as stats %v, file %q", r.Stats, r.File)
	}
	p, err := c.GetBackupStatus(id)
	if err != nil || p.Stats["filesDone"] != float64(1200) || p.File != `F:\Data\a.txt` {
		t.Fatalf("status = %+v, %v", p, err)
	}

	s.FinishRun(id, true, "done")
	s.ReportStats(id, map[string]interface{}{"filesDone": 1})
	if r = findRun(t, c, id); r.Stats["filesDone"] != float64(1200) {
		t.Fatalf("stats changed after completion: %v", r.Stats)
	}
}

// Pause, Resume and Stop reach the service's App; the pause shows on the
// running run, and an App error comes back as the client's error.
func TestRunControl(t *testing.T) {
	fake := &fakeHandler{}
	s, c := newTestService(t, fake)
	id := s.BeginRun("Set")

	st, err := c.ControlRun("pause", "")
	if err != nil || !st.Paused || st.PausedSince != 1700000000000 {
		t.Fatalf("pause = %+v, %v", st, err)
	}
	if r := findRun(t, c, id); !r.Paused || r.PausedSince != 1700000000000 {
		t.Fatalf("paused run listed as %+v", r)
	}
	if st, err = c.ControlRun("resume", ""); err != nil || st.Paused {
		t.Fatalf("resume = %+v, %v", st, err)
	}
	if r := findRun(t, c, id); r.Paused {
		t.Fatalf("resumed run still paused: %+v", r)
	}

	fake.stopErr = errors.New("no backup is running")
	if _, err = c.ControlRun("stop", ""); err == nil || !strings.Contains(err.Error(), "no backup is running") {
		t.Fatalf("stop error = %v", err)
	}
	fake.stopErr = nil
	if _, err = c.ControlRun("stop", ""); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if _, err = c.ControlRun("explode", ""); err == nil {
		t.Fatal("unknown action accepted")
	}

	// A finished run is never shown as paused.
	fake.paused = true
	s.FinishRun(id, true, "done")
	if r := findRun(t, c, id); r.Paused {
		t.Fatalf("finished run shown paused: %+v", r)
	}
}

// With several runs at once, Pause and Stop name the run: only that one is
// paused or stopped, and each run lists its own pause state.
func TestRunControlPerRun(t *testing.T) {
	fake := &fakeHandler{}
	s, c := newTestService(t, fake)
	music := s.BeginRun("Music first backup")
	data := s.BeginRun("Data")

	st, err := c.ControlRun("pause", music)
	if err != nil || !st.Paused {
		t.Fatalf("pause music = %+v, %v", st, err)
	}
	if r := findRun(t, c, music); !r.Paused {
		t.Fatalf("music not shown paused: %+v", r)
	}
	if r := findRun(t, c, data); r.Paused {
		t.Fatalf("data shown paused although only music was: %+v", r)
	}
	if _, err := c.ControlRun("stop", data); err != nil {
		t.Fatalf("stop data: %v", err)
	}
	if len(fake.stopped) != 1 || fake.stopped[0] != data {
		t.Fatalf("stop reached the service as %q, want %q", fake.stopped, data)
	}
	if st, err = c.ControlRun("resume", music); err != nil || st.Paused {
		t.Fatalf("resume music = %+v, %v", st, err)
	}
}

// A run the GUI asks for is listed with its name and backup-id, so a window
// showing several runs can tell them apart.
func TestRequestedRunNamed(t *testing.T) {
	fake := &fakeHandler{}
	s, c := newTestService(t, fake)
	resp, err := c.StartBackup(&BackupRequest{BackupType: "directory", BackupID: "fileserver-data", BackupDirs: []string{`F:\Data`}, Comment: "Fileserver - Data"})
	if err != nil {
		t.Fatal(err)
	}
	r := findRun(t, c, resp.JobID)
	if r.Name != "Fileserver - Data" || r.BackupID != "fileserver-data" {
		t.Fatalf("listed as name %q, backup-id %q", r.Name, r.BackupID)
	}
	_ = s
}

// A run asked for with a compression level nobody knows is refused, for a
// folder backup and a machine backup alike, and never reaches the service's
// App: it used to run at "fastest" without a word. The levels, the other
// spellings the service has always taken, and no level at all still start.
func TestUnknownCompressionRefused(t *testing.T) {
	fake := &fakeHandler{}
	_, c := newTestService(t, fake)
	for _, bad := range []string{"ultra", "zstd", "9", " best"} {
		req := &BackupRequest{BackupType: "directory", BackupID: "id", BackupDirs: []string{`C:\Data`}, Compression: bad}
		if _, err := c.StartBackup(req); err == nil || !strings.Contains(err.Error(), "use none, fastest, default, better or best") {
			t.Errorf("StartBackup with compression %q = %v, want it refused", bad, err)
		}
		if _, err := c.StartMachineBackup(req); err == nil || !strings.Contains(err.Error(), "use none, fastest, default, better or best") {
			t.Errorf("StartMachineBackup with compression %q = %v, want it refused", bad, err)
		}
	}
	runs, err := c.GetRuns()
	if err != nil || len(runs.Runs) != 0 {
		t.Fatalf("a refused request left a run listed: %+v, %v", runs, err)
	}
	good := []string{"", "none", "fastest", "default", "better", "best", "Best", "off", "0", "fast", "max"}
	for _, ok := range good {
		if _, err := c.StartBackup(&BackupRequest{BackupType: "directory", BackupID: "id", BackupDirs: []string{`C:\Data`}, Compression: ok}); err != nil {
			t.Errorf("StartBackup with compression %q refused: %v", ok, err)
		}
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		fake.mu.Lock()
		n := fake.dirRuns
		fake.mu.Unlock()
		if n == len(good) {
			return
		}
		if n > len(good) || time.Now().After(deadline) {
			t.Fatalf("%d folder backups started, want %d (the refused ones must not)", n, len(good))
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// Machine backups the GUI asks for reach StartMachineBackup (the route was
// missing: the request got a 404), with the disks and a tracked run key.
// A Backup Set without a backup ID (one the installer set up, or one whose
// ID was cleared) runs as the computer's name when the app hands it to the
// service, as it does everywhere else; it was refused with
// "backup_id is required".
func TestEmptyBackupIDIsComputerName(t *testing.T) {
	fake := &fakeHandler{}
	_, c := newTestService(t, fake)
	resp, err := c.StartBackup(&BackupRequest{BackupType: "directory", BackupDirs: []string{`C:\Data`}})
	if err != nil || resp.JobID == "" {
		t.Fatalf("StartBackup with no backup ID = %+v, %v", resp, err)
	}
	host, _ := os.Hostname()
	deadline := time.Now().Add(5 * time.Second)
	for {
		fake.mu.Lock()
		ids := append([]string(nil), fake.backupIDs...)
		fake.mu.Unlock()
		if len(ids) == 1 {
			if ids[0] != host {
				t.Fatalf("backup ID %q, want the computer's name %q", ids[0], host)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("backup not started: %v", ids)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestMachineBackupRoute(t *testing.T) {
	fake := &fakeHandler{}
	_, c := newTestService(t, fake)
	resp, err := c.StartMachineBackup(&BackupRequest{BackupType: "machine", BackupID: "fileserver", DriveLetters: []string{`\\.\PhysicalDrive0`}})
	if err != nil || resp.JobID == "" {
		t.Fatalf("StartMachineBackup = %+v, %v", resp, err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		p, err := c.GetBackupStatus(resp.JobID)
		if err == nil && p.Complete {
			if !p.Success {
				t.Fatalf("run ended %+v", p)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("run %s never completed (%+v, %v)", resp.JobID, p, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.machineDevs) != 1 || fake.machineDevs[0] != `\\.\PhysicalDrive0` || fake.dirRuns != 0 {
		t.Fatalf("machine devices %v, directory runs %d", fake.machineDevs, fake.dirRuns)
	}
	if len(fake.runKeys) != 1 || fake.tracked[fake.runKeys[0]] != resp.JobID {
		t.Fatalf("run key %v not tracked to %s (%v)", fake.runKeys, resp.JobID, fake.tracked)
	}
	if len(fake.diskIDs) != 0 {
		t.Fatalf("disk identities recorded for a request without any: %v", fake.diskIDs)
	}
}

// A machine Backup Set run the GUI hands over carries its disks' identities,
// and the partitions it leaves out, to the run before the backup starts.
func TestMachineBackupCarriesDiskIDs(t *testing.T) {
	fake := &fakeHandler{}
	_, c := newTestService(t, fake)
	skip := PartitionSkip{DiskID: "serial:AAA", Device: `\\.\PhysicalDrive0`, Start: 1 << 20, Size: 100 << 20, Number: 1, Label: "partition 1"}
	resp, err := c.StartMachineBackup(&BackupRequest{BackupType: "machine-vm", BackupID: "101",
		DriveLetters: []string{`\\.\PhysicalDrive0`, `\\.\PhysicalDrive1`}, DiskIDs: []string{"serial:AAA", ""},
		SkipPartitions: []PartitionSkip{skip}})
	if err != nil || resp.JobID == "" {
		t.Fatalf("StartMachineBackup = %+v, %v", resp, err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		fake.mu.Lock()
		keys := append([]string(nil), fake.runKeys...)
		ids := fake.diskIDs
		skips := fake.skips
		fake.mu.Unlock()
		if len(keys) == 1 {
			if got := ids["key-"+resp.JobID]; len(got) != 2 || got[0] != "serial:AAA" || got[1] != "" {
				t.Fatalf("disk identities on the run: %v", ids)
			}
			if got := skips["key-"+resp.JobID]; len(got) != 1 || got[0] != skip {
				t.Fatalf("partitions left out on the run: %+v", skips)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("machine backup not started: %v", keys)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
