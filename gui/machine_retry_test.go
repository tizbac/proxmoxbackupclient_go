package main

import (
	"context"
	"testing"
	"time"
)

// TestReconnectable (the rules moved from here) is in pkg/retry.

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
