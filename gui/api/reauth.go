package api

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"sync"
)

// The unauthorized hook runs when the service answers 401: the app has no
// token yet, or the one it holds was replaced (TokenRotator). It must get a
// fresh token (the elevated fetch: UAC on Windows, pkexec on Linux) and return
// it; sentToken is the one that was refused, so a hook can tell that another
// request already fetched a new one. An error or "" means there is none (the
// user declined, or the app may not ask now, such as at sign-in with no
// window open): the request keeps its 401. From upstream (reauth.go), which
// fetches at start; this app installs the hook from the start and lets the
// hook decide when it may ask.
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

// GetUnauthorizedHook returns the installed 401 hook, or nil.
func GetUnauthorizedHook() func(sentToken string) (string, error) {
	unauthorizedHookMu.RLock()
	defer unauthorizedHookMu.RUnlock()
	return unauthorizedHook
}

// maxReplayBody bounds the request bodies kept to replay a refused request.
// Everything the app posts (Backup Sets, the settings document) is far below.
const maxReplayBody = 8 << 20

// reauthTransport sends a request and, when it comes back 401 and the hook
// yields a new token, sends it once more with that token. Callers never see a
// rotation; the user sees one elevation prompt after the old token stopped
// working.
type reauthTransport struct {
	base      http.RoundTripper
	tokenPath string
}

func (t *reauthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	sent := resolveToken(t.tokenPath)

	// Keep the body: a refused POST has to be sent again, and the first
	// attempt consumes it.
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
	hook := GetUnauthorizedHook()
	if err != nil || resp.StatusCode != http.StatusUnauthorized || hook == nil {
		return resp, err
	}

	// Another request may have fetched a new token while this one was on its
	// way: use it rather than asking the user again.
	if now := resolveToken(t.tokenPath); now != "" && now != sent {
		_ = resp.Body.Close()
		return send()
	}

	// Keep the 401's body, to hand back as it was if the hook fails.
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	_ = resp.Body.Close()

	token, hookErr := hook(sent)
	if hookErr != nil || token == "" {
		resp.Body = io.NopCloser(bytes.NewReader(respBody))
		return resp, nil
	}
	SetTokenOverride(token)
	return send()
}
