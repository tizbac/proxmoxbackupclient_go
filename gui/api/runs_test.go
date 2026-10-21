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
	stopErr     error
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

func (f *fakeHandler) PauseRun() error  { f.mu.Lock(); f.paused = true; f.mu.Unlock(); return nil }
func (f *fakeHandler) ResumeRun() error { f.mu.Lock(); f.paused = false; f.mu.Unlock(); return nil }
func (f *fakeHandler) StopRun() error   { f.mu.Lock(); defer f.mu.Unlock(); return f.stopErr }
func (f *fakeHandler) RunPauseState() (bool, int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.paused {
		return true, 1700000000000
	}
	return false, 0
}

// newTestService serves a Server with fake behind the real auth, and returns
// a Client that talks to it with the token.
func newTestService(t *testing.T, fake *fakeHandler) (*Server, *Client) {
	t.Helper()
	tokenPath := filepath.Join(t.TempDir(), "api-token")
	tok, err := NewToken(tokenPath)
	if err != nil {
		t.Fatal(err)
	}
	s := NewServer("127.0.0.1:0", fake, tok, "test")
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
	id := s.BeginRun("Deepthought - Data")
	s.ReportProgress(id, 42, "Backing up F:\\Data")

	r := findRun(t, c, id)
	if !r.Running || !r.Scheduled || r.Name != "Deepthought - Data" || r.Progress != 42 || r.Message != "Backing up F:\\Data" {
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

	st, err := c.ControlRun("pause")
	if err != nil || !st.Paused || st.PausedSince != 1700000000000 {
		t.Fatalf("pause = %+v, %v", st, err)
	}
	if r := findRun(t, c, id); !r.Paused || r.PausedSince != 1700000000000 {
		t.Fatalf("paused run listed as %+v", r)
	}
	if st, err = c.ControlRun("resume"); err != nil || st.Paused {
		t.Fatalf("resume = %+v, %v", st, err)
	}
	if r := findRun(t, c, id); r.Paused {
		t.Fatalf("resumed run still paused: %+v", r)
	}

	fake.stopErr = errors.New("no backup is running")
	if _, err = c.ControlRun("stop"); err == nil || !strings.Contains(err.Error(), "no backup is running") {
		t.Fatalf("stop error = %v", err)
	}
	fake.stopErr = nil
	if _, err = c.ControlRun("stop"); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if _, err = c.ControlRun("explode"); err == nil {
		t.Fatal("unknown action accepted")
	}

	// A finished run is never shown as paused.
	fake.paused = true
	s.FinishRun(id, true, "done")
	if r := findRun(t, c, id); r.Paused {
		t.Fatalf("finished run shown paused: %+v", r)
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
	resp, err := c.StartMachineBackup(&BackupRequest{BackupType: "machine", BackupID: "deepthought", DriveLetters: []string{`\\.\PhysicalDrive0`}})
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
}
