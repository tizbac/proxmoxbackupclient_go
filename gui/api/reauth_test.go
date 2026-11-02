package api

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

// From upstream (reauth_test.go), adapted: a 401 turns into one elevated
// fetch (the hook) and a retry; a refusal reaches the caller as a 401; and
// here a status probe never runs the hook, so detecting the service never
// asks for elevation.

func respWithStatus(req *http.Request, code int, body string) *http.Response {
	return &http.Response{
		StatusCode: code,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}
}

func newReauthClient(t *testing.T) (*Server, *Client, string) {
	t.Helper()
	s, c := newTestService(t, &fakeHandler{})
	t.Cleanup(func() {
		SetUnauthorizedHook(nil)
		SetTokenOverride("")
	})
	return s, c, currentToken(s)
}

// rotateAway makes the server require another token, with no grace for the
// one the client holds.
func rotateAway(t *testing.T, s *Server) {
	t.Helper()
	s.SetTokenGrace(0)
	s.SetToken("rotated-token")
}

func TestReauthRetriesRejectedRequestWithFreshToken(t *testing.T) {
	s, c, first := newReauthClient(t)
	if _, err := c.GetStatus(); err != nil {
		t.Fatalf("GetStatus before rotation: %v", err)
	}
	rotateAway(t, s)

	if _, err := c.GetStatus(); err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("GetStatus after rotation (no hook) = %v, want a 401 error", err)
	}

	calls := 0
	SetUnauthorizedHook(func(sent string) (string, error) {
		calls++
		if sent != first {
			t.Errorf("hook was sent token %q, want the refused %q", sent, first)
		}
		return "rotated-token", nil
	})

	if _, err := c.GetStatus(); err != nil {
		t.Fatalf("GetStatus with the hook: %v", err)
	}
	if calls != 1 {
		t.Errorf("hook calls = %d, want 1", calls)
	}
	if got := GetTokenOverride(); got != "rotated-token" {
		t.Errorf("token override = %q, want the fetched token", got)
	}
	if code, err := c.ProbeStatus(); err != nil || code != http.StatusOK {
		t.Errorf("ProbeStatus = (%d, %v), want (200, nil)", code, err)
	}
	if _, err := c.GetStatus(); err != nil || calls != 1 {
		t.Errorf("GetStatus again: %v, hook calls %d; want the fetched token reused", err, calls)
	}
}

// A probe reports 401 as it is and never runs the hook: finding the service
// must not ask for elevation (the app starts at sign-in in the tray).
func TestProbeNeverAsks(t *testing.T) {
	s, c, _ := newReauthClient(t)
	rotateAway(t, s)
	SetUnauthorizedHook(func(string) (string, error) {
		t.Fatal("a probe ran the unauthorized hook")
		return "", nil
	})
	if code, _ := c.ProbeStatus(); code != http.StatusUnauthorized {
		t.Errorf("ProbeStatus = %d, want 401", code)
	}
	if c.IsServiceAvailable() {
		t.Error("IsServiceAvailable = true without a valid token")
	}
}

func TestReauthHookRefusalSurfacesUnauthorized(t *testing.T) {
	s, c, _ := newReauthClient(t)
	rotateAway(t, s)

	calls := 0
	SetUnauthorizedHook(func(string) (string, error) {
		calls++
		return "", errors.New("declined at the prompt")
	})
	if _, err := c.GetStatus(); err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("GetStatus = %v, want the 401", err)
	}
	if err := c.SetJobEnabled("x", true); !errors.Is(err, ErrUnauthorized) {
		t.Errorf("SetJobEnabled = %v, want ErrUnauthorized", err)
	}
	if calls != 2 {
		t.Errorf("hook calls = %d, want 2", calls)
	}
	if got := GetTokenOverride(); got != "" {
		t.Errorf("token override = %q, want none after a refusal", got)
	}
}

// A token another request fetched meanwhile is used without asking again,
// and the body is sent again with it.
func TestReauthSilentRetryReplaysBody(t *testing.T) {
	t.Cleanup(func() {
		SetUnauthorizedHook(nil)
		SetTokenOverride("")
	})
	SetTokenOverride("")

	calls := 0
	inner := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			SetTokenOverride("already-refreshed")
			return respWithStatus(req, http.StatusUnauthorized, "unauthorized"), nil
		}
		if got := req.Header.Get(tokenHeader); got != "already-refreshed" {
			t.Errorf("retried request carried token %q", got)
		}
		body, _ := io.ReadAll(req.Body)
		if string(body) != `{"job":"job-1"}` {
			t.Errorf("replayed body = %q", body)
		}
		return respWithStatus(req, http.StatusOK, `{}`), nil
	})
	tr := &reauthTransport{base: &tokenTransport{base: inner}}
	SetUnauthorizedHook(func(string) (string, error) {
		t.Fatal("the hook ran though another request had fetched a token")
		return "", nil
	})

	req, err := http.NewRequest(http.MethodPost, "http://127.0.0.1:18765/jobs", strings.NewReader(`{"job":"job-1"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || calls != 2 {
		t.Errorf("status %d, base calls %d; want 200 after one replay", resp.StatusCode, calls)
	}
}

func TestReauthRejectsOversizedBody(t *testing.T) {
	tr := &reauthTransport{
		base: roundTripFunc(func(*http.Request) (*http.Response, error) {
			t.Fatal("an oversized request was sent")
			return nil, nil
		}),
	}
	req, err := http.NewRequest(http.MethodPost, "http://127.0.0.1:18765/config", io.LimitReader(neverEnding{}, maxReplayBody+1))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tr.RoundTrip(req); err == nil {
		t.Fatal("RoundTrip accepted a body over the limit")
	}
}

type neverEnding struct{}

func (neverEnding) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'x'
	}
	return len(p), nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }
