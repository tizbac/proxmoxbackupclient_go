package tui

import (
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
)

func TestColorDepthFromTheEnvironment(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the Windows console always takes RGB")
	}
	for _, c := range []struct {
		term, colorterm, override string
		want                      int
	}{
		{"xterm-256color", "", "", 256},
		{"xterm-256color", "truecolor", "", 1 << 24},
		{"xterm-direct", "", "", 1 << 24},
		{"xterm-kitty", "", "", 256},
		{"linux", "", "", 8}, // the Linux console: 8 colours, bold gives the bright ones
		{"xterm", "", "", 8},
		{"screen", "", "", 8},
		{"vt100", "", "", 8},
		{"xterm-16color", "", "", 16},
		{"", "", "", 256},
		{"xterm-256color", "", "basic", 8},
		{"xterm-256color", "", "16", 16},
		{"linux", "", "truecolor", 1 << 24},
	} {
		t.Setenv("TERM", c.term)
		t.Setenv("COLORTERM", c.colorterm)
		t.Setenv("SALVANZA_TUI_COLORS", c.override)
		if got := colorDepth(); got != c.want {
			t.Errorf("TERM=%q COLORTERM=%q override=%q: %d colours, want %d", c.term, c.colorterm, c.override, got, c.want)
		}
	}
}

func TestPalettesSwitchAndRestore(t *testing.T) {
	t.Cleanup(func() { usePalette(1 << 24) })
	usePalette(1 << 24)
	if basicColors || tag(colHead) != "[#6cc3ff]" {
		t.Errorf("the RGB palette changed: basic %v, head %s", basicColors, tag(colHead))
	}
	usePalette(8)
	if !basicColors || tag(colHead) != "[teal]" || tag(colAccent) != "[navy]" {
		t.Errorf("the 8 colour palette: basic %v, head %s, accent %s", basicColors, tag(colHead), tag(colAccent))
	}
	usePalette(16)
	if tag(colHead) != "[aqua]" || spec(colWhite) != "white" {
		t.Errorf("the 16 colour palette: head %s, white %s", tag(colHead), spec(colWhite))
	}
	usePalette(256)
	if basicColors || tag(colAccent) != "[#0f62a8]" {
		t.Errorf("back to RGB: basic %v, accent %s", basicColors, tag(colAccent))
	}
}

// colourIndex says how a cell colour is drawn: the palette index, or -1 for the
// terminal's own colour, or -2 for an RGB value no 8 or 16 colour terminal has.
func colourIndex(c tcell.Color) int {
	switch {
	case c == tcell.ColorDefault:
		return -1
	case c&tcell.ColorIsRGB != 0:
		return -2
	}
	return int(c - tcell.ColorValid)
}

// ansiOnly checks every cell of the screen: nothing in RGB, and no palette
// index at or above limit.
func ansiOnly(t *testing.T, h *harness, limit int, where string) {
	t.Helper()
	cells, w, hgt := h.contents()
	bad := map[string]int{}
	var where1 []string // the first few offending cells, to find them
	for i, c := range cells {
		fg, bg, _ := c.Style.Decompose()
		for k, col := range []tcell.Color{fg, bg} {
			if idx := colourIndex(col); idx == -2 || idx >= limit {
				bad[fmt.Sprintf("colour %v (index %d)", col, idx)]++
				if len(where1) < 6 {
					r := " "
					if len(c.Runes) > 0 {
						r = string(c.Runes[0])
					}
					where1 = append(where1, fmt.Sprintf("%q at %d,%d as %s", r, i%w, i/w, []string{"text", "background"}[k]))
				}
			}
		}
	}
	if len(bad) > 0 {
		t.Errorf("%s (%dx%d): colours outside the %d colour palette: %v, e.g. %v", where, w, hgt, limit, bad, where1)
	}
}

// With 8 or 16 colours nothing in RGB may reach the screen, on any screen or in
// any form: the terminal would guess a nearest colour and could pick a teal.
func TestBasicPalettesUseOnlyAnsiColours(t *testing.T) {
	for _, limit := range []int{8, 16} {
		limit := limit
		t.Run(fmt.Sprintf("%d colours", limit), func(t *testing.T) {
			t.Cleanup(func() { usePalette(1 << 24) })
			usePalette(limit)
			h := newHarness(t, 130, 40)
			for i := 0; i < 9; i++ {
				h.sim.InjectKey(tcell.KeyF1+tcell.Key(i), 0, tcell.ModNone)
				time.Sleep(350 * time.Millisecond)
				ansiOnly(t, h, limit, fmt.Sprintf("screen %d", i+1))
			}
			// forms and windows over the screens
			h.sim.InjectKey(tcell.KeyF2, 0, tcell.ModNone)
			h.waitFor("Backup Sets")
			h.typ('n')
			h.waitFor("What should the new")
			ansiOnly(t, h, limit, "the new set question")
			h.key(tcell.KeyTab, 0)
			h.key(tcell.KeyEnter, 0) // disk image
			h.waitFor("no disk chosen")
			ansiOnly(t, h, limit, "the disk image set form")
			h.key(tcell.KeyCtrlD, 0)
			h.waitFor("Samsung")
			h.typ(' ')
			h.key(tcell.KeyDown, 0)
			h.typ(' ')
			time.Sleep(300 * time.Millisecond)
			ansiOnly(t, h, limit, "the disk picker")
			h.key(tcell.KeyEnter, 0)
			h.key(tcell.KeyCtrlQ, 0)
			h.waitFor("Quit the window?")
			ansiOnly(t, h, limit, "a question over a form")
			h.key(tcell.KeyTab, 0)
			h.key(tcell.KeyEnter, 0) // No
			h.key(tcell.KeyEscape, 0)
			h.waitFor("Backup Sets")
			h.sim.InjectKey(tcell.KeyF5, 0, tcell.ModNone)
			h.waitFor("Main PBS")
			h.key(tcell.KeyEnter, 0)
			h.waitFor("Edit server")
			ansiOnly(t, h, limit, "the server form")
			h.key(tcell.KeyEscape, 0)
			h.sim.InjectKey(tcell.KeyF6, 0, tcell.ModNone)
			h.waitFor("Backups on the server")
			h.key(tcell.KeyEnter, 0)
			h.waitFor("Snapshot")
			ansiOnly(t, h, limit, "Restore, snapshots")
			h.key(tcell.KeyEnter, 0)
			h.waitFor("Files")
			ansiOnly(t, h, limit, "Restore, files")
			h.key(tcell.KeyPgDn, 0)
			h.waitFor("Restore into")
			ansiOnly(t, h, limit, "Restore, destination")
			h.typ('?')
		})
	}
}

// findText returns the style of the first cell of text on the screen.
func findText(h *harness, text string) (tcell.Style, bool) {
	cells, w, hgt := h.contents()
	runes := []rune(text)
	for y := 0; y < hgt; y++ {
		for x := 0; x+len(runes) <= w; x++ {
			ok := true
			for k, r := range runes {
				cr := cells[y*w+x+k].Runes
				if len(cr) == 0 || cr[0] != r {
					ok = false
					break
				}
			}
			if ok {
				return cells[y*w+x].Style, true
			}
		}
	}
	return tcell.StyleDefault, false
}

// In the Linux console "white" is a light grey unless it is bold: the text of a
// button and of a field is bold in the ANSI palette.
func TestBasicPaletteButtonsAndFieldsAreBold(t *testing.T) {
	t.Cleanup(func() { usePalette(1 << 24) })
	usePalette(8)
	h := newHarness(t, 130, 40)
	h.sim.InjectKey(tcell.KeyF2, 0, tcell.ModNone)
	h.waitFor("Backup Sets")
	h.typ('n')
	h.waitFor("What should the new")
	h.key(tcell.KeyEnter, 0)
	h.waitFor("New Backup Set")
	for _, label := range []string{"Next", "Save", "Cancel"} {
		st, ok := findText(h, label)
		if !ok {
			t.Fatalf("no %q button on the screen:\n%s", label, h.screen())
		}
		fg, bg, attrs := st.Decompose()
		if attrs&tcell.AttrBold == 0 {
			t.Errorf("the %q button text is not bold", label)
		}
		if colourIndex(bg) != 4 { // navy, the ANSI blue
			t.Errorf("the %q button is not on the ANSI blue: background %v", label, bg)
		}
		if colourIndex(fg) != 7 { // the white an 8 colour terminal has
			t.Errorf("the %q button text is not that white: %v", label, fg)
		}
	}
}
