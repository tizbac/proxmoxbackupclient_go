package tui

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// openLog shows the log of the selected history entry's run.
func (a *App) openLog() {
	row, _ := a.histTable.GetSelection()
	a.mu.Lock()
	if row < 1 || row > len(a.hist) {
		a.mu.Unlock()
		return
	}
	h := a.hist[row-1]
	a.mu.Unlock()
	a.viewLog(fmt.Sprintf("Log: %s, %s", h.Name, when(h.At)), func(filter string, hide bool) ([]string, error) {
		lines, err := a.be.RunLog(h.BackupID, h.At, hide)
		return filterLines(lines, filter), err
	})
}

func filterLines(lines []string, filter string) []string {
	if filter == "" {
		return lines
	}
	var out []string
	for _, l := range lines {
		if strings.Contains(strings.ToLower(l), strings.ToLower(filter)) {
			out = append(out, l)
		}
	}
	return out
}

// viewLog is the log viewer: Esc closes it, c leaves out the chunk lines, /
// filters the lines, and Up, Down, PgUp, PgDn, Home and End scroll. fetch
// reads the lines again for a filter and the chunk setting.
func (a *App) viewLog(title string, fetch func(filter string, hideChunks bool) ([]string, error)) {
	view := tview.NewTextView().SetDynamicColors(true).SetScrollable(true).SetWrap(false)
	view.SetBorder(true).SetTitle(" " + tview.Escape(title) + " ").SetTitleAlign(tview.AlignLeft)
	filter := tview.NewInputField().SetLabel(" Filter: ").SetFieldWidth(40)
	filter.SetFieldBackgroundColor(colSel).SetLabelColor(colKey)
	foot := tview.NewTextView().SetDynamicColors(true)
	foot.SetBackgroundColor(colPanel)
	page := tview.NewFlex().SetDirection(tview.FlexRow).AddItem(view, 0, 1, true).AddItem(filter, 0, 0, false).AddItem(foot, 1, 0, false)

	hide := false
	current := ""
	setFoot := func() {
		state := ""
		if current != "" {
			state = tag(colWarn) + "  filter: " + tview.Escape(current)
		}
		if hide {
			state += tag(colWarn) + "  chunk lines hidden"
		}
		foot.SetText(" " + tag(colKey) + "[::b]Esc[-:-:-]" + tag(colDim) + " back  " + tag(colKey) + "[::b]/[-:-:-]" + tag(colDim) + " filter  " +
			tag(colKey) + "[::b]c[-:-:-]" + tag(colDim) + " hide or show chunk lines  " + tag(colKey) + "[::b]Up, Down, PgUp, PgDn, End[-:-:-]" + tag(colDim) + " scroll" + state)
	}
	load := func() {
		view.SetText(tag(colDim) + "reading the log...")
		setFoot()
		f, h := current, hide
		go func() {
			lines, err := fetch(f, h)
			a.app.QueueUpdateDraw(func() {
				if err != nil {
					view.SetText(tag(colErr) + "No log: " + tview.Escape(describeErr(err)))
					return
				}
				var b strings.Builder
				for _, l := range lines {
					b.WriteString(colourLogLine(l))
					b.WriteByte('\n')
				}
				if len(lines) == 0 {
					b.WriteString(tag(colDim) + "No lines.")
				}
				view.SetText(b.String())
				view.ScrollToEnd()
			})
		}()
	}
	closeView := func() {
		a.pages.RemovePage("log")
		a.modal = false
		a.switchTab(a.tab)
		a.renderChrome()
	}
	view.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		switch {
		case ev.Key() == tcell.KeyEscape, ev.Rune() == 'q':
			closeView()
			return nil
		case ev.Rune() == 'c':
			hide = !hide
			load()
			return nil
		case ev.Rune() == '/':
			filter.SetText(current)
			page.ResizeItem(filter, 1, 0)
			a.app.SetFocus(filter)
			return nil
		}
		return ev
	})
	filter.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			current = strings.TrimSpace(filter.GetText())
			load()
		}
		page.ResizeItem(filter, 0, 0)
		a.app.SetFocus(view)
	})
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
