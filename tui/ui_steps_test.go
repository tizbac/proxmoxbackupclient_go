package tui

import (
	"regexp"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

var styleTags = regexp.MustCompile(`\[[^\]]*\]`)

func plainSteps(s string) string { return strings.TrimSpace(styleTags.ReplaceAllString(s, "")) }

func TestStepBarIsNamesWithArrowsAndNoNumbers(t *testing.T) {
	got := plainSteps(stepBar([]string{"General", "Schedule", "Options"}, 1, false))
	if got != "General > Schedule > Options" {
		t.Fatalf("unexpected bar %q", got)
	}
	for _, n := range []string{"1", "2", "3"} {
		if strings.Contains(got, n) {
			t.Errorf("the bar has a number %q: %q", n, got)
		}
	}
	if strings.Count(got, ">") != 2 {
		t.Errorf("want two > between three steps: %q", got)
	}
	// the current step is highlighted, a passed step is green only when asked
	hl := stepBar([]string{"A", "B", "C"}, 1, true)
	if !strings.Contains(hl, "[black:#") || !strings.Contains(hl, tag(colOK)+" A ") {
		t.Errorf("highlight or done mark missing: %q", hl)
	}
	if strings.Contains(stepBar([]string{"A", "B", "C"}, 1, false), tag(colOK)) {
		t.Error("a form's passed step must not be marked done")
	}
}

// The step rows of the screens read as names with > between them, and the
// Restore row says how to move.
func TestStepRowsOnTheScreens(t *testing.T) {
	h := newHarness(t, 130, 40)
	h.typ('6')
	h.waitFor("Source")
	h.waitFor("Snapshot > Files")
	h.waitFor("Enter next step, Esc back")
	if got := h.screen(); strings.Contains(got, "1 Source") || strings.Contains(got, "2 Snapshot") {
		t.Errorf("the Restore steps are still numbered:\n%s", got)
	}

	h.sim.InjectKey(tcell.KeyF9, 0, tcell.ModNone)
	h.waitFor("Backup options > E-mail > Network shares > Settings file")

	h.sim.InjectKey(tcell.KeyF2, 0, tcell.ModNone) // Preferences has a field in focus: digits type into it
	h.waitFor("Backup Sets")
	h.typ('n')
	h.waitFor("What should the new")
	h.key(tcell.KeyEnter, 0)
	h.waitFor("General > Schedule > Options")
	h.key(tcell.KeyEscape, 0)

	h.typ('5')
	h.waitFor("Main PBS")
	h.key(tcell.KeyEnter, 0)
	h.waitFor("Server > Login > Encryption")
}
