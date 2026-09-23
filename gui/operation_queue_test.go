package main

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestAcquireOperationSlot_Serializes proves the core claim: a second
// operation cannot start until the first releases the slot, and it is
// notified (via onQueued) that it had to wait, naming what it's waiting on.
func TestAcquireOperationSlot_Serializes(t *testing.T) {
	var running atomic.Int32
	var maxConcurrent atomic.Int32
	var queuedFor atomic.Value // string
	var wg sync.WaitGroup

	track := func() { // record max concurrency ever observed
		n := running.Add(1)
		for {
			cur := maxConcurrent.Load()
			if n <= cur || maxConcurrent.CompareAndSwap(cur, n) {
				break
			}
		}
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		release := acquireOperationSlot("backup of alpha", nil)
		defer release()
		track()
		time.Sleep(150 * time.Millisecond)
		running.Add(-1)
	}()

	time.Sleep(30 * time.Millisecond) // let the first goroutine claim the slot first

	wg.Add(1)
	go func() {
		defer wg.Done()
		release := acquireOperationSlot("restore of alpha", func(heldBy string) {
			queuedFor.Store(heldBy)
		})
		defer release()
		track()
		running.Add(-1)
	}()

	wg.Wait()

	if got := maxConcurrent.Load(); got != 1 {
		t.Fatalf("max concurrent operations = %d, want 1 (second call should have queued, not run alongside the first)", got)
	}
	heldBy, _ := queuedFor.Load().(string)
	if heldBy != "backup of alpha" {
		t.Fatalf("onQueued reported heldBy = %q, want %q", heldBy, "backup of alpha")
	}
}

// TestAcquireOperationSlot_NoWaitNoNotify proves the queued callback is only
// invoked when the caller actually had to wait — an immediate acquire (slot
// idle) must not fire a spurious "queued" message.
func TestAcquireOperationSlot_NoWaitNoNotify(t *testing.T) {
	called := false
	release := acquireOperationSlot("backup of beta", func(string) { called = true })
	release()
	if called {
		t.Fatal("onQueued fired even though the slot was idle — should only fire when the caller had to wait")
	}
}
