package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// RollbackRecord is an earlier roll back of a Backup Set that left a safety
// folder, as `salvanza-svc -list-rollbacks` prints it.
type RollbackRecord struct {
	Created, Status, Policy string
	Moved, Removed, Restored int
	Bytes                    int64
}

// rollbackRecords lists a set's earlier roll backs (newest first) through the
// service program.
func rollbackRecords(be Backend, setID string) ([]RollbackRecord, error) {
	var out []RollbackRecord
	err := be.RunSetTool([]string{"-list-rollbacks", setID}, func(l string) {
		f := strings.Split(l, "\t")
		if len(f) != 8 || f[0] != "ROLLBACK" {
			return
		}
		n := func(s string) int { v, _ := strconv.Atoi(s); return v }
		b, _ := strconv.ParseInt(f[7], 10, 64)
		out = append(out, RollbackRecord{Created: f[1], Status: f[2], Policy: f[3], Moved: n(f[4]), Removed: n(f[5]), Restored: n(f[6]), Bytes: b})
	})
	return out, err
}

func (r RollbackRecord) when() string {
	if t, err := time.Parse(time.RFC3339, r.Created); err == nil {
		return t.Local().Format("2006-01-02 15:04")
	}
	return r.Created
}

// rollbackList is the list of a set's earlier roll backs, with Undo (u) and
// deleting the safety copies (d).
func (a *App) rollbackList(s Set) {
	tbl := tview.NewTable().SetSelectable(true, false).SetFixed(1, 0)
	tbl.SetBorder(true).SetTitle(" Earlier roll backs of " + s.Name + " ").SetBackgroundColor(colPanel)
	foot := tview.NewTextView().SetDynamicColors(true)
	foot.SetText(" " + tag(colDim) + "u undo the selected roll back   d delete its safety copies (it can no longer be undone)   Esc back")
	var recs []RollbackRecord
	fill := func(list []RollbackRecord, err error) {
		recs = list
		tbl.Clear()
		for i, h := range []string{"Rolled back", "State", "Policy", "Put aside", "Removed", "Restored", "Kept"} {
			tbl.SetCell(0, i, tview.NewTableCell(h).SetTextColor(colHead).SetSelectable(false))
		}
		if err != nil {
			tbl.SetCell(1, 0, tview.NewTableCell(describeErr(err)).SetTextColor(colErr))
			return
		}
		if len(list) == 0 {
			tbl.SetCell(1, 0, tview.NewTableCell("no earlier roll backs with a safety folder").SetTextColor(colDim).SetSelectable(false))
			return
		}
		for i, r := range list {
			state := r.Status
			if state == "" {
				state = "kept"
			}
			for c, v := range []string{r.when(), state, r.Policy, strconv.Itoa(r.Moved), strconv.Itoa(r.Removed), strconv.Itoa(r.Restored), human(float64(r.Bytes))} {
				tbl.SetCell(i+1, c, tview.NewTableCell(v).SetTextColor(colText))
			}
		}
		tbl.Select(1, 0)
	}
	load := func() {
		go func() {
			list, err := rollbackRecords(a.be, s.ID)
			a.app.QueueUpdateDraw(func() { fill(list, err) })
		}()
	}
	closeList := func() { a.closeModal("rollbacks") }
	selected := func() (RollbackRecord, bool) {
		row, _ := tbl.GetSelection()
		if row < 1 || row > len(recs) {
			return RollbackRecord{}, false
		}
		return recs[row-1], true
	}
	// after: reload the list when the output window of the action is closed
	act := func(flag string, ask string) {
		r, ok := selected()
		if !ok {
			return
		}
		a.confirm(fmt.Sprintf(ask, r.when()), func() {
			a.closeModal("rollbacks")
			a.toolOutput(strings.TrimPrefix(flag, "-")+": "+s.Name, []string{flag, s.ID, "-created", r.Created}, nil)
		})
	}
	tbl.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		switch {
		case ev.Key() == tcell.KeyEscape:
			closeList()
			return nil
		case ev.Key() == tcell.KeyRune && ev.Rune() == 'u':
			act("-undo-rollback", "Undo the roll back of %s? The files it restored are deleted and the files it put aside go back.")
			return nil
		case ev.Key() == tcell.KeyRune && ev.Rune() == 'd':
			act("-delete-rollback-copies", "Delete the safety copies of the roll back of %s? It can no longer be undone.")
			return nil
		}
		return ev
	})
	flex := tview.NewFlex().SetDirection(tview.FlexRow).AddItem(tbl, 0, 1, true).AddItem(foot, 1, 0, false)
	a.modal = true
	a.pages.AddPage("rollbacks", flex, true, false)
	a.pages.SwitchToPage("rollbacks")
	a.app.SetFocus(tbl)
	fill(nil, nil)
	load()
}
