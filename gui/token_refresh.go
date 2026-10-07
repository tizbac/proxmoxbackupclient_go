package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/tizbac/proxmoxbackupclient_go/gui/api"
)

// tokenRefreshCooldown is how long an elevation refusal is remembered, so a
// burst of requests rejected by a rotated token produces ONE prompt instead of
// one per call (and a user who just declined is not nagged by every retry).
// A var (not const) so tests can shrink it.
var tokenRefreshCooldown = 60 * time.Second

// installTokenRefreshHook teaches the API client how to re-authenticate after
// the service rotated its token: on 401 the transport calls this hook, which
// repeats the elevated fetch (pkexec/sudo on Linux, UAC on Windows) and hands
// the new token back. Idempotent, and installed only for a GUI that actually
// talks to the service — a standalone session must never be prompted just
// because a service exists (declining at launch already said "standalone").
func installTokenRefreshHook() {
	if api.GetUnauthorizedHook() != nil {
		return
	}
	api.SetUnauthorizedHook(newTokenRefreshHook(elevatedFetchTokenWithHandoff))
	writeDebugLog("token refresh hook installed: a rotated service token re-asks for elevation instead of failing every call")
}

// newTokenRefreshHook builds the callback installed via api.SetUnauthorizedHook. fetch is the
// elevated token fetch, injected so tests can drive the refusal/cooldown paths
// without pkexec or UAC.
func newTokenRefreshHook(fetch func() (string, error)) func(sentToken string) (string, error) {
	var mu sync.Mutex
	var refusedAt time.Time

	return func(sent string) (string, error) {
		mu.Lock()
		defer mu.Unlock()

		// Another goroutine may have refreshed the token while we waited on
		// the lock: use it instead of prompting a second time.
		if cur := api.GetTokenOverride(); cur != "" && cur != sent {
			return cur, nil
		}
		if !refusedAt.IsZero() && time.Since(refusedAt) < tokenRefreshCooldown {
			return "", fmt.Errorf("elevation to refresh the service token was declined; restart the application to be asked again (it will then start standalone)")
		}

		token, err := fetch()
		if err != nil {
			refusedAt = time.Now()
			return "", fmt.Errorf("the service token expired and refreshing it needs elevation: %w", err)
		}
		return token, nil
	}
}
