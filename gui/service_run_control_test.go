package main

import (
	"errors"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"

	"github.com/tizbac/proxmoxbackupclient_go/gui/api"
)

// fakeService stands in for the service's App behind the local API: it
// records Pause, Resume and Stop.
type fakeService struct {
	mu      sync.Mutex
	paused  bool
	stopped int
}

func (f *fakeService) StartBackup(string, []string, []string, []string, string, bool, string, string, string, string) error {
	return nil
}
func (f *fakeService) StartMachineBackup(string, []string, string, bool, string, string, string, string) error {
	return nil
}
func (f *fakeService) GetConfigWithHostname() map[string]interface{}          { return map[string]interface{}{} }
func (f *fakeService) GetScheduledJobsForAPI() []map[string]interface{}       { return nil }
func (f *fakeService) SaveScheduledJobFromMap(map[string]interface{}) error   { return nil }
func (f *fakeService) UpdateScheduledJobFromMap(map[string]interface{}) error { return nil }
func (f *fakeService) DeleteScheduledJobFromMap(string) error                 { return nil }
func (f *fakeService) PinServerFingerprint(string, string) error              { return nil }
func (f *fakeService) PauseRun() error                                        { f.mu.Lock(); f.paused = true; f.mu.Unlock(); return nil }
func (f *fakeService) ResumeRun() error                                       { f.mu.Lock(); f.paused = false; f.mu.Unlock(); return nil }
func (f *fakeService) StopRun() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.stopped > 0 {
		return errors.New("no backup is running")
	}
	f.stopped++
	return nil
}
func (f *fakeService) RunPauseState() (bool, int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.paused {
		return true, 1700000000000
	}
	return false, 0
}

// With Run as Service, the progress card's Pause, Resume and Stop act on the
// service's run (this window runs no backup of its own).
func TestPauseResumeStopGoToTheService(t *testing.T) {
	tokenPath := filepath.Join(t.TempDir(), "api-token")
	tok, err := api.NewToken(tokenPath)
	if err != nil {
		t.Fatal(err)
	}
	svc := &fakeService{}
	server := api.NewServer("127.0.0.1:0", svc, tok, "test")
	hs := httptest.NewServer(server.Handler())
	defer hs.Close()
	runID := server.BeginRun("Deepthought - Data")

	a := &App{mode: api.ModeService, apiClient: api.NewClientAt(hs.URL, tokenPath)}

	st, err := a.PauseBackup()
	if err != nil || !st.Paused || st.Since != 1700000000000 || !svc.paused {
		t.Fatalf("PauseBackup = %+v, %v (service paused %v)", st, err, svc.paused)
	}
	if got := a.GetBackupPauseState(); !got.Paused {
		t.Fatalf("GetBackupPauseState = %+v, want paused (a reloaded window)", got)
	}
	if st, err = a.ResumeBackup(); err != nil || st.Paused || svc.paused {
		t.Fatalf("ResumeBackup = %+v, %v (service paused %v)", st, err, svc.paused)
	}
	if err := a.CancelBackup(); err != nil || svc.stopped != 1 {
		t.Fatalf("CancelBackup = %v, service stopped %d times", err, svc.stopped)
	}
	// The service's own answer comes back to the window.
	if err := a.CancelBackup(); err == nil || err.Error() != "no backup is running" {
		t.Fatalf("second CancelBackup = %v", err)
	}

	server.FinishRun(runID, false, "Backup cancelled")
	if got := a.GetBackupPauseState(); got.Paused {
		t.Fatalf("pause state after the run = %+v", got)
	}
}

// Without the service, Pause with nothing running still says so.
func TestPauseWithoutServiceOrBackup(t *testing.T) {
	a := &App{mode: api.ModeStandalone}
	if _, err := a.PauseBackup(); err == nil {
		t.Fatal("PauseBackup with no backup running gave no error")
	}
}
