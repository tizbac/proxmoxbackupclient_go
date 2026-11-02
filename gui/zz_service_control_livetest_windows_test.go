//go:build windows && !service

package main

import (
	"os"
	"testing"

	"github.com/tizbac/proxmoxbackupclient_go/gui/api"
)

// Opt-in (PBS_SVCTEST=1), run elevated on a test machine with the test binary
// named ProxmoxBackupClient.exe and ProxmoxBackupClientSVC.exe beside it:
// install and start the service, see this app hand over to it, stop, start
// and remove it, leaving the machine as it was.
func TestServiceControlLive(t *testing.T) {
	if os.Getenv("PBS_SVCTEST") == "" {
		t.Skip("set PBS_SVCTEST=1")
	}
	a := &App{apiClient: api.NewClient(getAPITokenPath()), mode: api.ModeStandalone, stopScheduler: make(chan struct{})}
	defer a.StopScheduler()
	st := a.GetServiceStatus()
	t.Logf("before: %+v", st)
	if st.Installed {
		t.Skip("a service is already installed here; not touching it")
	}
	if !st.ExeAvailable || !st.IsAdmin {
		t.Fatalf("needs %s and an administrator: %+v", st.ExpectedExe, st)
	}
	st, err := a.InstallService()
	if err != nil || !st.Installed || st.State != "running" || st.StartType != "automatic" || st.OtherExe {
		t.Fatalf("install: %v %+v", err, st)
	}
	if a.mode != api.ModeService || !st.ServiceMode {
		t.Errorf("after install the app should hand over to the service: mode %v %+v", a.mode, st)
	}
	st, err = a.StopBackgroundService()
	if err != nil || st.State != "stopped" || a.mode != api.ModeStandalone {
		t.Errorf("stop: %v %+v mode %v", err, st, a.mode)
	}
	st, err = a.StartBackgroundService()
	if err != nil || st.State != "running" || a.mode != api.ModeService {
		t.Errorf("start: %v %+v mode %v", err, st, a.mode)
	}
	st, err = a.RemoveService()
	if err != nil || st.Installed || a.mode != api.ModeStandalone {
		t.Fatalf("remove: %v %+v mode %v", err, st, a.mode)
	}
	t.Logf("after: %+v", st)
}
