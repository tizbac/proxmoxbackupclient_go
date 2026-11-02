package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestMachineRetryable(t *testing.T) {
	for _, tc := range []struct {
		err  string
		want bool
	}{
		// The vega failure of 2026-10-24, as Backup returned it.
		{`backup device /dev/sdb: failed to upload chunk fb73...: Post "https://192.168.0.197:8007/fixed_chunk?...": http2: client connection lost`, true},
		{"dial tcp 192.168.0.197:8007: connect: no route to host", true},
		{"dial tcp 192.168.0.197:8007: i/o timeout", true},
		{"Error making request: unable to acquire lock on backup group directory - another backup is already running", true},
		{"read /dev/elastio-snap1 at segment 1048576: failed to read block: input/output error", false},
		{"no usable block snapshot module loaded", false},
		{"HTTP 401 - permission check failed", false},
	} {
		if got := machineRetryable(errors.New(tc.err)); got != tc.want {
			t.Errorf("machineRetryable(%q) = %v, want %v", tc.err, got, tc.want)
		}
	}
}

func TestWaitOrCancel(t *testing.T) {
	if !waitOrCancel(context.Background(), time.Millisecond) {
		t.Error("an uncancelled wait should report true")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	if waitOrCancel(ctx, time.Hour) {
		t.Error("a cancelled wait should report false")
	}
	if time.Since(start) > time.Second {
		t.Error("a cancelled wait should return at once")
	}
}
