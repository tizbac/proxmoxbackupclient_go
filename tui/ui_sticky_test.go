package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
)

func (h *harness) statusLine() string {
	lines := strings.Split(h.screen(), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) != "" && !strings.Contains(lines[i], "Ctrl+Q") {
			return lines[i]
		}
	}
	return ""
}

// age makes the message on the status line look as old as it is said to be.
func (h *harness) age(d time.Duration) {
	h.a.mu.Lock()
	h.a.flashAt = time.Now().Add(-d)
	h.a.mu.Unlock()
	h.a.app.QueueUpdateDraw(h.a.renderChrome)
	time.Sleep(200 * time.Millisecond)
}

func TestAResultStaysUntilTheNextAction(t *testing.T) {
	h := newHarness(t, 100, 30)
	h.waitFor("service running")
	h.onUI(func() { h.a.setFlash("connection to Main PBS failed: HTTP 401 (wrong login)") })
	h.waitFor("connection to Main PBS failed")
	h.age(10 * time.Minute) // far beyond the old six seconds
	if !strings.Contains(h.screen(), "connection to Main PBS failed") {
		t.Fatalf("the result cleared itself:\n%s", h.screen())
	}
	// it says when it happened (ten minutes ago, as aged), so it cannot pass for a current message
	h.a.mu.Lock()
	want := h.a.flashAt.Format("15:04")
	h.a.mu.Unlock()
	if !strings.Contains(h.statusLine(), want) {
		t.Errorf("the message has no time (%s): %q", want, h.statusLine())
	}
	// moving to another screen is not an action
	h.typ('3')
	h.waitFor("History")
	if !strings.Contains(h.screen(), "connection to Main PBS failed") {
		t.Errorf("a screen change cleared the message")
	}
	// the arrows are not an action either
	h.key(tcell.KeyDown, 0)
	if !strings.Contains(h.screen(), "connection to Main PBS failed") {
		t.Errorf("an arrow key cleared the message")
	}
	// Enter is
	h.key(tcell.KeyEnter, 0)
	if strings.Contains(h.screen(), "connection to Main PBS failed") {
		t.Errorf("the message stayed after Enter:\n%s", h.screen())
	}
}

func TestEscAClickOrANewerMessageReplacesAStandingResult(t *testing.T) {
	h := newHarness(t, 100, 30)
	h.waitFor("service running")
	h.onUI(func() { h.a.setFlash("saved Photos") })
	h.waitFor("saved Photos")
	h.key(tcell.KeyEscape, 0)
	if strings.Contains(h.screen(), "saved Photos") {
		t.Errorf("Esc did not clear the message")
	}
	h.onUI(func() { h.a.setFlash("saved Documents") })
	h.waitFor("saved Documents")
	h.sim.InjectMouse(50, 10, tcell.Button1, tcell.ModNone)
	h.sim.InjectMouse(50, 10, tcell.ButtonNone, tcell.ModNone)
	time.Sleep(300 * time.Millisecond)
	if strings.Contains(h.screen(), "saved Documents") {
		t.Errorf("a click did not clear the message")
	}
	h.onUI(func() { h.a.setFlash("first") })
	h.onUI(func() { h.a.setFlash("second") })
	h.waitFor("second")
	if strings.Contains(h.screen(), "first") {
		t.Errorf("an older message is still there")
	}
}

func TestAProgressNoteStillClearsItself(t *testing.T) {
	h := newHarness(t, 100, 30)
	h.waitFor("service running")
	h.onUI(func() { h.a.setFlash("testing the connection to Main PBS ...") })
	h.waitFor("testing the connection")
	h.age(10 * time.Second)
	if strings.Contains(h.screen(), "testing the connection") {
		t.Errorf("a note about something under way stayed on screen:\n%s", h.screen())
	}
}

// #143: a set that did not succeed shows why, and when it last did.
func TestFailedSetShowsWhyAndWhenItLastSucceeded(t *testing.T) {
	h := newHarness(t, 130, 40)
	h.waitFor("service running")
	h.typ('2')
	h.waitFor("Details")
	h.key(tcell.KeyDown, 0)
	h.key(tcell.KeyDown, 0)
	h.waitFor("Photos")
	scr := h.screen()
	for _, want := range []string{"Why", "2 files could not be read", "Last OK"} {
		if !strings.Contains(scr, want) {
			t.Errorf("the details of a failed set lack %q:\n%s", want, scr)
		}
	}
	// a set whose last run succeeded shows neither
	h.key(tcell.KeyUp, 0)
	h.key(tcell.KeyUp, 0)
	h.key(tcell.KeyUp, 0)
	time.Sleep(200 * time.Millisecond)
	scr = h.screen()
	if strings.Contains(scr, "Why") {
		t.Errorf("a set that succeeded shows a failure reason:\n%s", scr)
	}
}
