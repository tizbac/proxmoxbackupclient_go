//go:build !service

package main

// The app lets go of the service's token once its window has been in the
// tray for a while (token_drop.go), and the tray then reads the service's
// public status, which needs no token (gui/api/public_status.go).

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/tizbac/proxmoxbackupclient_go/gui/api"
)

// setDropGrace shortens the grace period (and the retry) for one test.
func setDropGrace(t *testing.T, grace time.Duration) {
	t.Helper()
	oldGrace, oldRetry := tokenDropGrace, tokenDropRetry
	tokenDropGrace, tokenDropRetry = grace, grace
	t.Cleanup(func() { tokenDropGrace, tokenDropRetry = oldGrace, oldRetry })
}

// setWindowVisible makes mainWindowVisible answer visible for one test.
func setWindowVisible(t *testing.T, visible bool) {
	t.Helper()
	old := mainWindowVisible
	mainWindowVisible = func(*App) bool { return visible }
	t.Cleanup(func() { mainWindowVisible = old })
}

// connectedTrayPair is newStrictPair with the app connected through a
// fetched token, as after the elevation prompt, and its window on screen.
func connectedTrayPair(t *testing.T) (*strictPair, string) {
	t.Helper()
	setWindowVisible(t, false)
	p := newStrictPair(t)
	tok, err := os.ReadFile(p.tokenPath)
	if err != nil {
		t.Fatal(err)
	}
	token := strings.TrimSpace(string(tok))
	a := p.app
	// newStrictPair connected it already (it may read the file); hold the
	// token in memory too, as after the prompt. Not through useServiceToken:
	// its reading of the settings runs on in the background.
	api.SetTokenOverride(token)
	if !a.svcConnected.Load() {
		t.Fatal("not connected")
	}
	if _, err := a.apiClient.GetRuns(); err != nil {
		t.Fatalf("GetRuns while connected: %v", err)
	}
	return p, token
}

// waitFor polls cond for up to two seconds.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting until %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// Hidden in the tray for the whole grace period: the token is gone from
// memory, the app is not connected, the service refuses it (an app that may
// read the token file does not fall back on it either), and it will ask
// once on its own when the window is shown. The public status still answers.
func TestTokenDroppedAfterGraceWhileHidden(t *testing.T) {
	setDropGrace(t, 30*time.Millisecond)
	p, _ := connectedTrayPair(t)
	a := p.app
	a.autoConnectTried.Store(true) // it asked when the window first came up

	a.windowHiddenToTray()
	waitFor(t, "the token is dropped", func() bool { return !a.svcConnected.Load() })

	if got := api.GetTokenOverride(); got != "" {
		t.Errorf("the token is still in memory: %q", got)
	}
	if !api.FetchedTokenOnly() {
		t.Error("the app would read the token file again on its next request")
	}
	if _, err := a.apiClient.GetRuns(); err == nil {
		t.Error("the service took a request after the token was dropped")
	}
	if a.IsServiceConnected() {
		t.Error("IsServiceConnected after the drop")
	}
	if a.autoConnectTried.Load() {
		t.Error("the app will not ask on its own when the window is shown")
	}
	if !a.holdsNoToken() {
		t.Error("holdsNoToken is false after the drop")
	}
	st, err := a.apiClient.PublicStatus()
	if err != nil || !st.Running {
		t.Errorf("public status after the drop = %+v, %v", st, err)
	}
}

// Shown again within the grace period: the token stays.
func TestTokenKeptWhenShownWithinGrace(t *testing.T) {
	setDropGrace(t, 150*time.Millisecond)
	p, token := connectedTrayPair(t)
	a := p.app

	a.windowHiddenToTray()
	time.Sleep(30 * time.Millisecond)
	a.windowShownAgain()
	time.Sleep(400 * time.Millisecond)

	if !a.svcConnected.Load() || api.GetTokenOverride() != token {
		t.Fatal("the token was dropped although the window was shown within the grace period")
	}
	if _, err := a.apiClient.GetRuns(); err != nil {
		t.Errorf("GetRuns after showing the window: %v", err)
	}

	// Hidden again, the grace period starts afresh and ends with the drop.
	a.windowHiddenToTray()
	waitFor(t, "the token is dropped", func() bool { return !a.svcConnected.Load() })
}

// The page asking to connect (its focus, or Connect) means the window is in
// use: the grace period ends without a drop.
func TestTokenKeptWhenPageConnects(t *testing.T) {
	setDropGrace(t, 100*time.Millisecond)
	p, _ := connectedTrayPair(t)
	a := p.app

	a.windowHiddenToTray()
	if err := a.ConnectService(false); err != nil {
		t.Fatalf("ConnectService while connected: %v", err)
	}
	time.Sleep(300 * time.Millisecond)
	if !a.svcConnected.Load() {
		t.Fatal("the token was dropped although the page asked to connect")
	}
}

// Shown by something the app was not told of (a second launch on Windows):
// the token stays.
func TestTokenKeptWhileWindowOnScreen(t *testing.T) {
	setDropGrace(t, 30*time.Millisecond)
	p, _ := connectedTrayPair(t)
	a := p.app
	setWindowVisible(t, true)

	a.windowHiddenToTray()
	time.Sleep(200 * time.Millisecond)
	if !a.svcConnected.Load() {
		t.Fatal("the token was dropped while the window was on screen")
	}
	a.trayHide.mu.Lock()
	hidden := a.trayHide.hidden
	a.trayHide.mu.Unlock()
	if hidden {
		t.Error("the window is still taken for hidden")
	}
}

// A connect under way (its prompt open) when the grace period ends: nothing
// is dropped then, and the app looks again later.
func TestTokenDropWaitsForConnectUnderWay(t *testing.T) {
	setDropGrace(t, 30*time.Millisecond)
	p, _ := connectedTrayPair(t)
	a := p.app

	a.tokenFetchMu.Lock()
	a.windowHiddenToTray()
	time.Sleep(120 * time.Millisecond)
	if !a.svcConnected.Load() {
		a.tokenFetchMu.Unlock()
		t.Fatal("the token was dropped while a connect was under way")
	}
	a.tokenFetchMu.Unlock()
	waitFor(t, "the token is dropped after the connect", func() bool { return !a.svcConnected.Load() })
}

// After the drop the app connects again with a token it fetched, and may
// read the token file itself again.
func TestReconnectAfterTokenDropped(t *testing.T) {
	setDropGrace(t, 20*time.Millisecond)
	p, token := connectedTrayPair(t)
	a := p.app
	a.windowHiddenToTray()
	waitFor(t, "the token is dropped", func() bool { return !a.svcConnected.Load() })

	a.windowShownAgain()
	configReads := func() int {
		p.mu.Lock()
		defer p.mu.Unlock()
		n := 0
		for _, s := range p.seen {
			if s == "GET /config" {
				n++
			}
		}
		return n
	}
	before := configReads()
	a.tokenFetchMu.Lock()
	err := a.useServiceToken(token, nil)
	a.tokenFetchMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	// Connected, the app reads the service's settings in the background.
	waitFor(t, "the settings are read again", func() bool { return configReads() > before })
	time.Sleep(50 * time.Millisecond)
	if api.FetchedTokenOnly() {
		t.Error("still limited to the fetched token after connecting")
	}
	if _, err := a.apiClient.GetRuns(); err != nil {
		t.Errorf("GetRuns after connecting again: %v", err)
	}
}

// Nothing to drop for an app on its own settings, or without the service.
func TestTokenDropOnlyInServiceMode(t *testing.T) {
	a := &App{mode: api.ModeStandalone}
	t.Cleanup(resetServiceGlobals)
	if a.dropServiceToken(time.Minute) {
		t.Error("busy in standalone mode")
	}
	if api.FetchedTokenOnly() {
		t.Error("standalone mode limited the token")
	}
}

// Without the token the background requests never ask for elevation: the
// run list is refused quietly, and the public status needs none.
func TestNoPromptWithoutToken(t *testing.T) {
	setDropGrace(t, 20*time.Millisecond)
	p, _ := connectedTrayPair(t)
	a := p.app
	a.windowHiddenToTray()
	waitFor(t, "the token is dropped", func() bool { return !a.svcConnected.Load() })

	asked := false
	hook := api.GetUnauthorizedHook()
	api.SetUnauthorizedHook(func(sent string) (string, error) {
		tok, err := hook(sent)
		if err == nil || !errors.Is(err, errNoPromptNow) {
			asked = true
		}
		return tok, err
	})
	if _, err := a.apiClient.GetRuns(); err == nil {
		t.Error("the run list answered without a token")
	}
	if _, err := a.apiClient.PublicStatus(); err != nil {
		t.Errorf("public status: %v", err)
	}
	if asked {
		t.Error("a background request would have asked for elevation")
	}
}

// What the tray says from the public status.
func TestPublicTrayText(t *testing.T) {
	ended := time.Date(2026, 11, 2, 21, 4, 0, 0, time.UTC)
	for _, tc := range []struct {
		st   *api.PublicStatus
		want string
	}{
		{nil, ""},
		{&api.PublicStatus{Running: true}, "Scheduled backups active"},
		{&api.PublicStatus{Running: true, BackupsRunning: 1}, "1 backup running"},
		{&api.PublicStatus{Running: true, BackupsRunning: 2, BackupsWaiting: 1}, "2 backups running, 1 waiting"},
		{&api.PublicStatus{Running: true, BackupsWaiting: 3}, "3 backups waiting"},
		{&api.PublicStatus{Running: true, BackupsRunning: 1, LastRunFailed: true}, "1 backup running; last backup failed"},
		{&api.PublicStatus{Running: true, LastRunFailed: true, LastRunEnded: ended.Format(time.RFC3339)}, "Last backup failed: open the app"},
		{&api.PublicStatus{Running: true, LastRunEnded: ended.Format(time.RFC3339)}, "Last backup ended " + ended.Local().Format("02 Jan 15:04")},
		{&api.PublicStatus{Running: true, LastRunEnded: "not a time"}, "Scheduled backups active"},
	} {
		if got := publicTrayText(tc.st); got != tc.want {
			t.Errorf("publicTrayText(%+v) = %q, want %q", tc.st, got, tc.want)
		}
	}
}

// The outcome of the last runs: the most recent run of each Backup Set, by
// name; restores, one-off backups and deleted sets do not count for failed.
func TestLastRunOutcome(t *testing.T) {
	jobs := []ScheduledJob{{ID: "1", Name: "Documents"}, {ID: "2", Name: "Photos"}}
	h := func(name, status, ts string, mod func(*JobHistory)) JobHistory {
		e := JobHistory{Name: name, Status: status, Timestamp: ts}
		if mod != nil {
			mod(&e)
		}
		return e
	}
	restore := func(e *JobHistory) { e.Kind = "restore" }
	oneoff := func(e *JobHistory) { e.Trigger = "oneoff" }
	for _, tc := range []struct {
		name       string
		hist       []JobHistory
		wantFailed bool
		wantEnded  string
	}{
		{"no history", nil, false, ""},
		{"all fine", []JobHistory{h("Documents", "success", "t3", nil), h("Photos", "success", "t2", nil)}, false, "t3"},
		{"one set's last run failed", []JobHistory{h("Documents", "success", "t3", nil), h("Photos", "failed", "t2", nil)}, true, "t3"},
		{"failed earlier, fine since", []JobHistory{h("Photos", "success", "t3", nil), h("Photos", "failed", "t2", nil)}, false, "t3"},
		{"a restore is not a backup", []JobHistory{h("Restore of x", "failed", "t4", restore), h("Documents", "success", "t3", nil)}, false, "t3"},
		{"a one-off counts only for ended", []JobHistory{h("Documents", "failed", "t4", oneoff), h("Documents", "success", "t3", nil)}, false, "t4"},
		{"a deleted set does not count", []JobHistory{h("Old set", "failed", "t4", nil), h("Documents", "success", "t3", nil)}, false, "t4"},
		{"a run still marked running", []JobHistory{h("Documents", "running", "t4", nil), h("Documents", "failed", "t3", nil)}, true, "t3"},
	} {
		failed, ended := lastRunOutcome(jobs, tc.hist)
		if failed != tc.wantFailed || ended != tc.wantEnded {
			t.Errorf("%s: lastRunOutcome = %v, %q; want %v, %q", tc.name, failed, ended, tc.wantFailed, tc.wantEnded)
		}
	}
}

// End to end: the service's history and Backup Sets reach an app that holds
// no token, through the public status, as two numbers and a time.
func TestPublicStatusFromServiceHistory(t *testing.T) {
	p := newStrictService(t)
	when := time.Date(2026, 11, 2, 21, 4, 0, 0, time.UTC).Format(time.RFC3339)
	p.service(func(s *App) {
		if err := s.SaveScheduledJob(strictJob("set1", t.TempDir())); err != nil {
			t.Fatal(err)
		}
		if err := s.appendJobHistory(JobHistory{ID: "1", Name: "strict set1", Timestamp: when, Status: "failed", Message: "PBS refused the login"}); err != nil {
			t.Fatal(err)
		}
	})
	c := api.NewClientAt(p.url, "")
	st, err := c.PublicStatus()
	if err != nil {
		t.Fatal(err)
	}
	if !st.Running || st.Version != "test" || !st.LastRunFailed || st.LastRunEnded != when || st.BackupsRunning != 0 || st.BackupsWaiting != 0 {
		t.Fatalf("public status %+v", *st)
	}
	p.saw(t, "GET "+api.PublicStatusPath)
}
