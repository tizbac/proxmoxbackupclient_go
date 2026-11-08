//go:build linux && !service

package main

// Opt-in live tests of the Linux background service, on a test machine with
// the .deb installed (they install, start and remove the real systemd unit).
//
//   PBS_LINUX_SVC=install  (as root, HOME = the user whose settings to move):
//       Run as Service > Install: settings copied to /var/lib/salvanza,
//       which is root's alone (0700, files 0600), unit enabled and running,
//       app in service mode.
//   PBS_LINUX_SVC=user     (as an ordinary account, not root): the app finds
//       the service but cannot read its token, so it is not connected, asks
//       nobody while no window is in front of the user, and sees no Backup
//       Sets until it connects.
//   PBS_LINUX_SVC=remove   (as root): Remove stops and disables the unit and
//       keeps the settings.

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/salvanza/salvanza_client/gui/api"
)

func liveLinuxSvc(t *testing.T, step string) {
	t.Helper()
	if os.Getenv("PBS_LINUX_SVC") != step {
		t.Skip("set PBS_LINUX_SVC=" + step)
	}
	useRealSettings(t)
}

func newLiveApp() *App {
	a := &App{
		config:        LoadConfig(),
		stopScheduler: make(chan struct{}),
		apiClient:     api.NewClient(getAPITokenPath()),
		callbacksMap:  make(map[string]*progressCallbacks),
		headless:      true,
	}
	a.resolveMode()
	return a
}

func fileOwnerMode(t *testing.T, p string) (uid, gid uint32, mode os.FileMode) {
	t.Helper()
	info, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	st := info.Sys().(*syscall.Stat_t)
	return st.Uid, st.Gid, info.Mode()
}

func TestLiveLinuxServiceInstall(t *testing.T) {
	liveLinuxSvc(t, "install")
	if os.Geteuid() != 0 {
		t.Fatal("run as root")
	}
	serviceExeOverride = packagedServiceExe
	a := newLiveApp()
	st, err := a.InstallService()
	if err != nil {
		t.Fatalf("install: %v (status %+v)", err, st)
	}
	t.Logf("status: %+v", st)
	if !st.Installed || st.State != "running" || st.StartType != "boot" || !st.ServiceMode || st.OtherExe {
		t.Fatalf("after install: %+v", st)
	}
	if u, g, m := fileOwnerMode(t, sharedConfigDir); u != 0 || g != 0 || m.Perm() != 0o700 || m&os.ModeSetgid != 0 {
		t.Errorf("%s: owner %d:%d mode %v, want 0:0 0700", sharedConfigDir, u, g, m)
	}
	for _, f := range []string{"api-token", "config.json", "scheduled_jobs.json"} {
		p := filepath.Join(sharedConfigDir, f)
		u, g, m := fileOwnerMode(t, p)
		t.Logf("%s: %d:%d %v", f, u, g, m)
		if u != 0 || g != 0 || m.Perm() != 0o600 {
			t.Errorf("%s: owner %d:%d mode %v, want 0:0 0600", f, u, g, m.Perm())
		}
	}
	if !a.IsServiceConnected() {
		t.Error("an app run as root is not connected: it may read the token itself")
	}
}

func TestLiveLinuxServiceUserView(t *testing.T) {
	liveLinuxSvc(t, "user")
	if os.Geteuid() == 0 {
		t.Fatal("run as an ordinary account, not root")
	}
	if _, err := os.ReadFile(getAPITokenPath()); err == nil {
		t.Fatal("an ordinary account can read the service's token file")
	}
	a := newLiveApp()
	if a.mode != api.ModeService {
		t.Fatalf("mode %s, want service", a.mode)
	}
	if a.IsServiceConnected() {
		t.Fatal("connected without fetching the token")
	}
	// No window in front of the user: nobody is asked, the call is refused.
	jobs, err := a.GetScheduledJobs()
	if !errors.Is(err, api.ErrUnauthorized) || len(jobs) != 0 {
		t.Fatalf("Backup Sets before connecting: %d, %v; want none and ErrUnauthorized", len(jobs), err)
	}
	st := a.GetServiceStatus()
	if !st.Installed || st.State != "running" || st.IsAdmin || !st.ServiceMode {
		t.Fatalf("status as a user: %+v", st)
	}
	if _, err := a.InstallService(); err == nil {
		t.Fatal("a non-root app may install the service")
	}
}

func TestLiveLinuxServiceRemove(t *testing.T) {
	liveLinuxSvc(t, "remove")
	if os.Geteuid() != 0 {
		t.Fatal("run as root")
	}
	serviceExeOverride = packagedServiceExe
	a := newLiveApp()
	st, err := a.RemoveService()
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	t.Logf("status: %+v", st)
	if st.State == "running" || st.StartType == "boot" || st.ServiceMode {
		t.Fatalf("after remove: %+v", st)
	}
	if _, err := os.Stat(filepath.Join(sharedConfigDir, "scheduled_jobs.json")); err != nil {
		t.Fatalf("settings not kept: %v", err)
	}
}
