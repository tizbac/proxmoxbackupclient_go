package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
)

// Ctrl+Q quits from every screen, a text field included.
func TestCtrlQQuitsFromEveryScreen(t *testing.T) {
	for i := 0; i < 9; i++ {
		i := i
		t.Run(fmt.Sprintf("F%d", i+1), func(t *testing.T) {
			h := newHarness(t, 130, 40)
			h.sim.InjectKey(tcell.KeyF1+tcell.Key(i), 0, tcell.ModNone)
			time.Sleep(300 * time.Millisecond)
			h.key(tcell.KeyCtrlQ, 0)
			if !h.stopped() {
				t.Errorf("Ctrl+Q did not quit on screen %d:\n%s", i+1, h.screen())
			}
		})
	}
}

// Ctrl+X quits too, as in nano, except in a text area where it is cut.
func TestCtrlXQuits(t *testing.T) {
	h := newHarness(t, 130, 40)
	h.sim.InjectKey(tcell.KeyF9, 0, tcell.ModNone)
	h.waitFor("Upload limit")
	h.key(tcell.KeyCtrlX, 0)
	if !h.stopped() {
		t.Error("Ctrl+X did not quit from a Preferences field")
	}
}

func TestCtrlXInATextAreaDoesNotQuit(t *testing.T) {
	h := newHarness(t, 130, 40)
	h.typ('2')
	h.waitFor("Backup Sets")
	h.typ('n')
	h.waitFor("What should the new")
	h.key(tcell.KeyEnter, 0) // Folders
	h.waitFor("New Backup Set")
	h.key(tcell.KeyTab, 0) // Backup ID
	h.key(tcell.KeyTab, 0) // Folders: a text area
	var inArea bool
	h.onUI(func() { _, inArea = h.a.app.GetFocus().(interface{ GetText() string }) })
	h.key(tcell.KeyCtrlX, 0)
	select {
	case <-h.done:
		t.Errorf("Ctrl+X quit with the focus in a text area (focus took text: %v)", inArea)
	case <-time.After(700 * time.Millisecond):
	}
}

// Under an open form Ctrl+Q asks first: No keeps the form, Yes quits.
func TestCtrlQUnderAFormAsksFirst(t *testing.T) {
	open := func(h *harness) {
		h.typ('2')
		h.waitFor("Backup Sets")
		h.typ('n')
		h.waitFor("What should the new")
		h.key(tcell.KeyEnter, 0)
		h.waitFor("New Backup Set")
		h.key(tcell.KeyCtrlQ, 0)
		h.waitFor("Quit the window?")
	}
	t.Run("No", func(t *testing.T) {
		h := newHarness(t, 130, 40)
		open(h)
		h.key(tcell.KeyTab, 0) // No
		h.key(tcell.KeyEnter, 0)
		h.waitFor("New Backup Set")
		select {
		case <-h.done:
			t.Error("the window quit after No")
		case <-time.After(500 * time.Millisecond):
		}
	})
	t.Run("Yes", func(t *testing.T) {
		h := newHarness(t, 130, 40)
		open(h)
		h.key(tcell.KeyEnter, 0) // Yes
		if !h.stopped() {
			t.Errorf("the window did not quit after Yes:\n%s", h.screen())
		}
	})
}

// Ctrl+G opens the key list from anywhere, in a field too.
func TestCtrlGOpensHelpFromAField(t *testing.T) {
	h := newHarness(t, 130, 40)
	h.sim.InjectKey(tcell.KeyF9, 0, tcell.ModNone)
	h.waitFor("Upload limit")
	h.key(tcell.KeyCtrlG, 0)
	h.waitFor("quit from anywhere")
	h.key(tcell.KeyEscape, 0)
	h.waitFor("Upload limit")
}

// Left and Right move between the screens from a list; at the ends they wrap.
func TestArrowsMoveBetweenScreens(t *testing.T) {
	h := newHarness(t, 130, 40)
	h.waitFor("Running now") // the Dashboard
	h.key(tcell.KeyRight, 0)
	h.waitFor("Details") // Backup Sets
	h.key(tcell.KeyLeft, 0)
	h.waitFor("Running now")
	h.key(tcell.KeyLeft, 0) // wraps to the last screen
	h.waitFor("Upload limit")
	// in Preferences the arrows belong to the fields: the screen does not change
	h.key(tcell.KeyRight, 0)
	time.Sleep(300 * time.Millisecond)
	if !strings.Contains(h.screen(), "Upload limit") {
		t.Errorf("Right left Preferences from a field:\n%s", h.screen())
	}
}

// On the Restore lists Right goes in and Left goes back.
func TestRestoreArrowsGoInAndBack(t *testing.T) {
	h := newHarness(t, 130, 40)
	h.typ('6')
	h.waitFor("Backups on the server")
	h.key(tcell.KeyRight, 0)
	h.waitFor("Snapshots")
	h.key(tcell.KeyLeft, 0)
	h.waitFor("Backups on the server")
	// and Left on the first list does not leave Restore
	h.key(tcell.KeyLeft, 0)
	time.Sleep(300 * time.Millisecond)
	if !strings.Contains(h.screen(), "Backups on the server") {
		t.Errorf("Left on the first Restore list left the screen:\n%s", h.screen())
	}
}

// Alt+Right and Alt+Left move between the pages of Preferences and of a form.
func TestAltArrowsMoveBetweenPages(t *testing.T) {
	h := newHarness(t, 130, 40)
	h.sim.InjectKey(tcell.KeyF9, 0, tcell.ModNone)
	h.waitFor("Upload limit")
	h.sim.InjectKey(tcell.KeyRight, 0, tcell.ModAlt)
	time.Sleep(200 * time.Millisecond)
	h.sim.InjectKey(tcell.KeyRight, 0, tcell.ModAlt)
	h.waitFor("Saved logins")
	h.sim.InjectKey(tcell.KeyLeft, 0, tcell.ModAlt)
	time.Sleep(300 * time.Millisecond)
	if strings.Contains(h.screen(), "Saved logins") {
		t.Errorf("Alt+Left stayed on the Network shares page:\n%s", h.screen())
	}
	h.sim.InjectKey(tcell.KeyLeft, 0, tcell.ModAlt)
	h.waitFor("Upload limit")

	// the Backup Set editor
	h.sim.InjectKey(tcell.KeyF2, 0, tcell.ModNone)
	h.waitFor("Backup Sets")
	h.typ('n')
	h.waitFor("What should the new")
	h.key(tcell.KeyEnter, 0)
	h.waitFor("New Backup Set")
	h.sim.InjectKey(tcell.KeyRight, 0, tcell.ModAlt)
	h.waitFor("Repeat every")
	h.sim.InjectKey(tcell.KeyLeft, 0, tcell.ModAlt)
	h.waitFor("Backup ID")
}
