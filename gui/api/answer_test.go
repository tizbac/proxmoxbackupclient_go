package api

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// Whether the service answers at all (answer.go): a probe answered 200 or
// 401 is a running service, no answer or a refused peer is a service that is
// not running, and the hook hears of each change.

// statusServer answers every request with code.
func statusServer(t *testing.T, code int) *httptest.Server {
	t.Helper()
	hs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(code)
	}))
	t.Cleanup(hs.Close)
	return hs
}

// closedAddr is a loopback address nothing listens on.
func closedAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	_ = ln.Close()
	return addr
}

func answering(t *testing.T, c *Client, want bool) {
	t.Helper()
	answers, known := c.Answering()
	if !known || answers != want {
		t.Fatalf("Answering = (%v, known %v), want (%v, known)", answers, known, want)
	}
}

func TestAnsweringFromProbe(t *testing.T) {
	for _, code := range []int{http.StatusOK, http.StatusUnauthorized} {
		hs := statusServer(t, code)
		c := NewClientAt(hs.URL, filepath.Join(t.TempDir(), "api-token"))
		if _, known := c.Answering(); known {
			t.Fatal("known before any request")
		}
		if got, err := c.ProbeStatus(); got != code || err != nil {
			t.Fatalf("ProbeStatus = (%d, %v), want (%d, nil)", got, err, code)
		}
		answering(t, c, true)
	}
}

func TestNotAnsweringWhenNothingListens(t *testing.T) {
	c := NewClientAt("http://"+closedAddr(t), filepath.Join(t.TempDir(), "api-token"))
	code, err := c.ProbeStatus()
	if code != 0 || !IsNoAnswer(err) {
		t.Fatalf("ProbeStatus = (%d, %v), want (0, no answer)", code, err)
	}
	answering(t, c, false)
	// Any request records it, not only a probe.
	if _, err := c.GetRuns(); err == nil {
		t.Fatal("GetRuns answered with nothing listening")
	}
	answering(t, c, false)
}

// Something other than the service on its port: refused by the peer check,
// and so not running.
func TestNotAnsweringWhenPeerRefused(t *testing.T) {
	hs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("the request reached the impostor")
	}))
	defer hs.Close()
	_, port, _ := net.SplitHostPort(strings.TrimPrefix(hs.URL, "http://"))
	asServicePort(t, port, func(conn net.Conn) error {
		return &ServicePeerError{Addr: conn.RemoteAddr().String(), Owner: "process 6666"}
	})
	SetPeerLog(func(string) {})
	t.Cleanup(func() { SetPeerLog(nil) })
	c := NewClientAt(hs.URL, filepath.Join(t.TempDir(), "api-token"))
	code, err := c.ProbeStatus()
	var pe *ServicePeerError
	if code != 0 || !errors.As(err, &pe) || !IsNoAnswer(err) {
		t.Fatalf("ProbeStatus = (%d, %v), want (0, a refused peer)", code, err)
	}
	answering(t, c, false)
}

// The hook hears of each change after the first finding, once, with why the
// service does not answer.
func TestAnswerHookOnChanges(t *testing.T) {
	var mu sync.Mutex
	var heard []string
	hook := func(answers bool, err error) {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case answers && err == nil:
			heard = append(heard, "up")
		case !answers && IsNoAnswer(err):
			heard = append(heard, "down")
		default:
			heard = append(heard, "odd")
		}
	}
	c := NewClientAt("http://"+closedAddr(t), filepath.Join(t.TempDir(), "api-token"))
	c.SetAnswerHook(hook)
	_, _ = c.ProbeStatus() // the first finding: not a change
	_, _ = c.ProbeStatus()

	hs := statusServer(t, http.StatusUnauthorized)
	c.baseURL = hs.URL
	_, _ = c.ProbeStatus() // started
	_, _ = c.GetRuns()     // still answering (401)

	c.baseURL = "http://" + closedAddr(t)
	_, _ = c.GetRuns() // stopped
	_, _ = c.ProbeStatus()

	mu.Lock()
	got := strings.Join(heard, ",")
	mu.Unlock()
	if got != "up,down" {
		t.Fatalf("hook heard %q, want \"up,down\"", got)
	}

	c.SetAnswerHook(nil)
	c.baseURL = hs.URL
	_, _ = c.ProbeStatus()
	answering(t, c, true)
}

// A request that failed for another reason (cancelled, or an answer that
// could not be read) says nothing about whether the service runs.
func TestAnswerUnchangedByOtherErrors(t *testing.T) {
	hs := statusServer(t, http.StatusOK)
	c := NewClientAt(hs.URL, filepath.Join(t.TempDir(), "api-token"))
	if _, err := c.ProbeStatus(); err != nil {
		t.Fatal(err)
	}
	c.noteAnswer(context.Canceled)
	c.noteAnswer(errors.New("unexpected EOF"))
	answering(t, c, true)

	if IsNoAnswer(nil) || IsNoAnswer(errors.New("boom")) || IsNoAnswer(&net.OpError{Op: "read", Err: errors.New("reset")}) {
		t.Error("IsNoAnswer for an error that is not a missing answer")
	}
	if !IsNoAnswer(&net.OpError{Op: "dial", Err: errors.New("connection refused")}) {
		t.Error("IsNoAnswer false for a refused connection")
	}
}
