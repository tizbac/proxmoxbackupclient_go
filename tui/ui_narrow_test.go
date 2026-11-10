package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func screenLines(h *harness) []string { return strings.Split(h.screen(), "\n") }

// In an 80 column text mode window the tab bar wrapped: Logs and Preferences fell
// off the screen and a click on the wrapped part landed on something else.
func TestTabBarFitsEveryWidthWithAllNineTabs(t *testing.T) {
	for _, w := range []int{80, 90, 100, 110, 125, 140} {
		h := newHarness(t, w, 25)
		h.waitFor("service running")
		bar := screenLines(h)[1]
		for _, n := range []string{"1 ", "2 ", "3 ", "4 ", "5 ", "6 ", "7 ", "8 ", "9 "} {
			if !strings.Contains(bar, n) {
				t.Errorf("width %d: the tab bar has no %q:\n%q", w, n, bar)
			}
		}
		if !strings.Contains(bar, "Prefs") && !strings.Contains(bar, "Preferences") {
			t.Errorf("width %d: the last tab is missing: %q", w, bar)
		}
		if got := tabBarWidth(tabNamesFor(w)); got > w-1 {
			t.Errorf("width %d: the tab bar needs %d columns", w, got)
		}
	}
}

// Key hints are left out whole when they do not fit, never cut off in the middle of a word.
func TestFooterHintsAreWholeAtEveryWidth(t *testing.T) {
	hints := [][2]string{{"Ctrl+Q", "quit"}, {"Ctrl+G", "help"}, {"→ ←", "screen"}, {"r", "run now"}, {"p", "pause/resume"}, {"s", "stop"}}
	var whole []string
	for _, h := range hints {
		whole = append(whole, h[0]+" "+h[1])
	}
	for w := 20; w <= 140; w++ {
		plain := strings.TrimRight(tview.Escape(stripTags(footerText(hints, w))), " ")
		if tview.TaggedStringWidth(footerText(hints, w)) > w-1 {
			t.Fatalf("width %d: the footer is wider than the window", w)
		}
		// what is shown is a run of whole hints from the start
		joined := ""
		for _, s := range whole {
			if strings.HasPrefix(strings.TrimLeft(plain, " "), strings.TrimSpace(joined+"  "+s)) || strings.TrimSpace(plain) == strings.TrimSpace(joined+"  "+s) {
				joined += "  " + s
			}
		}
		if strings.TrimSpace(plain) != strings.TrimSpace(joined) {
			t.Fatalf("width %d: the footer %q is not made of whole hints", w, plain)
		}
	}
}

func stripTags(s string) string {
	var b strings.Builder
	in := false
	for _, r := range s {
		switch {
		case r == '[':
			in = true
		case r == ']' && in:
			in = false
		case !in:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func TestNoScreenCutsAKeyHintInTheMiddleAt80Columns(t *testing.T) {
	h := newHarness(t, 80, 25)
	h.waitFor("service running")
	// F1 to F9 switch screens everywhere, also out of a form where digits are typed into a field
	for i, fk := range []tcell.Key{tcell.KeyF1, tcell.KeyF2, tcell.KeyF3, tcell.KeyF4, tcell.KeyF5, tcell.KeyF6, tcell.KeyF7, tcell.KeyF8, tcell.KeyF9} {
		h.key(fk, 0)
		lines := screenLines(h)
		foot := strings.TrimRight(lines[23], " ")
		// every hint ends with its description; none may end on a lone key such as "e"
		if strings.HasSuffix(foot, " e") || strings.HasSuffix(foot, " u") || strings.HasSuffix(foot, " PgDn") {
			t.Errorf("tab %d: the footer is cut off: %q", i+1, foot)
		}
		if i == 5 {
			if bar := lines[2]; !strings.Contains(bar, "Esc back") {
				t.Errorf("the Restore steps line lost its hint or cut it: %q", bar)
			}
		}
		if i == 8 {
			if bar := strings.TrimRight(lines[2], " "); strings.HasSuffix(bar, ",") || strings.HasSuffix(bar, "Tab next") {
				t.Errorf("the Preferences steps line is cut off: %q", bar)
			}
		}
	}
}

func TestDashboardRecentDoesNotWrapOutOfItsBox(t *testing.T) {
	h := newHarness(t, 80, 25)
	h.waitFor("Recent")
	lines := screenLines(h)
	in := false
	for _, l := range lines {
		if strings.Contains(l, "Recent ") && strings.HasPrefix(l, "┌") {
			in = true
			continue
		}
		if in && strings.HasPrefix(l, "└") {
			break
		}
		if in && !strings.HasPrefix(l, "│") {
			t.Errorf("a line of the Recent box left its column: %q", l)
		}
		if in && strings.HasPrefix(l, "│chunks") {
			t.Errorf("the Recent box wrapped a message: %q", l)
		}
	}
}

// Clicking a name in the tab bar opens that screen.
func TestClickingATabOpensItAt80Columns(t *testing.T) {
	h := newHarness(t, 80, 25)
	h.waitFor("service running")
	click := func(tab int) {
		h.a.mu.Lock()
		sp := h.a.tabSpans[tab]
		h.a.mu.Unlock()
		x := sp[0] + 2
		h.sim.InjectMouse(x, 1, tcell.Button1, tcell.ModNone)
		h.sim.InjectMouse(x, 1, tcell.ButtonNone, tcell.ModNone)
		time.Sleep(700 * time.Millisecond) // clicks closer together than this count as a double click
	}
	click(4)
	h.waitFor("PBS servers")
	click(8)
	h.waitFor("Backup options")
	click(2)
	h.waitFor("When ")
	click(0)
	h.waitFor("Running now")
}
