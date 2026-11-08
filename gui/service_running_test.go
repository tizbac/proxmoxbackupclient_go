//go:build !service

package main

// Whether the installed background service runs (service_connect.go
// serviceRunning, onServiceAnswer): GetSystemInfo's service_running follows
// what the app's requests to the service find, and Connect does not ask for
// elevation while it is not running. A refused peer (something other than
// the service on its port) counts as not running: gui/api/answer_test.go.

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/salvanza/salvanza_client/gui/api"
)

// svcStatusServer stands in for the service: it answers every request with
// code.
func svcStatusServer(t *testing.T, code int) *httptest.Server {
	t.Helper()
	hs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(code)
	}))
	t.Cleanup(hs.Close)
	return hs
}

// serviceModeApp is an app in service mode, not connected, whose client
// talks to url; it has probed the service once, as resolveMode does.
func serviceModeApp(t *testing.T, url string) *App {
	t.Helper()
	strictHome(t)
	a := &App{apiClient: api.NewClientAt(url, filepath.Join(t.TempDir(), "api-token"))}
	t.Cleanup(resetServiceGlobals)
	_, _ = a.apiClient.ProbeStatus()
	a.enterServiceMode(false)
	return a
}

func freeLoopbackURL(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	u := "http://" + ln.Addr().String()
	_ = ln.Close()
	return u
}

func TestServiceRunningFromProbe(t *testing.T) {
	for _, tc := range []struct {
		name string
		code int // 0: nothing listens
		want bool
	}{
		{"answers 200", http.StatusOK, true},
		{"answers 401", http.StatusUnauthorized, true},
		{"nothing listens", 0, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			url := freeLoopbackURL(t)
			if tc.code != 0 {
				url = svcStatusServer(t, tc.code).URL
			}
			a := serviceModeApp(t, url)
			if got := a.serviceRunning(); got != tc.want {
				t.Errorf("serviceRunning = %v, want %v", got, tc.want)
			}
			if got, _ := a.GetSystemInfo()["service_running"].(bool); got != tc.want {
				t.Errorf("GetSystemInfo service_running = %v, want %v", got, tc.want)
			}
		})
	}
}

// Without the service (standalone, or the user's own settings) it is never
// "running": the window shows no service banner then.
func TestServiceRunningOnlyInServiceMode(t *testing.T) {
	hs := svcStatusServer(t, http.StatusUnauthorized)
	a := &App{apiClient: api.NewClientAt(hs.URL, filepath.Join(t.TempDir(), "api-token")), mode: api.ModeStandalone}
	if a.serviceRunning() {
		t.Error("running in standalone mode")
	}
	a = serviceModeApp(t, hs.URL)
	a.ownSettings.Store(true)
	if a.serviceRunning() {
		t.Error("running on the user's own settings")
	}
}

// Nothing sent yet: serviceRunning probes.
func TestServiceRunningProbesWhenUnknown(t *testing.T) {
	hs := svcStatusServer(t, http.StatusUnauthorized)
	strictHome(t)
	a := &App{apiClient: api.NewClientAt(hs.URL, filepath.Join(t.TempDir(), "api-token"))}
	t.Cleanup(resetServiceGlobals)
	a.enterServiceMode(false)
	if !a.serviceRunning() {
		t.Error("not running, though the service answers")
	}
	if _, known := a.apiClient.Answering(); !known {
		t.Error("the probe was not recorded")
	}
}

// The service stops and starts again: the app notices from its ordinary
// requests (the run list, every 3 s), and once it answers again the app may
// ask once more on its own when the window comes to the front.
func TestServiceStopAndStartNoticed(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	hs := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	_ = hs.Listener.Close()
	hs.Listener = ln
	hs.Start()

	a := serviceModeApp(t, "http://"+addr)
	if !a.serviceRunning() {
		t.Fatal("not running at first")
	}
	a.autoConnectTried.Store(true) // it asked once already

	hs.Close() // the service stops
	if _, err := a.apiClient.GetRuns(); err == nil {
		t.Fatal("GetRuns answered by a stopped service")
	}
	if a.serviceRunning() {
		t.Fatal("still running after the service stopped")
	}
	// Connect does not ask for elevation for a service that is not running.
	if err := a.ConnectService(true); !errors.Is(err, errServiceNotRunning) {
		t.Fatalf("ConnectService = %v, want errServiceNotRunning", err)
	}

	// The service starts again on the same port.
	ln2, err := net.Listen("tcp", addr)
	if err != nil {
		t.Skipf("the port could not be taken again: %v", err)
	}
	hs2 := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	_ = hs2.Listener.Close()
	hs2.Listener = ln2
	hs2.Start()
	defer hs2.Close()
	if _, err := a.apiClient.GetRuns(); err == nil {
		t.Fatal("GetRuns succeeded without a token")
	}
	if !a.serviceRunning() {
		t.Fatal("not running after the service started again")
	}
	if a.autoConnectTried.Load() {
		t.Error("the app will not ask on its own after the service started again")
	}
	if a.svcConnected.Load() {
		t.Error("connected without a token")
	}
}

// Leaving service mode removes the hook: a later start says nothing.
func TestLeaveServiceModeDropsAnswerHook(t *testing.T) {
	url := freeLoopbackURL(t)
	a := serviceModeApp(t, url)
	if a.serviceRunning() {
		t.Fatal("running with nothing listening")
	}
	a.leaveServiceMode()
	a.autoConnectTried.Store(true)
	ln, err := net.Listen("tcp", url[len("http://"):])
	if err != nil {
		t.Skipf("the port could not be taken again: %v", err)
	}
	hs := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	_ = hs.Listener.Close()
	hs.Listener = ln
	hs.Start()
	defer hs.Close()
	if code, _ := a.apiClient.ProbeStatus(); code != http.StatusUnauthorized {
		t.Fatalf("ProbeStatus = %d, want 401", code)
	}
	if a.serviceRunning() {
		t.Error("running in standalone mode")
	}
	if !a.autoConnectTried.Load() {
		t.Error("the hook ran after leaving service mode")
	}
}
