package main

import (
	"context"
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
	// only picks disks and posts the request.
	service := newElevationTestApp(t, api.ModeService)
	if service.needsLocalElevation() {
		t.Error("service mode: an unprivileged GUI must not need local elevation")
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
	if got {
		t.Error("service mode reported needs_local_elevation=true")
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
