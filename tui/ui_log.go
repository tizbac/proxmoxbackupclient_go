package tui

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// openLog shows the log of the selected history entry's run: Esc closes it,
// c leaves out the chunk lines, Up, Down, PgUp, PgDn, Home and End scroll.
func (a *App) openLog() {
	row, _ := a.histTable.GetSelection()
	a.mu.Lock()
	if row < 1 || row > len(a.hist) {
		a.mu.Unlock()
		return
	}
	h := a.hist[row-1]
	a.mu.Unlock()

	view := tview.NewTextView().SetDynamicColors(true).SetScrollable(true).SetWrap(false)
	view.SetBorder(true).SetTitle(fmt.Sprintf(" Log: %s, %s ", h.Name, when(h.At))).SetTitleAlign(tview.AlignLeft)
	hide := false
	load := func() {
		view.SetText(tag(colDim) + "reading the log...")
		go func() {
			lines, err := a.be.RunLog(h.BackupID, h.At, hide)
			a.app.QueueUpdateDraw(func() {
				if err != nil {
					view.SetText(tag(colErr) + "No log for this run: " + tview.Escape(describeErr(err)))
					return
				}
				var b strings.Builder
				for _, l := range lines {
					b.WriteString(colourLogLine(l))
					b.WriteByte('\n')
				}
				if len(lines) == 0 {
					b.WriteString(tag(colDim) + "The log is empty.")
				}
				view.SetText(b.String())
				view.ScrollToEnd()
			})
		}()
	}
	view.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		switch {
		case ev.Key() == tcell.KeyEscape, ev.Rune() == 'q':
			a.pages.RemovePage("log")
			a.modal = false
			a.switchTab(a.tab)
			a.renderChrome()
			return nil
		case ev.Rune() == 'c':
			hide = !hide
			load()
			return nil
		}
		return ev
	})
	foot := tview.NewTextView().SetDynamicColors(true)
	foot.SetBackgroundColor(colPanel)
	foot.SetText(" " + tag(colKey) + "[::b]Esc[-:-:-]" + tag(colDim) + " back  " + tag(colKey) + "[::b]c[-:-:-]" + tag(colDim) + " hide or show chunk lines  " +
		tag(colKey) + "[::b]Up, Down, PgUp, PgDn, End[-:-:-]" + tag(colDim) + " scroll")
	page := tview.NewFlex().SetDirection(tview.FlexRow).AddItem(view, 0, 1, true).AddItem(foot, 1, 0, false)
	a.modal = true
	a.pages.AddPage("log", page, true, false)
	a.pages.SwitchToPage("log")
	a.app.SetFocus(view)
	load()
}

// colourLogLine colours a log line by what it says.
func colourLogLine(l string) string {
	esc := tview.Escape(l)
	low := strings.ToLower(l)
	switch {
	case strings.Contains(low, "error"), strings.Contains(low, "failed"):
		return tag(colErr) + esc + "[-]"
	case strings.Contains(low, "warning"), strings.Contains(low, "retry"):
		return tag(colWarn) + esc + "[-]"
	case strings.Contains(low, "completed"), strings.Contains(low, "snapshot created"):
		return tag(colOK) + esc + "[-]"
	case strings.Contains(low, "chunk"):
		return tag(colDim) + esc + "[-]"
	}
	return esc
}
