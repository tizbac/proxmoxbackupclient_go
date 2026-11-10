package tui

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
)

// altStep says which way Alt+Left (-1) or Alt+Right (1) asks to move between the
// pages of a form, as PgUp and PgDn do; 0 for any other key.
func altStep(ev *tcell.EventKey) int {
	if ev.Modifiers()&tcell.ModAlt == 0 {
		return 0
	}
	switch ev.Key() {
	case tcell.KeyLeft:
		return -1
	case tcell.KeyRight:
		return 1
	}
	return 0
}

// stepBar draws the steps of a wizard or a form as their names with > between
// them, the current one highlighted. With markDone the steps already passed
// are green (a wizard that only goes forward, like Restore). The names are not
// numbered: a number invites a key press, and the steps move with PgDn and
// PgUp (or Enter and Esc in Restore).
func stepBar(names []string, current int, markDone bool) string {
	var b strings.Builder
	b.WriteString(" ")
	for i, n := range names {
		if i > 0 {
			b.WriteString(tag(colDim) + ">[-:-]")
		}
		switch {
		case i == current:
			fmt.Fprintf(&b, "[black:%s] %s [-:-]", spec(colHead), n)
		case markDone && i < current:
			b.WriteString(tag(colOK) + " " + n + " [-:-]")
		default:
			b.WriteString(tag(colDim) + " " + n + " [-:-]")
		}
	}
	b.WriteString(" ")
	return b.String()
}
