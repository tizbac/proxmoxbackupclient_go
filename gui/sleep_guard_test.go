package main

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

// fakeSystem stands in for the operating system's sleep request.
type fakeSystem struct {
	acquired, released atomic.Int32
	fail               atomic.Bool
	reasons            []string
	mu                 sync.Mutex
}

func (f *fakeSystem) install(t *testing.T) {
	t.Helper()
	old := acquireSleepLock
	acquireSleepLock = func(reason string) (func(), error) {
		if f.fail.Load() {
			return nil, errors.New("no permission")
		}
		f.acquired.Add(1)
		f.mu.Lock()
		f.reasons = append(f.reasons, reason)
		f.mu.Unlock()
		return func() { f.released.Add(1) }, nil
	}
	t.Cleanup(func() {
		acquireSleepLock = old
		sleepGuard.mu.Lock()
		sleepGuard.holders, sleepGuard.release = 0, nil
		sleepGuard.mu.Unlock()
	})
}

func TestOneRunTakesTheRequestAndGivesItBack(t *testing.T) {
	var f fakeSystem
	f.install(t)
	end, err := preventSleep("a backup is running")
	if err != nil {
		t.Fatal(err)
	}
	if f.acquired.Load() != 1 || f.released.Load() != 0 {
		t.Fatalf("while the run lasts: acquired %d, released %d", f.acquired.Load(), f.released.Load())
	}
	end()
	if f.released.Load() != 1 {
		t.Fatalf("the request was not given back: released %d", f.released.Load())
	}
}

func TestRunsAtOnceShareOneRequestAndTheLastOneEndsIt(t *testing.T) {
	var f fakeSystem
	f.install(t)
	a, _ := preventSleep("a")
	b, _ := preventSleep("b")
	c, _ := preventSleep("c")
	if f.acquired.Load() != 1 {
		t.Fatalf("three runs took %d requests, want one shared", f.acquired.Load())
	}
	a()
	b()
	if f.released.Load() != 0 {
		t.Fatalf("the computer may sleep while a run still goes on (released %d)", f.released.Load())
	}
	c()
	if f.released.Load() != 1 {
		t.Fatalf("the last run did not end the request (released %d)", f.released.Load())
	}
	// a later run takes it again
	d, _ := preventSleep("d")
	if f.acquired.Load() != 2 {
		t.Fatalf("a new run after the others did not take a request (acquired %d)", f.acquired.Load())
	}
	d()
}

func TestEndingTwiceDoesNotGiveBackAnotherRunsRequest(t *testing.T) {
	var f fakeSystem
	f.install(t)
	a, _ := preventSleep("a")
	b, _ := preventSleep("b")
	a()
	a() // a second call must not count as b ending
	a()
	if f.released.Load() != 0 {
		t.Fatalf("ending one run twice released the request of the other (released %d)", f.released.Load())
	}
	b()
	if f.released.Load() != 1 {
		t.Fatalf("released %d", f.released.Load())
	}
}

func TestARefusedRequestDoesNotStopTheRunAndIsRetriedByTheNext(t *testing.T) {
	var f fakeSystem
	f.install(t)
	f.fail.Store(true)
	end, err := preventSleep("a")
	if err == nil {
		t.Fatal("a refusal was not reported")
	}
	if end == nil {
		t.Fatal("no end function for a refused request: the run would crash on it")
	}
	end() // must be safe
	f.fail.Store(false)
	next, err := preventSleep("b")
	if err != nil || f.acquired.Load() != 1 {
		t.Fatalf("the next run did not get the request (err %v, acquired %d)", err, f.acquired.Load())
	}
	next()
}

func TestSleepRequestHoldersAreRaceFree(t *testing.T) {
	var f fakeSystem
	f.install(t)
	var wg sync.WaitGroup
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				end, _ := preventSleep("x")
				end()
			}
		}()
	}
	wg.Wait()
	if f.acquired.Load() != f.released.Load() {
		t.Fatalf("acquired %d, released %d: a request leaked", f.acquired.Load(), f.released.Load())
	}
	sleepGuard.mu.Lock()
	defer sleepGuard.mu.Unlock()
	if sleepGuard.holders != 0 || sleepGuard.release != nil {
		t.Fatalf("holders %d, release set %v after every run ended", sleepGuard.holders, sleepGuard.release != nil)
	}
}

func TestSleepReasonNamesTheProduct(t *testing.T) {
	if r := sleepReason(); len(r) < 10 || r[len(r)-len("a backup is running"):] != "a backup is running" {
		t.Fatalf("reason %q", r)
	}
}

// A run that fails at once (nothing to back up, bad settings) must still end the request.
func TestAFailedRunEndsTheSleepRequest(t *testing.T) {
	var f fakeSystem
	f.install(t)
	err := RunBackupInline(BackupOptions{})
	if err == nil {
		t.Skip("an empty set of options was accepted: this test needs a run that fails")
	}
	if f.acquired.Load() != 1 {
		t.Fatalf("the run did not ask to keep the computer awake (acquired %d)", f.acquired.Load())
	}
	if f.released.Load() != 1 {
		t.Fatalf("the failed run left the request standing (released %d)", f.released.Load())
	}
}
