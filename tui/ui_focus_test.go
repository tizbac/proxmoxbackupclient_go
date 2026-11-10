package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func TestFocusTakesText(t *testing.T) {
	for _, c := range []struct {
		p    tview.Primitive
		want bool
	}{
		{tview.NewInputField(), true},
		{tview.NewTextArea(), true},
		{tview.NewDropDown(), true},
		{tview.NewButton("Save"), false},
		{tview.NewCheckbox(), false},
		{tview.NewTextView(), false},
		{nil, false},
	} {
		if got := focusTakesText(c.p); got != c.want {
			t.Errorf("%T: got %v, want %v", c.p, got, c.want)
		}
	}
}

// stopped waits a moment for the UI to stop (q, Ctrl+C) and says whether it did.
func (h *harness) stopped() bool {
	select {
	case <-h.done:
		return true
	case <-time.After(1500 * time.Millisecond):
		return false
	}
}

// onUI runs f on the UI goroutine, the one that handles keys, and waits for it.
func (h *harness) onUI(f func()) {
	h.t.Helper()
	ch := make(chan struct{})
	h.a.app.QueueUpdate(func() {
		f()
		close(ch)
	})
	select {
	case <-ch:
	case <-time.After(5 * time.Second):
		h.t.Fatal("the window did not answer")
	}
}

// q quits from every screen the moment it opens, except Preferences, which
// opens on a text field (q is typed there; Tab to a button, or Ctrl+C).
func TestQQuitsFromEveryScreen(t *testing.T) {
	for i := 0; i < 8; i++ {
		i := i
		t.Run(fmt.Sprintf("F%d", i+1), func(t *testing.T) {
			h := newHarness(t, 130, 40)
			h.sim.InjectKey(tcell.KeyF1+tcell.Key(i), 0, tcell.ModNone)
			time.Sleep(300 * time.Millisecond)
			h.typ('q')
			if !h.stopped() {
				t.Errorf("q did not quit on screen %d:\n%s", i+1, h.screen())
			}
		})
	}
}

func TestQIsTypedInAPreferencesField(t *testing.T) {
	h := newHarness(t, 130, 40)
	h.sim.InjectKey(tcell.KeyF9, 0, tcell.ModNone)
	h.waitFor("Upload limit")
	h.typ('q')
	select {
	case <-h.done:
		t.Error("q quit while a Preferences text field had the focus")
	case <-time.After(600 * time.Millisecond):
	}
}

// focusSweep presses Tab around a screen and checks at every stop that the
// keyboard belongs to the screen exactly when a field that takes text has the
// focus: on a button or a tick box q quits and the digits change screen, in an
// input field they are typed.
func focusSweep(t *testing.T, h *harness, where string) (buttons, fields int) {
	t.Helper()
	seen := map[string]bool{}
	for i := 0; i < 40; i++ {
		var name string
		var typing, takes bool
		h.onUI(func() {
			f := h.a.app.GetFocus()
			name = fmt.Sprintf("%T", f)
			takes = focusTakesText(f)
			typing = h.a.typingNow()
		})
		if typing != takes {
			t.Errorf("%s, stop %d (%s): typingNow %v but the field takes text: %v", where, i, name, typing, takes)
		}
		switch name {
		case "*tview.Button", "*tview.Checkbox":
			buttons++
		case "*tview.InputField", "*tview.TextArea", "*tview.DropDown":
			fields++
		}
		seen[name] = true
		h.key(tcell.KeyTab, 0)
	}
	return
}

// Every Tab stop of every Preferences page.
func TestEveryPreferencesStopKeepsTheRightKeys(t *testing.T) {
	h := newHarness(t, 130, 40)
	h.sim.InjectKey(tcell.KeyF9, 0, tcell.ModNone)
	h.waitFor("Upload limit")
	total := 0
	for page := 0; page < 4; page++ {
		b, f := focusSweep(t, h, fmt.Sprintf("Preferences page %d", page+1))
		total += b
		if b == 0 {
			t.Errorf("Preferences page %d: no button or tick box was reached", page+1)
		}
		_ = f
		h.key(tcell.KeyPgDn, 0)
		time.Sleep(200 * time.Millisecond)
	}
	if total == 0 {
		t.Error("the sweep reached nothing")
	}
}

// Every Tab stop of the Restore screen's Destination step; on its buttons q
// quits.
func TestRestoreDestinationStopsKeepTheRightKeys(t *testing.T) {
	h := newHarness(t, 130, 40)
	h.typ('6')
	h.waitFor("Source")
	h.key(tcell.KeyEnter, 0)
	h.waitFor("Snapshot")
	h.key(tcell.KeyEnter, 0)
	h.waitFor("Files")
	h.key(tcell.KeyPgDn, 0)
	h.waitFor("Restore into")
	if b, _ := focusSweep(t, h, "Restore destination"); b == 0 {
		t.Error("no button or tick box was reached")
	}
}

// q really quits on a button: Preferences and the Restore destination step.
func TestQQuitsFromAButton(t *testing.T) {
	t.Run("Preferences", func(t *testing.T) {
		h := newHarness(t, 130, 40)
		h.sim.InjectKey(tcell.KeyF9, 0, tcell.ModNone)
		h.waitFor("Upload limit")
		for i := 0; i < 10; i++ {
			h.key(tcell.KeyTab, 0)
		}
		var onButton bool
		h.onUI(func() { _, onButton = h.a.app.GetFocus().(*tview.Button) })
		if !onButton {
			t.Fatal("the test did not reach a button")
		}
		h.typ('q')
		if !h.stopped() {
			t.Error("q did not quit with a Preferences button focused")
		}
	})
	t.Run("Restore destination", func(t *testing.T) {
		h := newHarness(t, 130, 40)
		h.typ('6')
		h.waitFor("Source")
		h.key(tcell.KeyEnter, 0)
		h.waitFor("Snapshot")
		h.key(tcell.KeyEnter, 0)
		h.waitFor("Files")
		h.key(tcell.KeyPgDn, 0)
		h.waitFor("Restore into")
		h.key(tcell.KeyTab, 0)
		h.key(tcell.KeyTab, 0)
		h.key(tcell.KeyTab, 0)
		h.typ('q')
		if !h.stopped() {
			t.Errorf("q did not quit with a Restore button focused:\n%s", h.screen())
		}
	})
}

// In a field a digit is typed; on the tick box or a button 2 opens Backup Sets.
func TestDigitsTypeInFieldsAndSwitchScreensOnButtons(t *testing.T) {
	h := newHarness(t, 130, 40)
	h.sim.InjectKey(tcell.KeyF9, 0, tcell.ModNone)
	h.waitFor("Upload limit")
	h.typeText("7")
	h.waitFor("Upload limit")
	field := ""
	for _, l := range strings.Split(h.screen(), "\n") {
		if strings.Contains(l, "Upload limit") {
			field = l
		}
	}
	if !strings.Contains(field, "7") {
		t.Errorf("the digit was not typed into the Upload limit field: %q", field)
	}
	for i := 0; i < 10; i++ {
		h.key(tcell.KeyTab, 0)
	}
	h.typ('2')
	h.waitFor("Backup Sets")
	h.waitFor("Details")
	if strings.Contains(h.screen(), "Upload limit") {
		t.Errorf("still on Preferences after 2 on a button:\n%s", h.screen())
	}
}
