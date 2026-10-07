package api

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"sync"
)

// The unauthorized hook is called whenever the local service answers 401: the
// token this request presented is no longer accepted because the service
// rotated it (see TokenRotator) or it expired. The hook must obtain a fresh
// token — in practice by repeating the elevated fetch (pkexec/sudo on Linux,
// UAC on Windows) — and return it; sentToken is the token that was rejected, so
// a hook can detect that another goroutine already refreshed it and skip the
// prompt. Returning an error or an empty token means elevation was refused or
// unavailable: the request keeps its 401 and the caller surfaces it.
//
// Install it ONCE at startup, AFTER the launch-time mode resolution: the probe
// in resolveExecutionMode must keep seeing the raw 401 so it can run its own
// elevated fetch and fall back to standalone mode when that is declined.
//
// It is set through SetUnauthorizedHook rather than being a plain variable
// because installation happens while requests are already in flight (the late
// switch to service mode during a backup), and every transport reads it.
var (
	unauthorizedHookMu sync.RWMutex
	unauthorizedHook   func(sentToken string) (string, error)
)

// SetUnauthorizedHook installs (or, with nil, removes) the callback the
// transport runs on a 401.
func SetUnauthorizedHook(hook func(sentToken string) (string, error)) {
	unauthorizedHookMu.Lock()
	unauthorizedHook = hook
	unauthorizedHookMu.Unlock()
}

// GetUnauthorizedHook returns the installed 401 hook, or nil when requests
// must surface the rejection as-is.
func GetUnauthorizedHook() func(sentToken string) (string, error) {
	unauthorizedHookMu.RLock()
	defer unauthorizedHookMu.RUnlock()
	return unauthorizedHook
}

// maxReplayBody bounds the request bodies buffered to be able to replay a
// rejected request. Everything the GUI posts (jobs, config documents, PBS test
// requests) is far below it.
const maxReplayBody = 8 << 20

// reauthTransport wraps the token-injecting transport with a one-shot replay:
// when a request comes back 401 and the installed hook yields a new token, the
// request is resent with it. Callers therefore never see a rotation, they only
// see an elevated prompt (once, after the previous token stopped working).
//
// It lives in the transport so every client call — including the bare
// http.Clients built for probing — gets the behaviour without the ~19 call
// sites having to know about it.
type reauthTransport struct {
	base      http.RoundTripper
	tokenPath string
}

func (t *reauthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	sent := resolveToken(t.tokenPath)

	// Buffer the body up front: a rejected POST/DELETE has to be replayable,
	// and the body is consumed by the first attempt.
	var body []byte
	if req.Body != nil {
		b, err := io.ReadAll(io.LimitReader(req.Body, maxReplayBody+1))
		_ = req.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("read request body: %w", err)
		}
		if len(b) > maxReplayBody {
			return nil, fmt.Errorf("request body larger than %d bytes", maxReplayBody)
		}
		body = b
	}
	send := func() (*http.Response, error) {
		r := req.Clone(req.Context())
		if body != nil {
			r.Body = io.NopCloser(bytes.NewReader(body))
			r.ContentLength = int64(len(body))
		}
		return t.base.RoundTrip(r)
	}

	resp, err := send()
	// Read the hook once: it can be installed or cleared while this request
	// was in flight (the late switch to service mode).
	hook := GetUnauthorizedHook()
	if err != nil || resp.StatusCode != http.StatusUnauthorized || hook == nil {
		return resp, err
	}

	// Another goroutine may already have refreshed the token while this request
	// was in flight: retry with it instead of prompting the user a second time.
	if now := resolveToken(t.tokenPath); now != "" && now != sent {
		resp.Body.Close()
		return send()
	}

	// Keep the 401 body: if the hook fails it is handed back untouched, so the
	// caller still reports a proper "unauthorized" from the service.
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	resp.Body.Close()

	token, hookErr := hook(sent)
	if hookErr != nil || token == "" {
		resp.Body = io.NopCloser(bytes.NewReader(respBody))
		return resp, nil
	}
	SetTokenOverride(token)
	return send()
}
