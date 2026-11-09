package tui

import (
	"fmt"
	"strings"
)

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
			fmt.Fprintf(&b, "[black:#%06x] %s [-:-]", colHead.Hex(), n)
		case markDone && i < current:
			b.WriteString(tag(colOK) + " " + n + " [-:-]")
		default:
			b.WriteString(tag(colDim) + " " + n + " [-:-]")
		}
	}
	b.WriteString(" ")
	return b.String()
}
