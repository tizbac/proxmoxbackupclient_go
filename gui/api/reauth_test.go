package api

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

// When the service rotates its token (see TokenRotator), requests the GUI has
// in flight — and the ones it sends afterwards — come back 401. The transport
// must turn that into ONE elevated re-fetch (the hook, installed by the GUI as
// pkexec/sudo/UAC) and a transparent retry, and must surface the raw 401 when
// elevation is refused, so the caller can tell the user what happened.

func respWithStatus(req *http.Request, code int, body string) *http.Response {
	return &http.Response{
		StatusCode: code,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}
}

// newReauthClient returns a server whose stored token the client accepts, plus
// the globals reset after the test so nothing leaks into the next one.
func newReauthClient(t *testing.T) (*Server, *Client) {
	t.Helper()
	s, _, c := newTestServer(t, &fakeHandler{})
	t.Cleanup(func() {
		SetUnauthorizedHook(nil)
		SetTokenOverride("")
	})
	return s, c
}

// rotateAway replaces the token the server enforces, leaving no grace for the
// token the client is holding.
func rotateAway(t *testing.T, s *Server) {
	t.Helper()
	s.SetTokenGrace(0)
	s.SetToken("rotated-token")
}

func TestReauthRetriesRejectedRequestWithFreshToken(t *testing.T) {
	s, c := newReauthClient(t)

	// Baseline: the stored token works.
	if _, err := c.GetStatus(); err != nil {
		t.Fatalf("GetStatus before rotation: %v", err)
	}

	rotateAway(t, s)

	// Without a hook the rejection is what the caller sees: no silent
	// connection, no half-authenticated retry.
	_, err := c.GetStatus()
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("GetStatus after rotation (no hook) = %v, want a 401 error", err)
	}

	calls := 0
	SetUnauthorizedHook(func(sent string) (string, error) {
		calls++
		if sent != testToken {
			t.Errorf("hook was sent token %q, want the rejected %q", sent, testToken)
		}
		return "rotated-token", nil
	})

	if _, err := c.GetStatus(); err != nil {
		t.Fatalf("GetStatus after the hook is installed: %v", err)
	}
	if calls != 1 {
		t.Errorf("hook calls = %d, want 1", calls)
	}
	if got := GetTokenOverride(); got != "rotated-token" {
		t.Errorf("token override = %q, want the refreshed token", got)
	}

	// The probe path shares the transport: mode re-detection must recover too
	// without prompting a second time.
	if code, err := c.ProbeStatus(); err != nil || code != http.StatusOK {
		t.Errorf("ProbeStatus = (%d, %v), want (200, nil)", code, err)
	}
	if !c.IsServiceAvailable() {
		t.Error("IsServiceAvailable = false, want true after the refresh")
	}
	if _, err := c.GetStatus(); err != nil {
		t.Errorf("GetStatus with the refreshed token: %v", err)
	}
	if calls != 1 {
		t.Errorf("hook calls = %d, want 1 (the refreshed token must be reused)", calls)
	}
}

func TestReauthHookRefusalSurfacesUnauthorized(t *testing.T) {
	s, c := newReauthClient(t)
	rotateAway(t, s)

	calls := 0
	SetUnauthorizedHook(func(sent string) (string, error) {
		calls++
		return "", errors.New("pkexec: dismissed by the user")
	})

	_, err := c.GetStatus()
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("GetStatus = %v, want the service's 401 to reach the caller", err)
	}
	if calls != 1 {
		t.Errorf("hook calls = %d, want 1", calls)
	}
	if got := GetTokenOverride(); got != "" {
		t.Errorf("token override = %q, want it untouched when elevation fails", got)
	}
}

// A token somebody else refreshed while the request was in flight must be
// picked up without prompting the user again — and the request body has to be
// replayed with it, otherwise a rejected POST would be resent empty.
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
			// Another goroutine wins the race and refreshes the token while
			// this very request is being rejected.
			SetTokenOverride("already-refreshed")
			return respWithStatus(req, http.StatusUnauthorized, "unauthorized"), nil
		}
		if got := req.Header.Get(tokenHeader); got != "already-refreshed" {
			t.Errorf("retried request carried token %q, want the refreshed one", got)
		}
		body, err := io.ReadAll(req.Body)
		if err != nil {
			t.Errorf("reading replayed body: %v", err)
		}
		if string(body) != `{"job":"job-1"}` {
			t.Errorf("replayed body = %q, want the original payload", body)
		}
		return respWithStatus(req, http.StatusOK, `{}`), nil
	})
	tr := &reauthTransport{
		base:      &tokenTransport{tokenPath: "", base: inner},
		tokenPath: "",
	}
	SetUnauthorizedHook(func(string) (string, error) {
		t.Fatal("the hook must not prompt when the token was refreshed concurrently")
		return "", nil
	})

	req, err := http.NewRequest(http.MethodPost, "http://127.0.0.1:18765/jobs", strings.NewReader(`{"job":"job-1"}`))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := tr.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200 after the silent retry", resp.StatusCode)
	}
	if calls != 2 {
		t.Errorf("base calls = %d, want 2 (reject, then replay)", calls)
	}
}

// Oversized bodies are refused up front rather than buffered.
func TestReauthRejectsOversizedBody(t *testing.T) {
	tr := &reauthTransport{
		base: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			t.Fatal("an oversized request must not be sent at all")
			return nil, nil
		}),
	}
	req, err := http.NewRequest(http.MethodPost, "http://127.0.0.1:18765/config", io.LimitReader(neverEnding{}, maxReplayBody+1))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if _, err := tr.RoundTrip(req); err == nil {
		t.Fatal("RoundTrip = nil error, want the body-size refusal")
	}
}

// neverEnding is an infinite reader used to exceed maxReplayBody.
type neverEnding struct{}

func (neverEnding) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = 'x'
	}
	return len(p), nil
}

// roundTripFunc lets a test stand in for the token-injecting transport.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }
