package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
)

// publicHandler is a service App that also gives the outcome of the last
// runs (PublicStatusAPI), and counts how often it is asked.
type publicHandler struct {
	fakeHandler
	omu    sync.Mutex
	failed bool
	ended  string
	asked  int
}

func (p *publicHandler) LastRunOutcomeForAPI() (bool, string) {
	p.omu.Lock()
	defer p.omu.Unlock()
	p.asked++
	return p.failed, p.ended
}

// newPublicService serves a Server with h behind the real auth and returns
// it, its address and the headers each request arrived with.
func newPublicService(t *testing.T, h BackupHandler) (*Server, string, *[]http.Header) {
	t.Helper()
	tok, err := EnsureToken(filepath.Join(t.TempDir(), "api-token"))
	if err != nil {
		t.Fatal(err)
	}
	s := NewServer("127.0.0.1:0", h, tok, "2.0.0-test")
	var mu sync.Mutex
	seen := &[]http.Header{}
	inner := s.Handler()
	hs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		*seen = append(*seen, r.Header.Clone())
		mu.Unlock()
		inner.ServeHTTP(w, r)
	}))
	t.Cleanup(hs.Close)
	return s, hs.URL, seen
}

// GET /status/public answers without a token, with exactly the six fields
// and nothing of the Backup Sets: no names, no messages, no paths.
func TestPublicStatusAnswersWithoutTokenAndLeaksNothing(t *testing.T) {
	h := &publicHandler{failed: true, ended: "2026-11-02T21:04:05Z"}
	s, url, _ := newPublicService(t, h)
	music := s.BeginRun("Secret Music Set")
	s.ReportProgress(music, 40, `Backing up C:\Users\someone\Music\private.flac`)
	queued := s.BeginRun("Secret Photos Set")
	s.ReportProgress(queued, 0, QueuedMessagePrefix+"Secret Photos Set, waiting for Secret Music Set to finish...")
	done := s.BeginRun("Secret Done Set")
	s.FinishRun(done, false, "Backup failed: pbs.example.invalid refused the login")

	resp, err := http.Get(url + PublicStatusPath)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d without a token, want 200: %s", resp.StatusCode, body)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("not JSON: %v: %s", err, body)
	}
	keys := make([]string, 0, len(got))
	for k := range got {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	want := []string{"backups_running", "backups_waiting", "last_run_ended", "last_run_failed", "running", "version"}
	if strings.Join(keys, ",") != strings.Join(want, ",") {
		t.Fatalf("keys %v, want exactly %v", keys, want)
	}
	if got["running"] != true || got["version"] != "2.0.0-test" || got["backups_running"] != float64(1) ||
		got["backups_waiting"] != float64(1) || got["last_run_failed"] != true || got["last_run_ended"] != "2026-11-02T21:04:05Z" {
		t.Fatalf("answer %v", got)
	}
	for _, leak := range []string{"Secret", "Music", "Photos", "Users", "flac", "pbs.example", "Backing up", "Queued", "scheduled-"} {
		if strings.Contains(string(body), leak) {
			t.Errorf("the answer holds %q: %s", leak, body)
		}
	}
}

// The token check still applies to everything else: another method on the
// public path, any other path, and a browser's request (Origin).
func TestPublicStatusOnlyGetIsPublic(t *testing.T) {
	_, url, _ := newPublicService(t, &publicHandler{})
	for _, tc := range []struct {
		method, path, origin string
		want                 int
	}{
		{http.MethodPost, PublicStatusPath, "", http.StatusUnauthorized},
		{http.MethodPut, PublicStatusPath, "", http.StatusUnauthorized},
		{http.MethodGet, "/status", "", http.StatusUnauthorized},
		{http.MethodGet, PublicStatusPath + "/", "", http.StatusUnauthorized},
		{http.MethodGet, "/backup/runs", "", http.StatusUnauthorized},
		{http.MethodGet, "/jobs", "", http.StatusUnauthorized},
		{http.MethodGet, "/history", "", http.StatusUnauthorized},
		{http.MethodGet, PublicStatusPath, "http://evil.example", http.StatusForbidden},
	} {
		req, err := http.NewRequest(tc.method, url+tc.path, nil)
		if err != nil {
			t.Fatal(err)
		}
		if tc.origin != "" {
			req.Header.Set("Origin", tc.origin)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != tc.want {
			t.Errorf("%s %s (Origin %q) = %d, want %d", tc.method, tc.path, tc.origin, resp.StatusCode, tc.want)
		}
	}
}

// Without the handler's outcome the status still answers, with no failure
// and no last run.
func TestPublicStatusWithoutOutcome(t *testing.T) {
	_, url, _ := newPublicService(t, &fakeHandler{})
	c := NewClientAt(url, filepath.Join(t.TempDir(), "no-token-here"))
	st, err := c.PublicStatus()
	if err != nil {
		t.Fatal(err)
	}
	if *st != (PublicStatus{Running: true, Version: "2.0.0-test"}) {
		t.Fatalf("status %+v", *st)
	}
}

// The client reads it without sending a token, even while it holds one, and
// the 401 hook never runs; a reading counts as the service answering.
func TestClientPublicStatusSendsNoToken(t *testing.T) {
	h := &publicHandler{ended: "2026-11-02T08:00:00+01:00"}
	_, url, seen := newPublicService(t, h)
	tokenPath := filepath.Join(t.TempDir(), "api-token")
	if _, err := EnsureToken(tokenPath); err != nil {
		t.Fatal(err)
	}
	c := NewClientAt(url, tokenPath)
	SetTokenOverride("a-fetched-token")
	hooked := false
	SetUnauthorizedHook(func(string) (string, error) { hooked = true; return "", nil })
	t.Cleanup(func() { SetTokenOverride(""); SetUnauthorizedHook(nil) })

	st, err := c.PublicStatus()
	if err != nil {
		t.Fatal(err)
	}
	if !st.Running || st.LastRunFailed || st.LastRunEnded != "2026-11-02T08:00:00+01:00" {
		t.Fatalf("status %+v", *st)
	}
	for _, hdr := range *seen {
		if hdr.Get(tokenHeader) != "" {
			t.Fatal("the public status request carried a token")
		}
	}
	if hooked {
		t.Error("the unauthorized hook ran")
	}
	if answers, known := c.Answering(); !answers || !known {
		t.Errorf("Answering = %v, %v after a reading; want true, true", answers, known)
	}
}

// Asked often, the service reads the outcome of the last runs at most once
// in publicOutcomeCache.
func TestPublicStatusOutcomeKeptForAFewSeconds(t *testing.T) {
	h := &publicHandler{}
	_, url, _ := newPublicService(t, h)
	c := NewClientAt(url, "")
	for i := 0; i < 5; i++ {
		if _, err := c.PublicStatus(); err != nil {
			t.Fatal(err)
		}
	}
	h.omu.Lock()
	defer h.omu.Unlock()
	if h.asked != 1 {
		t.Errorf("the outcome was read %d times for 5 requests, want 1", h.asked)
	}
}

// A service older than the route answers it 401: the client says so, and
// the hook never runs.
func TestClientPublicStatusOlderService(t *testing.T) {
	hs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	t.Cleanup(hs.Close)
	hooked := false
	SetUnauthorizedHook(func(string) (string, error) { hooked = true; return "", nil })
	t.Cleanup(func() { SetUnauthorizedHook(nil) })
	c := NewClientAt(hs.URL, "")
	if _, err := c.PublicStatus(); err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("PublicStatus = %v, want a 401 error", err)
	}
	if hooked {
		t.Error("the unauthorized hook ran")
	}
}

// With SetFetchedTokenOnly on, a request carries the fetched token only: not
// the one in the token file, which this process could read.
func TestFetchedTokenOnlyIgnoresTokenFile(t *testing.T) {
	tokenPath := filepath.Join(t.TempDir(), "api-token")
	tok, err := EnsureToken(tokenPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { SetFetchedTokenOnly(false); SetTokenOverride("") })
	if got := resolveToken(tokenPath); got != tok {
		t.Fatalf("resolveToken = %q, want the file's token", got)
	}
	SetFetchedTokenOnly(true)
	if got := resolveToken(tokenPath); got != "" {
		t.Fatalf("resolveToken = %q with only the fetched token allowed and none fetched", got)
	}
	SetTokenOverride("fetched")
	if got := resolveToken(tokenPath); got != "fetched" {
		t.Fatalf("resolveToken = %q, want the fetched token", got)
	}
	SetTokenOverride("")
	SetFetchedTokenOnly(false)
	if got := resolveToken(tokenPath); got != tok {
		t.Fatalf("resolveToken = %q after turning it off, want the file's token", got)
	}
}
