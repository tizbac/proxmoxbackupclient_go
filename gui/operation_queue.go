package main

import "sync"

// Only one backup or restore may run at a time — whether it's a scheduled
// job's automatic fire, a manual one-off backup, "Run Now" on a backup set,
// or a restore. PBS's own per-backup-group/session locking doesn't reliably
// serialize an overlapping backup+restore or backup+backup against the same
// backup-id (investigated 2026-09-23 after a restore wedged on a stuck
// reader session — timing ruled out an actual clash that time, but nothing
// in the app stopped one from happening on a future run), so we serialize
// client-side instead. Whichever operation asks second queues — it blocks
// until the first one finishes, then runs automatically — rather than
// running concurrently (risking contention) or being refused outright.
//
// This is a single process-wide slot, so it only serializes operations that
// run in THIS process. In "service mode" a scheduled backup executes inside
// the separate Windows service process while a manual restore run from the
// GUI still executes inline in the GUI process (restore has no
// service-dispatch path today) — those two are NOT covered by this lock.
// Fixing that would mean routing restore through the service the same way
// backup already is; out of scope here, flagged as a known gap.
var (
	operationMu    sync.Mutex
	operationHeld  string // human label of whatever currently holds the slot; "" when idle
	operationHeldMu sync.Mutex
)

// acquireOperationSlot blocks until any currently-running backup/restore in
// this process finishes, then claims the slot for `name` (used only for the
// "queued" message a caller shows while waiting). onQueued fires at most
// once, and only if this call actually had to wait, so a caller can surface
// a real "queued, waiting for X" status instead of just going quiet with no
// explanation. The returned func releases the slot — always defer it
// immediately after a successful call.
func acquireOperationSlot(name string, onQueued func(heldBy string)) func() {
	if !operationMu.TryLock() {
		if onQueued != nil {
			operationHeldMu.Lock()
			heldBy := operationHeld
			operationHeldMu.Unlock()
			onQueued(heldBy)
		}
		operationMu.Lock()
	}
	operationHeldMu.Lock()
	operationHeld = name
	operationHeldMu.Unlock()
	return func() {
		operationHeldMu.Lock()
		operationHeld = ""
		operationHeldMu.Unlock()
		operationMu.Unlock()
	}
}
