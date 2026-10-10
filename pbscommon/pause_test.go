package pbscommon

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPauseGateNilNeverBlocks(t *testing.T) {
	var g *PauseGate
	if err := g.Wait(context.Background()); err != nil {
		t.Fatalf("nil gate: %v", err)
	}
	if p, _ := g.Paused(); p {
		t.Fatal("nil gate reports paused")
	}
}

func TestPauseGateBlocksUntilResume(t *testing.T) {
	g := &PauseGate{}
	if !g.Pause() {
		t.Fatal("first Pause returned false")
	}
	if g.Pause() {
		t.Fatal("second Pause returned true")
	}
	done := make(chan error, 1)
	go func() { done <- g.Wait(context.Background()) }()
	select {
	case err := <-done:
		t.Fatalf("Wait returned while paused: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	d, ok := g.Resume()
	if !ok || d <= 0 {
		t.Fatalf("Resume = %v, %v", d, ok)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Wait after Resume: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Wait still blocked after Resume")
	}
	if _, ok := g.Resume(); ok {
		t.Fatal("Resume of a running gate returned true")
	}
	if err := g.Wait(context.Background()); err != nil {
		t.Fatalf("Wait on an open gate: %v", err)
	}
}

func TestPauseGateStopWhilePaused(t *testing.T) {
	g := &PauseGate{}
	g.Pause()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- g.Wait(ctx) }()
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Wait = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Stop did not end the pause")
	}
	if p, _ := g.Paused(); !p {
		t.Fatal("cancelling the run should not clear the pause by itself")
	}
}
