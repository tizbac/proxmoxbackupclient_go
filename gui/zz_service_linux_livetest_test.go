//go:build linux && !service

package main

// Opt-in live tests of the Linux background service, on a test machine with
// the .deb installed (they install, start and remove the real systemd unit).
//
//   PBS_LINUX_SVC=install  (as root, HOME = the user whose settings to move):
//       Run as Service > Install: settings copied to /var/lib/proxmoxbackupclient,
//       folder and token permissions, unit enabled and running, app in service mode.
//   PBS_LINUX_SVC=user     (as an administrator, not root): the app finds the
//       service, reads its token, sees the Backup Sets, and may not change them.
//   PBS_LINUX_SVC=remove   (as root): Remove stops and disables the unit and
//       keeps the settings.

import (
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"

	"github.com/tizbac/proxmoxbackupclient_go/gui/api"
)

func liveLinuxSvc(t *testing.T, step string) {
	t.Helper()
	if os.Getenv("PBS_LINUX_SVC") != step {
		t.Skip("set PBS_LINUX_SVC=" + step)
	}
	useRealSettings(t)
}

func newLiveApp() *App {
	return &App{
		config:        LoadConfig(),
		stopScheduler: make(chan struct{}),
		apiClient:     api.NewClient(getAPITokenPath()),
		mode:          api.NewModeDetector(getAPITokenPath()).DetectMode(),
		callbacksMap:  make(map[string]*progressCallbacks),
	}
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
	serviceExeOverride = "/usr/lib/proxmoxbackupclient/ProxmoxBackupClientSVC"
	a := newLiveApp()
	st, err := a.InstallService()
	if err != nil {
		t.Fatalf("install: %v (status %+v)", err, st)
	}
	t.Logf("status: %+v", st)
	if !st.Installed || st.State != "running" || st.StartType != "boot" || !st.ServiceMode || st.OtherExe {
		t.Fatalf("after install: %+v", st)
	}
	gid, group := adminGroup()
	if _, g, m := fileOwnerMode(t, sharedConfigDir); g != uint32(gid) || m.Perm() != 0o750 || m&os.ModeSetgid == 0 {
		t.Errorf("%s: group %d (want %d %s), mode %v", sharedConfigDir, g, gid, group, m)
	}
	for _, f := range []string{"api-token", "config.json", "scheduled_jobs.json"} {
		p := filepath.Join(sharedConfigDir, f)
		u, g, m := fileOwnerMode(t, p)
		t.Logf("%s: %d:%d %v", f, u, g, m)
		if u != 0 || g != uint32(gid) || m.Perm() != 0o640 {
			t.Errorf("%s: owner %d:%d mode %v, want 0:%d 0640", f, u, g, m.Perm(), gid)
		}
	}
	if got, _ := getConfigDir(); got != sharedConfigDir {
		t.Errorf("app settings folder %s, want %s", got, sharedConfigDir)
	}
}

func TestLiveLinuxServiceUserView(t *testing.T) {
	liveLinuxSvc(t, "user")
	if os.Geteuid() == 0 {
		t.Fatal("run as an administrator account, not root")
	}
	if u, err := user.Current(); err == nil {
		gid, group := adminGroup()
		ids, _ := u.GroupIds()
		in := false
		for _, g := range ids {
			if g == strconv.Itoa(gid) {
				in = true
			}
		}
		if !in {
			t.Fatalf("%s is not in group %s", u.Username, group)
		}
	}
	if dir, _ := getConfigDir(); dir != sharedConfigDir {
		t.Fatalf("settings folder %s, want %s", dir, sharedConfigDir)
	}
	a := newLiveApp()
	if a.mode != api.ModeService {
		t.Fatalf("mode %s, want service", a.mode)
	}
	jobs, err := a.GetScheduledJobs()
	if err != nil || len(jobs) == 0 {
		t.Fatalf("Backup Sets: %d, %v", len(jobs), err)
	}
	p := a.GetJobPolicy()
	t.Logf("policy %+v, %d set(s)", p, len(jobs))
	if !p.Supported || !p.RequireAdmin || p.IsAdmin || p.CanModify || p.SettingsLocked || !p.Fixed {
		t.Fatalf("policy %+v", p)
	}
	if err := a.checkJobsChangeAllowed(); err == nil {
		t.Fatal("a non-root app may change the shared Backup Sets")
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
	serviceExeOverride = "/usr/lib/proxmoxbackupclient/ProxmoxBackupClientSVC"
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
