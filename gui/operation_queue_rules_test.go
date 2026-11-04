package main

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// The rules of operation_queue.go: what may run side by side and what waits.

// withRunLimit sets how many operations may run at once for one test.
func withRunLimit(t *testing.T, n int) {
	t.Helper()
	old := maxConcurrentRuns
	maxConcurrentRuns = func() int { return n }
	t.Cleanup(func() { maxConcurrentRuns = old })
}

// startOp asks for an operation in the background. started is closed once it
// may run; queuedFor receives what it waited for (only if it had to wait).
type pendingOp struct {
	started   chan struct{}
	queuedFor chan string
	handle    *opHandle
	err       error
	done      chan struct{}
}

func startOp(ctx context.Context, req opRequest) *pendingOp {
	p := &pendingOp{started: make(chan struct{}), queuedFor: make(chan string, 1), done: make(chan struct{})}
	go func() {
		defer close(p.done)
		p.handle, p.err = acquireOperation(ctx, req, func(w string) { p.queuedFor <- w })
		if p.err == nil {
			close(p.started)
		}
	}()
	return p
}

func mustStart(t *testing.T, p *pendingOp, what string) {
	t.Helper()
	select {
	case <-p.started:
	case <-time.After(2 * time.Second):
		t.Fatalf("%s did not start", what)
	}
}

func mustWait(t *testing.T, p *pendingOp, what string) string {
	t.Helper()
	select {
	case <-p.started:
		t.Fatalf("%s started, but it should have waited", what)
	case w := <-p.queuedFor:
		return w
	case <-time.After(2 * time.Second):
		t.Fatalf("%s neither started nor queued", what)
	}
	return ""
}

func checkIdle(t *testing.T) {
	t.Helper()
	opMu.Lock()
	defer opMu.Unlock()
	if len(opRunning) != 0 || len(opWaiting) != 0 {
		t.Fatalf("queue not empty after the test: %d running, %d waiting", len(opRunning), len(opWaiting))
	}
}

// Three folder backups of different sets run at once; a fourth waits for a
// free place and starts as soon as one ends.
func TestQueueRunsDifferentSetsTogether(t *testing.T) {
	withRunLimit(t, 3)
	a := startOp(context.Background(), opRequest{name: "Music", kind: opBackup, group: "music"})
	b := startOp(context.Background(), opRequest{name: "Data", kind: opBackup, group: "data"})
	c := startOp(context.Background(), opRequest{name: "Software", kind: opBackup, group: "software"})
	mustStart(t, a, "Music")
	mustStart(t, b, "Data")
	mustStart(t, c, "Software")
	d := startOp(context.Background(), opRequest{name: "Home", kind: opBackup, group: "home"})
	w := mustWait(t, d, "a fourth backup")
	for _, name := range []string{"Music", "Data", "Software"} {
		if !strings.Contains(w, name) {
			t.Fatalf("waiting for %q, want it to name %s", w, name)
		}
	}
	b.handle.Release()
	mustStart(t, d, "the fourth backup once Data ended")
	a.handle.Release()
	c.handle.Release()
	d.handle.Release()
	checkIdle(t)
}

// Two runs of the same backup group never overlap, even with places free.
func TestQueueSameGroupTakesTurns(t *testing.T) {
	withRunLimit(t, 3)
	first := startOp(context.Background(), opRequest{name: "Data (Run now)", kind: opBackup, group: "data"})
	mustStart(t, first, "first Data run")
	second := startOp(context.Background(), opRequest{name: "Data", kind: opBackup, group: "data"})
	if w := mustWait(t, second, "second Data run"); w != "Data (Run now)" {
		t.Fatalf("waiting for %q, want the first run", w)
	}
	// A restore of the same group waits too.
	restore := startOp(context.Background(), opRequest{name: "Restore of data", kind: opRestore, group: "data"})
	mustWait(t, restore, "restore of the same group")
	// Another set is not held up by them.
	other := startOp(context.Background(), opRequest{name: "Music", kind: opBackup, group: "music"})
	mustStart(t, other, "Music beside Data")
	first.handle.Release()
	mustStart(t, second, "second Data run")
	second.handle.Release()
	mustStart(t, restore, "restore after the Data runs")
	restore.handle.Release()
	other.handle.Release()
	checkIdle(t)
}

// One machine backup at a time, but it runs beside folder backups.
func TestQueueOneMachineBackupBesideFolders(t *testing.T) {
	withRunLimit(t, 3)
	long := startOp(context.Background(), opRequest{name: "Music first backup", kind: opBackup, group: "music"})
	mustStart(t, long, "the long folder backup")
	m1 := startOp(context.Background(), opRequest{name: "Full Machine", kind: opMachine, group: "machine-a"})
	mustStart(t, m1, "machine backup beside the folder backup")
	m2 := startOp(context.Background(), opRequest{name: "Other disk image", kind: opMachine, group: "machine-b"})
	if w := mustWait(t, m2, "second machine backup"); w != "Full Machine" {
		t.Fatalf("second machine backup waits for %q, want Full Machine", w)
	}
	// A folder backup asked after it still starts: the machine backup waits
	// for the machine place, not for a free place.
	f := startOp(context.Background(), opRequest{name: "Data", kind: opBackup, group: "data"})
	mustStart(t, f, "folder backup asked after a waiting machine backup")
	m1.handle.Release()
	mustStart(t, m2, "second machine backup")
	for _, p := range []*pendingOp{long, m2, f} {
		p.handle.Release()
	}
	checkIdle(t)
}

// One restore at a time.
func TestQueueOneRestoreAtATime(t *testing.T) {
	withRunLimit(t, 3)
	r1 := startOp(context.Background(), opRequest{name: "Restore A", kind: opRestore, group: "a"})
	mustStart(t, r1, "first restore")
	r2 := startOp(context.Background(), opRequest{name: "Restore B", kind: opRestore, group: "b"})
	mustWait(t, r2, "second restore")
	r1.handle.Release()
	mustStart(t, r2, "second restore")
	r2.handle.Release()
	checkIdle(t)
}

// A roll back waits until everything running has ended, runs alone, and
// nothing asked after it starts before it has had its turn.
func TestQueueExclusiveRunsAlone(t *testing.T) {
	withRunLimit(t, 3)
	a := startOp(context.Background(), opRequest{name: "Music", kind: opBackup, group: "music"})
	mustStart(t, a, "Music")
	rb := startOp(context.Background(), opRequest{name: "Roll back: Data", kind: opExclusive})
	if w := mustWait(t, rb, "roll back"); w != "Music" {
		t.Fatalf("roll back waits for %q, want Music", w)
	}
	later := startOp(context.Background(), opRequest{name: "Software", kind: opBackup, group: "software"})
	if w := mustWait(t, later, "backup asked after the roll back"); w != "Roll back: Data" {
		t.Fatalf("later backup waits for %q, want the roll back", w)
	}
	a.handle.Release()
	mustStart(t, rb, "roll back")
	select {
	case <-later.started:
		t.Fatal("a backup started while the roll back ran")
	case <-time.After(100 * time.Millisecond):
	}
	rb.handle.Release()
	mustStart(t, later, "the later backup after the roll back")
	later.handle.Release()
	checkIdle(t)
}

// Waiting runs start in the order they asked, one place at a time.
func TestQueueKeepsOrder(t *testing.T) {
	withRunLimit(t, 1)
	first := startOp(context.Background(), opRequest{name: "first", kind: opBackup, group: "1"})
	mustStart(t, first, "first")
	var order []string
	var mu sync.Mutex
	var waiters []*pendingOp
	for _, n := range []string{"second", "third", "fourth"} {
		p := startOp(context.Background(), opRequest{name: n, kind: opBackup, group: n})
		mustWait(t, p, n)
		waiters = append(waiters, p)
		n := n
		go func() {
			<-p.started
			mu.Lock()
			order = append(order, n)
			mu.Unlock()
			time.Sleep(20 * time.Millisecond)
			p.handle.Release()
		}()
	}
	first.handle.Release()
	for _, p := range waiters {
		<-p.done
	}
	time.Sleep(100 * time.Millisecond)
	mu.Lock()
	got := strings.Join(order, ",")
	mu.Unlock()
	if got != "second,third,fourth" {
		t.Fatalf("started in order %s, want second,third,fourth", got)
	}
	checkIdle(t)
}

// Stop on a run still waiting ends its wait at once and gives up its place.
func TestQueueStopWhileQueued(t *testing.T) {
	withRunLimit(t, 1)
	a := startOp(context.Background(), opRequest{name: "Music", kind: opBackup, group: "music"})
	mustStart(t, a, "Music")
	ctx, cancel := context.WithCancel(context.Background())
	b := startOp(ctx, opRequest{name: "Data", kind: opBackup, group: "data"})
	mustWait(t, b, "Data")
	cancel()
	select {
	case <-b.done:
	case <-time.After(2 * time.Second):
		t.Fatal("a stopped run kept waiting")
	}
	if !errors.Is(b.err, context.Canceled) {
		t.Fatalf("stopped run got %v, want context.Canceled", b.err)
	}
	c := startOp(context.Background(), opRequest{name: "Software", kind: opBackup, group: "software"})
	mustWait(t, c, "Software")
	a.handle.Release()
	mustStart(t, c, "Software, not the stopped Data")
	c.handle.Release()
	checkIdle(t)
}

// A limit of 1 runs everything one at a time, as before.
func TestQueueLimitOneIsOneAtATime(t *testing.T) {
	withRunLimit(t, 1)
	a := startOp(context.Background(), opRequest{name: "A", kind: opBackup, group: "a"})
	mustStart(t, a, "A")
	b := startOp(context.Background(), opRequest{name: "B", kind: opMachine, group: "b"})
	mustWait(t, b, "B")
	a.handle.Release()
	mustStart(t, b, "B")
	b.handle.Release()
	checkIdle(t)
}

// The case this is for: a first backup of a large set runs for hours, and a
// set every few minutes (and a machine backup) still get their runs. The
// long backup never ends during the test.
func TestQueueLongFirstBackupDoesNotBlockScheduledRuns(t *testing.T) {
	withRunLimit(t, 3)
	long := startOp(context.Background(), opRequest{name: "Music first backup", kind: opBackup, group: "music"})
	mustStart(t, long, "the first backup")
	for i := 0; i < 10; i++ {
		data := startOp(context.Background(), opRequest{name: "Data", kind: opBackup, group: "data"})
		mustStart(t, data, "scheduled Data run")
		if i%3 == 0 {
			m := startOp(context.Background(), opRequest{name: "Full Machine", kind: opMachine, group: "machine"})
			mustStart(t, m, "machine backup")
			m.handle.Release()
		}
		data.handle.Release()
	}
	long.handle.Release()
	checkIdle(t)
}

// An exit or shutdown after a set's run waits for the runs already going,
// not for ones that ask later.
func TestWaitForEarlierOperations(t *testing.T) {
	withRunLimit(t, 3)
	a := startOp(context.Background(), opRequest{name: "Music", kind: opBackup, group: "music"})
	mustStart(t, a, "Music")
	waited := make(chan struct{})
	go func() {
		waitForEarlierOperations()
		close(waited)
	}()
	time.Sleep(50 * time.Millisecond)
	later := startOp(context.Background(), opRequest{name: "Data", kind: opBackup, group: "data"})
	mustStart(t, later, "Data")
	select {
	case <-waited:
		t.Fatal("the wait ended while Music still ran")
	case <-time.After(100 * time.Millisecond):
	}
	a.handle.Release()
	select {
	case <-waited:
	case <-time.After(2 * time.Second):
		t.Fatal("the wait went on for a run that started after it")
	}
	later.handle.Release()
	checkIdle(t)
}

// A set's next run never starts in the second its last one ended: PBS
// refuses a second snapshot of a group with the same backup time. Another
// group, and a restore, do not wait.
func TestQueueNewSecondForTheSameGroup(t *testing.T) {
	withRunLimit(t, 3)
	for i := 0; i < 3; i++ {
		a := startOp(context.Background(), opRequest{name: "Quick set", kind: opBackup, group: "quick"})
		mustStart(t, a, "first run")
		a.handle.Release()
		ended := time.Now().Unix()
		other := time.Now()
		o := startOp(context.Background(), opRequest{name: "Other set", kind: opBackup, group: "other"})
		mustStart(t, o, "another set")
		if d := time.Since(other); d > 200*time.Millisecond {
			t.Errorf("another group waited %s", d)
		}
		o.handle.Release()
		b := startOp(context.Background(), opRequest{name: "Quick set", kind: opBackup, group: "quick"})
		mustStart(t, b, "next run")
		if started := time.Now().Unix(); started <= ended {
			t.Fatalf("next run of the group started in second %d, the one its last run ended in", started)
		}
		b.handle.Release()
	}
	// Stop while it waits for the next second.
	a := startOp(context.Background(), opRequest{name: "Quick set", kind: opBackup, group: "quick"})
	mustStart(t, a, "run")
	a.handle.Release()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	h, err := acquireOperation(ctx, opRequest{name: "Quick set", kind: opBackup, group: "quick"}, nil)
	if err == nil {
		// Only when the second had already turned (nothing to wait for).
		h.Release()
	} else if !errors.Is(err, context.Canceled) {
		t.Errorf("stopped while waiting for the next second: %v", err)
	}
	time.Sleep(1100 * time.Millisecond)
	checkIdle(t)
}

func TestClampRunLimit(t *testing.T) {
	for in, want := range map[int]int{-1: DefaultMaxConcurrentRuns, 0: DefaultMaxConcurrentRuns, 1: 1, 3: 3, 8: 8, 9: MaxConcurrentRunsLimit, 100: MaxConcurrentRunsLimit} {
		if got := clampRunLimit(in); got != want {
			t.Errorf("clampRunLimit(%d) = %d, want %d", in, got, want)
		}
	}
}

func TestQueuedMessageMatchesTheWindow(t *testing.T) {
	// App.jsx: /^Queued: (.+) — waiting for .+ to finish\.\.\.$/
	msg := queuedMessage("Data", describeBlockers([]*opTicket{{opRequest: opRequest{name: "Music"}}, {opRequest: opRequest{name: "Full Machine"}}}))
	if msg != "Queued: Data — waiting for Music and Full Machine to finish..." {
		t.Fatalf("queued message %q", msg)
	}
}
