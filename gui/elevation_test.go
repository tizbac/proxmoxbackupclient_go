package main

import (
	"context"
	stdruntime "runtime"
	"testing"

	"github.com/tizbac/proxmoxbackupclient_go/gui/api"
)

// The frontend decides from three functions whether to show the "administrator
// privileges required" box and to disable disk selection for machine backups,
// and whether to relaunch the app with pkexec. In service mode all of that work
// happens inside the privileged service, so an unprivileged GUI must NOT be
// told it needs elevation.

// newElevationTestApp returns a GUI standing in for an unprivileged client
// (mode + api client, no Wails context).
func newElevationTestApp(t *testing.T, mode api.ExecutionMode) *App {
	t.Helper()
	SetConfigDir(t.TempDir())
	t.Cleanup(func() { SetConfigDir("") })
	// switchToServiceMode installs the token-refresh hook (a package global
	// that would otherwise survive this test and try a real pkexec/UAC prompt
	// in the next one).
	t.Cleanup(func() {
		api.SetUnauthorizedHook(nil)
		api.SetTokenOverride("")
	})
	return &App{mode: mode, apiClient: api.NewClient(getAPITokenPath())}
}

func TestNeedsLocalElevation(t *testing.T) {
	// Standalone: this process opens the block devices itself, so it must be
	// root — true unless the tests happen to run as root.
	standalone := newElevationTestApp(t, api.ModeStandalone)
	if got, want := standalone.needsLocalElevation(), !isAdmin(); got != want {
		t.Errorf("standalone: needsLocalElevation() = %v, want %v", got, want)
	}

	// Service mode: the privileged service runs the backup as root, the GUI
	// only picks disks and posts the request — true on Linux. On Windows the
	// GUI still has to open \\.\PhysicalDriveN itself to LIST the disks, which
	// needs an elevated token even though the backup runs in the service.
	service := newElevationTestApp(t, api.ModeService)
	if got, want := service.needsLocalElevation(), stdruntime.GOOS == "windows"; got != want {
		t.Errorf("service mode: needsLocalElevation() = %v, want %v", got, want)
	}

	// The service process itself IS the privileged helper.
	svcProc := NewAppForService(context.Background())
	t.Cleanup(func() { SetConfigDir("") })
	if svcProc.needsLocalElevation() {
		t.Error("service process: must never need local elevation")
	}
}

func TestGetSystemInfoExposesElevationFlag(t *testing.T) {
	a := newElevationTestApp(t, api.ModeService)

	info := a.GetSystemInfo()
	got, ok := info["needs_local_elevation"].(bool)
	if !ok {
		t.Fatalf("needs_local_elevation missing or not a bool: %#v", info["needs_local_elevation"])
	}
	if want := stdruntime.GOOS == "windows"; got != want {
		t.Errorf("service mode needs_local_elevation = %v, want %v (disk listing rules per platform)", got, want)
	}
	if info["mode"] != api.ModeService.String() {
		t.Errorf("mode = %v, want %v", info["mode"], api.ModeService.String())
	}
	// is_admin keeps reporting the raw process state: other UI (VSS notice)
	// still needs it.
	if v, ok := info["is_admin"].(bool); !ok || v != isAdmin() {
		t.Errorf("is_admin = %v (present=%v), want %v", info["is_admin"], ok, isAdmin())
	}

	// Standalone, without root, does need it.
	b := newElevationTestApp(t, api.ModeStandalone)
	if got, want := b.GetSystemInfo()["needs_local_elevation"], !isAdmin(); got != want {
		t.Errorf("standalone needs_local_elevation = %v, want %v", got, want)
	}
}

func TestCanModifyJobsDoesNotDemandAdminInServiceMode(t *testing.T) {
	// Job writes go through the service API, so no local admin rights: this
	// must hold even on a host where canModifyJobs() is false.
	service := newElevationTestApp(t, api.ModeService)
	if !service.CanModifyJobs() {
		t.Error("service mode: editing scheduled jobs must not require local admin")
	}

	// Standalone keeps the platform rule.
	standalone := newElevationTestApp(t, api.ModeStandalone)
	if got, want := standalone.CanModifyJobs(), canModifyJobs(); got != want {
		t.Errorf("standalone CanModifyJobs() = %v, want %v", got, want)
	}
}

func TestRequestElevationIsNoopInServiceMode(t *testing.T) {
	// Relaunching the unprivileged GUI with pkexec would only spawn a second
	// unprivileged copy: in service mode there is nothing to elevate.
	a := newElevationTestApp(t, api.ModeService)
	if err := a.RequestElevation(); err != nil {
		t.Errorf("RequestElevation() = %v, want nil in service mode", err)
	}
	if err := a.RequestJobModificationElevation(); err != nil {
		t.Errorf("RequestJobModificationElevation() = %v, want nil in service mode", err)
	}
}

func TestSwitchToServiceMode(t *testing.T) {
	a := newElevationTestApp(t, api.ModeStandalone)
	a.standaloneReason = "no_service"

	// The tests have no Wails context: switching must not panic and must
	// still clear the now-obsolete standalone reason.
	a.switchToServiceMode()
	if a.mode != api.ModeService {
		t.Errorf("mode = %v, want %v", a.mode, api.ModeService)
	}
	if a.standaloneReason != "" {
		t.Errorf("standaloneReason = %q, want empty once the service takes over", a.standaloneReason)
	}

	// Idempotent.
	a.switchToServiceMode()
	if a.mode != api.ModeService {
		t.Errorf("second switch: mode = %v, want %v", a.mode, api.ModeService)
	}
}

// The elevation rule itself, exercised for both platforms on this host: the
// frontend turns exactly this flag into "run as administrator" or "the service
// handles it", so the two must never disagree.
func TestNeedsElevationFor(t *testing.T) {
	cases := []struct {
		name        string
		goos        string
		admin       bool
		serviceProc bool
		mode        api.ExecutionMode
		want        bool
	}{
		{"linux standalone unprivileged", "linux", false, false, api.ModeStandalone, true},
		{"linux standalone root", "linux", true, false, api.ModeStandalone, false},
		{"linux service unprivileged", "linux", false, false, api.ModeService, false},
		{"linux service root", "linux", true, false, api.ModeService, false},
		{"windows standalone unprivileged", "windows", false, false, api.ModeStandalone, true},
		{"windows standalone elevated", "windows", true, false, api.ModeStandalone, false},
		// The Windows difference: listing \\.\PhysicalDriveN needs an
		// elevated token, so even a GUI that delegates the backup to the
		// service must ask for one.
		{"windows service unprivileged", "windows", false, false, api.ModeService, true},
		{"windows service elevated", "windows", true, false, api.ModeService, false},
		{"the service process itself never elevates", "linux", false, true, api.ModeStandalone, false},
		{"the service process itself never elevates (windows)", "windows", false, true, api.ModeStandalone, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := needsElevationFor(c.goos, c.admin, c.serviceProc, c.mode); got != c.want {
				t.Errorf("needsElevationFor(%s, admin=%v, serviceProc=%v, %v) = %v, want %v",
					c.goos, c.admin, c.serviceProc, c.mode, got, c.want)
			}
		})
	}
}
