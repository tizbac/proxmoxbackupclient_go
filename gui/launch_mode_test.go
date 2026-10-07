package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/tizbac/proxmoxbackupclient_go/gui/api"
)

// Launch-time handshake: probe the local service, and if it is up but our
// token is missing/invalid, ask for elevation exactly once — pkexec/sudo on
// Linux, UAC on Windows, same code path. Refused, failed or unavailable
// elevation must land in standalone mode, never in a silent service
// connection. These tests drive the decision table with an injected probe and
// fetch, so no real prompt and no real service are involved.

// probeStub answers with the given status codes in order (sticking to the
// last one once exhausted) and counts how often it was asked.
type probeStub struct {
	codes []int
	calls int
}

func (p *probeStub) probe() int {
	p.calls++
	if p.calls <= len(p.codes) {
		return p.codes[p.calls-1]
	}
	return p.codes[len(p.codes)-1]
}

func TestResolveExecutionModeWith(t *testing.T) {
	cases := []struct {
		name          string
		force         bool
		probe         *probeStub
		fetchOK       bool
		envFetchFail  bool
		wantMode      api.ExecutionMode
		wantReason    string
		wantOverride  string
		wantProbeCall int
		wantFetchCall bool
	}{
		{
			name:    "service running with accepted token",
			probe:   &probeStub{codes: []int{200}},
			fetchOK: true, wantMode: api.ModeService, wantProbeCall: 1,
		},
		{
			name:    "no service",
			probe:   &probeStub{codes: []int{0}},
			fetchOK: true, wantMode: api.ModeStandalone, wantReason: "no_service",
			wantProbeCall: 1,
		},
		{
			name:    "service up, elevation granted",
			probe:   &probeStub{codes: []int{401, 200}},
			fetchOK: true, wantMode: api.ModeService, wantOverride: "fetched-token",
			wantProbeCall: 2, wantFetchCall: true,
		},
		{
			name:    "service up, elevation declined",
			probe:   &probeStub{codes: []int{401}},
			fetchOK: false, wantMode: api.ModeStandalone, wantReason: "auth_failed",
			wantProbeCall: 1, wantFetchCall: true,
		},
		{
			name:    "service up, elevation granted but token still rejected",
			probe:   &probeStub{codes: []int{401, 401}},
			fetchOK: true, wantMode: api.ModeStandalone, wantReason: "auth_failed",
			wantProbeCall: 2, wantFetchCall: true,
		},
		{
			name:    "launcher already tried and failed: no second prompt",
			probe:   &probeStub{codes: []int{401}},
			fetchOK: false, envFetchFail: true,
			wantMode: api.ModeStandalone, wantReason: "auth_failed",
			wantProbeCall: 1, wantFetchCall: false,
		},
		{
			name:    "forced standalone never probes or prompts",
			force:   true,
			probe:   &probeStub{codes: []int{200}},
			fetchOK: true, wantMode: api.ModeStandalone, wantReason: "forced",
			wantProbeCall: 0, wantFetchCall: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			api.SetTokenOverride("")
			t.Cleanup(func() { api.SetTokenOverride("") })
			if tc.envFetchFail {
				t.Setenv("PBSGO_TOKEN_FETCH_FAILED", "1")
			}
			// Keep a process-wide token (if any) out of the probe results.
			t.Setenv("PBSGO_API_TOKEN", "")

			fetchCalls := 0
			fetch := func() (string, error) {
				fetchCalls++
				if tc.fetchOK {
					return "fetched-token", nil
				}
				return "", errors.New("pkexec: dismissed by the user")
			}

			mode, reason := resolveExecutionModeWith(tc.force, tc.probe.probe, fetch)

			if mode != tc.wantMode {
				t.Errorf("mode = %v, want %v", mode, tc.wantMode)
			}
			if reason != tc.wantReason {
				t.Errorf("reason = %q, want %q", reason, tc.wantReason)
			}
			if got := api.GetTokenOverride(); got != tc.wantOverride {
				t.Errorf("token override = %q, want %q", got, tc.wantOverride)
			}
			if tc.probe.calls != tc.wantProbeCall {
				t.Errorf("probe calls = %d, want %d", tc.probe.calls, tc.wantProbeCall)
			}
			if tc.wantFetchCall && fetchCalls != 1 {
				t.Errorf("elevated fetch calls = %d, want 1", fetchCalls)
			}
			if !tc.wantFetchCall && fetchCalls != 0 {
				t.Errorf("elevated fetch calls = %d, want 0", fetchCalls)
			}
		})
	}
}

// A token the service refuses must not linger: the next attempt would keep
// presenting it instead of asking for a fresh one.
func TestResolveExecutionModeClearsRejectedToken(t *testing.T) {
	t.Setenv("PBSGO_API_TOKEN", "")
	api.SetTokenOverride("stale-token")
	t.Cleanup(func() { api.SetTokenOverride("") })

	_, reason := resolveExecutionModeWith(false, (&probeStub{codes: []int{401, 401}}).probe, func() (string, error) {
		return "fresh-token", nil
	})
	if reason != "auth_failed" {
		t.Fatalf("reason = %q, want auth_failed", reason)
	}
	if got := api.GetTokenOverride(); got != "" {
		t.Errorf("override = %q, want it cleared when the service still rejects it", got)
	}
}

// The refresh hook is what makes an expired/rotated token ask for elevation
// again *while the GUI is running*, instead of failing every call.
func TestNewTokenRefreshHook(t *testing.T) {
	t.Cleanup(func() { api.SetTokenOverride("") })
	api.SetTokenOverride("")

	t.Run("grants a fresh token", func(t *testing.T) {
		calls := 0
		hook := newTokenRefreshHook(func() (string, error) {
			calls++
			return "fresh-token", nil
		})
		got, err := hook("old-token")
		if err != nil || got != "fresh-token" {
			t.Fatalf("hook = (%q, %v), want fresh-token, nil", got, err)
		}
		if calls != 1 {
			t.Errorf("fetch calls = %d, want 1", calls)
		}
	})

	t.Run("one prompt per cooldown window", func(t *testing.T) {
		calls := 0
		hook := newTokenRefreshHook(func() (string, error) {
			calls++
			return "", errors.New("UAC elevation prompt was declined")
		})
		_, err := hook("old-token")
		if err == nil || !strings.Contains(err.Error(), "needs elevation") {
			t.Fatalf("err = %v, want it to say elevation is needed", err)
		}
		// Every subsequent rejection inside the cooldown must be swallowed —
		// no second prompt.
		for i := 0; i < 5; i++ {
			if _, err := hook("old-token"); err == nil {
				t.Fatalf("call %d inside cooldown: err = nil, want the refusal to stick", i)
			}
		}
		if calls != 1 {
			t.Errorf("fetch calls = %d, want 1 (only the first attempt prompts)", calls)
		}
	})

	t.Run("asks again after the cooldown", func(t *testing.T) {
		old := tokenRefreshCooldown
		tokenRefreshCooldown = 0
		t.Cleanup(func() { tokenRefreshCooldown = old })

		calls := 0
		hook := newTokenRefreshHook(func() (string, error) {
			calls++
			if calls == 1 {
				return "", errors.New("dismissed")
			}
			return "fresh-token", nil
		})
		if _, err := hook("old-token"); err == nil {
			t.Fatal("first call: err = nil, want refusal")
		}
		got, err := hook("old-token")
		if err != nil || got != "fresh-token" {
			t.Fatalf("second call = (%q, %v), want fresh-token after cooldown", got, err)
		}
		if calls != 2 {
			t.Errorf("fetch calls = %d, want 2", calls)
		}
	})

	t.Run("does not prompt when someone already refreshed the token", func(t *testing.T) {
		api.SetTokenOverride("already-refreshed")
		defer api.SetTokenOverride("")

		hook := newTokenRefreshHook(func() (string, error) {
			t.Fatal("elevated fetch must not run when the token is already fresh")
			return "", nil
		})
		got, err := hook("stale-token")
		if err != nil || got != "already-refreshed" {
			t.Fatalf("hook = (%q, %v), want the already-refreshed token", got, err)
		}
	})
}

func TestInstallTokenRefreshHookIsIdempotent(t *testing.T) {
	t.Cleanup(func() { api.SetUnauthorizedHook(nil) })

	installTokenRefreshHook()
	if api.GetUnauthorizedHook() == nil {
		t.Fatal("hook not installed")
	}
	installTokenRefreshHook()
	if api.GetUnauthorizedHook() == nil {
		t.Fatal("hook cleared by a second install")
	}
}
