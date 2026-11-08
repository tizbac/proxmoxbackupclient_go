package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// What may run at once in this process. Backups used to take one single slot,
// so a long first backup of one set (a day and more for terabytes) held up
// every other set on the machine, the hourly ones included. They now run side
// by side within these rules:
//
//   - at most maxConcurrentRuns() operations at a time (Settings, default 3);
//   - one machine (whole-disk) backup at a time;
//   - one restore at a time (the window follows, stops and watches one);
//   - two operations on the same PBS backup group never overlap: PBS's own
//     group lock did not reliably keep a backup and a restore (or two
//     backups) of one group apart (2026-09-23), so they take turns here;
//   - a roll back or its undo runs alone, as it moves files about.
//
// Whatever cannot start yet waits, in the order it asked. One waiting for a
// free place holds back those behind it (they want the same place); one
// waiting for its group or for the machine place does not, so a set waiting
// behind its own earlier run never stops the other sets. An operation that
// must run alone stops anything later from starting, so it is not kept
// waiting for ever.
//
// This only covers operations in THIS process. With "Run as Service", a
// restore started from the window runs in the GUI process, which the
// service's queue cannot see (as before).

// opKind is what an operation is, for the rules above.
type opKind int

const (
	opBackup    opKind = iota // a folder backup
	opMachine                 // a machine (whole-disk) backup
	opRestore                 // a restore
	opExclusive               // a roll back or its undo: runs alone
)

// opRequest describes an operation asking to start.
type opRequest struct {
	name  string // shown in the queue message and the tray
	kind  opKind
	group string // its PBS backup group (opGroupKey), "" for none
}

type opTicket struct {
	opRequest
	seq      uint64
	admitted chan struct{} // closed when the operation may start
}

// DefaultMaxConcurrentRuns is how many backups and restores may run at once
// when the setting has not been changed. 3 lets a long first backup and a
// machine backup run while the other sets still get their turn.
const DefaultMaxConcurrentRuns = 3

// MaxConcurrentRunsLimit is the most the setting allows.
const MaxConcurrentRunsLimit = 8

var (
	opMu      sync.Mutex
	opCond    = sync.NewCond(&opMu) // signalled when an operation ends
	opSeq     uint64
	opRunning []*opTicket
	opWaiting []*opTicket

	// groupEnded is the Unix second each backup group's last backup in this
	// process ended. A snapshot's backup time is in whole seconds and PBS
	// refuses a second snapshot of a group in the same second (HTTP 400 on
	// the catalog index), which happened when a quick run of a set was
	// followed at once by its next one (found on a Linux test machine, 2026-11-01).
	groupEnded = map[string]int64{}

	// maxConcurrentRuns is the limit in force; the app points it at its
	// config (setRunLimitSource).
	maxConcurrentRuns = func() int { return DefaultMaxConcurrentRuns }
)

// clampRunLimit keeps a configured limit within 1..MaxConcurrentRunsLimit;
// 0 (not set) is the default.
func clampRunLimit(n int) int {
	switch {
	case n <= 0:
		return DefaultMaxConcurrentRuns
	case n > MaxConcurrentRunsLimit:
		return MaxConcurrentRunsLimit
	}
	return n
}

// opGroupKey names a PBS backup group: server, datastore, namespace, backup
// type ("host" or "vm") and backup-id.
func opGroupKey(baseURL, datastore, namespace, backupType, backupID string) string {
	if backupType == "" {
		backupType = "host"
	}
	return strings.Join([]string{strings.TrimRight(baseURL, "/"), datastore, namespace, backupType, backupID}, "|")
}

// opHandle is a started operation. Release it when the operation ends.
type opHandle struct {
	t    *opTicket
	once sync.Once
}

// Release ends the operation and lets the next ones start. Safe to call more
// than once and on a nil handle.
func (h *opHandle) Release() {
	if h == nil {
		return
	}
	h.once.Do(func() {
		opMu.Lock()
		if makesSnapshot(h.t) {
			groupEnded[h.t.group] = time.Now().Unix()
		}
		opRunning = removeTicket(opRunning, h.t)
		admitWaitingLocked()
		opCond.Broadcast()
		opMu.Unlock()
	})
}

// makesSnapshot reports whether t is a backup that makes a snapshot of its
// group (a restore only reads one).
func makesSnapshot(t *opTicket) bool {
	return t.group != "" && (t.kind == opBackup || t.kind == opMachine)
}

// waitNewSecond holds a backup whose group's previous backup ended in this
// very second until the next one starts, so the new snapshot's backup time
// is later than the last one's (see groupEnded).
func waitNewSecond(ctx context.Context, t *opTicket) error {
	if !makesSnapshot(t) {
		return nil
	}
	opMu.Lock()
	last, ok := groupEnded[t.group]
	opMu.Unlock()
	if !ok {
		return nil
	}
	next := time.Unix(last+1, 0)
	if !time.Now().Before(next) {
		return nil
	}
	tm := time.NewTimer(time.Until(next))
	defer tm.Stop()
	select {
	case <-tm.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// acquireOperation waits until the operation may start under the rules
// above, then returns its handle. onQueued (may be nil) is called once, only
// if it has to wait, with what it waits for. A cancelled ctx ends the wait
// with ctx.Err(), so Stop works on a run that is still queued.
func acquireOperation(ctx context.Context, req opRequest, onQueued func(waitingFor string)) (*opHandle, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	opMu.Lock()
	opSeq++
	t := &opTicket{opRequest: req, seq: opSeq, admitted: make(chan struct{})}
	opWaiting = append(opWaiting, t)
	admitWaitingLocked()
	waitingFor := ""
	if !ticketAdmitted(t) {
		waitingFor = describeBlockers(blockersLocked(t, clampRunLimit(maxConcurrentRuns()), waitingAhead(t)))
	}
	opMu.Unlock()

	if waitingFor != "" {
		if onQueued != nil {
			onQueued(waitingFor)
		}
		select {
		case <-t.admitted:
		case <-ctx.Done():
			opMu.Lock()
			if ticketAdmitted(t) {
				opRunning = removeTicket(opRunning, t)
			} else {
				opWaiting = removeTicket(opWaiting, t)
			}
			admitWaitingLocked()
			opCond.Broadcast()
			opMu.Unlock()
			return nil, ctx.Err()
		}
	}
	h := &opHandle{t: t}
	if err := waitNewSecond(ctx, t); err != nil {
		h.Release()
		return nil, err
	}
	return h, nil
}

// acquireOperationSlot is acquireOperation for a caller that cannot be
// stopped while it waits; it returns the release function.
func acquireOperationSlot(req opRequest, onQueued func(waitingFor string)) func() {
	h, _ := acquireOperation(context.Background(), req, onQueued)
	return h.Release
}

func ticketAdmitted(t *opTicket) bool {
	select {
	case <-t.admitted:
		return true
	default:
		return false
	}
}

func removeTicket(list []*opTicket, t *opTicket) []*opTicket {
	for i, x := range list {
		if x == t {
			return append(list[:i:i], list[i+1:]...)
		}
	}
	return list
}

// waitingAhead returns the operations still waiting that asked before t.
func waitingAhead(t *opTicket) []*opTicket {
	var ahead []*opTicket
	for _, w := range opWaiting {
		if w.seq < t.seq {
			ahead = append(ahead, w)
		}
	}
	return ahead
}

// blockersLocked lists what stops t starting now: running operations, or an
// operation that runs alone waiting ahead of it. Empty means it may start.
func blockersLocked(t *opTicket, limit int, ahead []*opTicket) []*opTicket {
	for _, w := range ahead {
		if w.kind == opExclusive {
			return []*opTicket{w}
		}
	}
	if len(opRunning) == 0 {
		return nil
	}
	if t.kind == opExclusive || len(opRunning) >= limit {
		return opRunning
	}
	var out []*opTicket
	for _, r := range opRunning {
		switch {
		case r.kind == opExclusive,
			t.kind == opMachine && r.kind == opMachine,
			t.kind == opRestore && r.kind == opRestore,
			t.group != "" && r.group == t.group:
			out = append(out, r)
		}
	}
	return out
}

// admitWaitingLocked starts every waiting operation that may start, oldest
// first. Called with opMu held whenever the running or waiting list changes.
func admitWaitingLocked() {
	limit := clampRunLimit(maxConcurrentRuns())
	var still []*opTicket
	for _, t := range opWaiting {
		if len(blockersLocked(t, limit, still)) > 0 {
			still = append(still, t)
			continue
		}
		opRunning = append(opRunning, t)
		close(t.admitted)
	}
	opWaiting = still
}

// describeBlockers names what an operation waits for, for "Queued: X —
// waiting for <this> to finish...".
func describeBlockers(list []*opTicket) string {
	var names []string
	seen := map[string]bool{}
	for _, b := range list {
		if !seen[b.name] {
			seen[b.name] = true
			names = append(names, b.name)
		}
	}
	switch len(names) {
	case 0:
		return "another backup"
	case 1:
		return names[0]
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}

// queuedMessage is the progress line of an operation that has to wait. The
// window recognises this exact wording (QUEUED_MSG_RE in App.jsx).
func queuedMessage(name, waitingFor string) string {
	return fmt.Sprintf("Queued: %s — waiting for %s to finish...", name, waitingFor)
}

// currentOperationLabel names whatever is running in this process ("" when
// idle). The tray shows it and keeps the idle tooltip off while it is set.
func currentOperationLabel() string {
	opMu.Lock()
	defer opMu.Unlock()
	if len(opRunning) == 0 {
		return ""
	}
	return describeBlockers(opRunning)
}

// runningOperationCount is how many operations are running in this process.
func runningOperationCount() int {
	opMu.Lock()
	defer opMu.Unlock()
	return len(opRunning)
}

// waitForEarlierOperations returns once every operation that had asked to
// start by the time it was called (running or still waiting) has ended. A
// set that exits the app or shuts the computer down afterwards waits here so
// it does not cut short the backups running beside it; operations that ask
// later (the next hour's run) are not waited for.
func waitForEarlierOperations() {
	opMu.Lock()
	defer opMu.Unlock()
	horizon := opSeq
	for {
		pending := false
		for _, list := range [][]*opTicket{opRunning, opWaiting} {
			for _, t := range list {
				if t.seq <= horizon {
					pending = true
				}
			}
		}
		if !pending {
			return
		}
		opCond.Wait()
	}
}
