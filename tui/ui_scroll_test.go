package tui

import (
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
)

// longListBackend is the demo backend with more backups and snapshots than a
// window shows, answered after a moment, as a real server does: the "reading
// ..." line is on screen first.
type longListBackend struct {
	*demoBackend
	groups, snaps int
	runs          atomic.Int64 // the length of the history; 0 leaves the demo's
}

func (b *longListBackend) History() ([]HistoryEntry, error) {
	n := int(b.runs.Load())
	if n == 0 {
		return b.demoBackend.History()
	}
	at := time.Date(2026, 1, 1, 12, 0, 0, 0, time.Local)
	out := make([]HistoryEntry, 0, n)
	for i := 1; i <= n; i++ {
		out = append(out, HistoryEntry{At: at.Add(-time.Duration(i) * time.Hour), Name: fmt.Sprintf("set-%03d", i), Status: "success", Message: "ok"})
	}
	return out, nil
}

func (b *longListBackend) RestoreGroups(serverID string) ([]Group, error) {
	time.Sleep(400 * time.Millisecond)
	now := time.Now()
	out := make([]Group, 0, b.groups)
	for i := 1; i <= b.groups; i++ {
		out = append(out, Group{Type: "host", ID: fmt.Sprintf("pc-%03d", i), Count: 3, Latest: now.Add(-time.Duration(i) * time.Hour)})
	}
	return out, nil
}

func (b *longListBackend) RestoreSnapshots(serverID string, g Group) ([]SnapshotInfo, error) {
	time.Sleep(400 * time.Millisecond)
	out := make([]SnapshotInfo, 0, b.snaps)
	for i := 0; i < b.snaps; i++ {
		out = append(out, SnapshotInfo{
			Type: g.Type, BackupID: g.ID, Time: time.Date(2026, 1, 1, 12, 0, 0, 0, time.Local).Add(-time.Duration(i) * 24 * time.Hour),
			Archives: []string{"backup.pxar.didx"}, Size: 1e6,
		})
	}
	return out, nil
}

// A list longer than the window opens at its start with the first row
// selected. It used to open scrolled to its end: the table had been drawn short
// (the "reading the server..." line) and went on following its end once filled.
func TestRestoreListsLongerThanTheWindowOpenAtTheirStart(t *testing.T) {
	be := &longListBackend{demoBackend: newDemoBackend(), groups: 80, snaps: 90}
	h := newHarnessWith(t, be, 130, 30)
	h.typ('6')
	h.waitFor("reading the server")
	h.waitFor("host/pc-0")
	s := h.screen()
	if !strings.Contains(s, "host/pc-001") || strings.Contains(s, "host/pc-080") {
		t.Fatalf("the list of backups does not open at its start:\n%s", s)
	}
	if row, _ := h.a.rs.groupsTable.GetSelection(); row != 1 {
		t.Errorf("row %d is selected, not the first", row)
	}

	// the list scrolls with the selection, and End reaches its last row
	h.key(tcell.KeyEnd, 0)
	h.waitFor("host/pc-080")
	h.key(tcell.KeyHome, 0)
	h.waitFor("host/pc-001")

	// the snapshots of a backup: newest first, the newest on screen
	h.key(tcell.KeyEnter, 0)
	h.waitFor("reading the snapshots")
	h.waitGone("reading the snapshots")
	s = h.screen()
	if !strings.Contains(s, " 1 Jan 2026") {
		t.Fatalf("the list of snapshots does not open at its start:\n%s", s)
	}
	if row, _ := h.a.rs.snapTable.GetSelection(); row != 1 {
		t.Errorf("snapshot row %d is selected, not the first", row)
	}

	// back, and the list read again as picking a server does: from the start
	// again (opening the screen reads it twice, which hid this there)
	h.key(tcell.KeyEsc, 0)
	h.waitFor("host/pc-0")
	h.key(tcell.KeyEnd, 0)
	h.waitFor("host/pc-080")
	h.onUI(h.a.rs.loadGroups)
	h.waitFor("reading the server")
	h.waitFor("host/pc-001")
	if strings.Contains(h.screen(), "host/pc-080") {
		t.Errorf("the list read again does not open at its start:\n%s", h.screen())
	}
}

// A list refreshed in place keeps the place it was scrolled to and its selected
// row, and does not jump to its end when it grows past the window.
func TestRefreshedListKeepsItsPlace(t *testing.T) {
	be := &longListBackend{demoBackend: newDemoBackend()}
	be.runs.Store(2)
	h := newHarnessWith(t, be, 130, 30)
	h.typ('3')
	h.waitFor("Result")
	grow := func(n int64) {
		be.runs.Store(n)
		h.a.refresh(true) // what the window does every ten seconds
		time.Sleep(200 * time.Millisecond)
	}
	grow(2) // shorter than the box: tview starts following the end here
	h.waitFor("set-002")
	grow(120)
	s := h.screen()
	if !strings.Contains(s, "set-001") || strings.Contains(s, "set-120") {
		t.Fatalf("the list jumped to its end when it grew:\n%s", s)
	}
	// scrolled down, then refreshed: the same rows stay on screen
	for i := 0; i < 60; i++ {
		h.sim.InjectKey(tcell.KeyDown, 0, tcell.ModNone)
	}
	h.waitFor("set-061")
	rows := func() string {
		var out []string
		for _, l := range strings.Split(h.screen(), "\n") {
			if strings.Contains(l, "set-") {
				out = append(out, l)
			}
		}
		return strings.Join(out, "\n")
	}
	before := rows()
	grow(120)
	if after := rows(); after != before {
		t.Errorf("a refresh moved the list:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

// waitGone waits until a text has left the screen.
func (h *harness) waitGone(s string) {
	h.t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for strings.Contains(h.screen(), s) {
		if time.Now().After(deadline) {
			h.t.Fatalf("%q is still on screen:\n%s", s, h.screen())
		}
		time.Sleep(50 * time.Millisecond)
	}
}
