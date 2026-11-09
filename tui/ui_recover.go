package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// Undelete and Roll back for the selected Backup Set, on the service program's
// command line (backend_tool.go). Both look first (a dry run) and show what
// they found; Roll back then waits for a y before it changes anything.

var rollbackPolicies = []struct{ id, text string }{
	{"replace", "replace: restore missing and changed files, keep files added since"},
	{"missing", "missing: restore only the files that are gone"},
	{"exact", "exact: also remove files added since"},
}

func (a *App) formModal(page string, form *tview.Form, width int, cancel func()) {
	form.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		if ev.Key() == tcell.KeyEscape {
			cancel()
			return nil
		}
		return ev
	})
	flex := tview.NewFlex().AddItem(nil, 0, 1, false).AddItem(form, width, 0, true).AddItem(nil, 0, 1, false)
	a.modal = true
	a.pages.AddPage(page, flex, true, false)
	a.pages.SwitchToPage(page)
	a.app.SetFocus(form)
}

func (a *App) closeModal(page string) {
	a.pages.RemovePage(page)
	a.modal = false
	a.switchTab(a.tab)
}

// undeleteSet is the Undelete form of the selected Backup Set.
func (a *App) undeleteSet() {
	s, ok := a.selectedSet()
	if !ok || (a.tab != 0 && a.tab != 1) {
		return
	}
	days := tview.NewInputField().SetLabel("Also look in the backups of the last N days (0 = the newest) ").SetFieldWidth(5).
		SetAcceptanceFunc(tview.InputFieldInteger).SetText("0")
	to := tview.NewInputField().SetLabel("Restore under this folder (empty = where the files were) ").SetFieldWidth(40)
	verify := tview.NewCheckbox().SetLabel("Check the restored files against the backup ").SetChecked(true)
	form := styleForm(tview.NewForm()).AddFormItem(days).AddFormItem(to).AddFormItem(verify)
	form.SetTitle(" Undelete: " + s.Name + " ")
	form.AddTextView("", tag(colDim)+"Finds files that are in the backups but gone from the disk, and puts them back. A file that has come back is never replaced. Files that look moved are listed, not restored.", 80, 3, true, false)
	args := func(dry bool) []string {
		out := []string{"-undelete", s.ID}
		if n := atoi(days); n > 0 {
			out = append(out, "-days", strconv.Itoa(n))
		}
		if t := strings.TrimSpace(to.GetText()); t != "" {
			out = append(out, "-to", t)
		}
		if !verify.IsChecked() {
			out = append(out, "-no-verify")
		}
		if dry {
			out = append(out, "-dry-run")
		}
		return out
	}
	cancel := func() { a.closeModal("recover") }
	form.AddButton("Look", func() {
		a.closeModal("recover")
		a.toolOutput("Undelete: "+s.Name+" (looking)", args(true), nil)
	})
	form.AddButton("Restore", func() {
		a.closeModal("recover")
		a.toolOutput("Undelete: "+s.Name, args(false), nil)
	})
	form.AddButton("Cancel", cancel)
	a.formModal("recover", form, 96, cancel)
}

// rollbackSet is the Roll back form of the selected Backup Set.
func (a *App) rollbackSet() {
	s, ok := a.selectedSet()
	if !ok || (a.tab != 0 && a.tab != 1) {
		return
	}
	at := tview.NewInputField().SetLabel("Back to the backup at or before (YYYY-MM-DD HH:MM, empty = newest) ").SetFieldWidth(18)
	var texts []string
	for _, p := range rollbackPolicies {
		texts = append(texts, p.text)
	}
	policy := tview.NewDropDown().SetLabel("What to do ").SetOptions(texts, nil).SetCurrentOption(0)
	first := tview.NewCheckbox().SetLabel("Back the set up first ")
	verify := tview.NewCheckbox().SetLabel("Check the restored files against the backup ").SetChecked(true)
	form := styleForm(tview.NewForm()).AddFormItem(at).AddFormItem(policy).AddFormItem(first).AddFormItem(verify)
	form.SetTitle(" Roll back: " + s.Name + " ")
	form.AddTextView("", tag(colDim)+"Puts the set's folders back as that backup had them. It compares first and shows the counts, then waits for y. Everything replaced or removed is kept in a safety folder; undoing the roll back is done in the desktop app.", 80, 4, true, false)
	args := func() []string {
		i, _ := policy.GetCurrentOption()
		out := []string{"-rollback", s.ID, "-policy", rollbackPolicies[i].id}
		if t := strings.TrimSpace(at.GetText()); t != "" {
			out = append(out, "-at", t)
		}
		if first.IsChecked() {
			out = append(out, "-backup-first")
		}
		if !verify.IsChecked() {
			out = append(out, "-no-verify")
		}
		return out
	}
	cancel := func() { a.closeModal("recover") }
	form.AddButton("Preview", func() {
		a.closeModal("recover")
		a.toolOutput("Roll back: "+s.Name+" (preview)", append(args(), "-dry-run"), nil)
	})
	form.AddButton("Roll back", func() {
		a.closeModal("recover")
		a.toolOutput("Roll back: "+s.Name, append(args(), "-dry-run"), append(args(), "-yes"))
	})
	form.AddButton("Cancel", cancel)
	a.formModal("recover", form, 100, cancel)
}

// toolOutput runs the service program with args and shows its output as it
// comes. With next, it then waits for y and runs next (the real roll back after
// its preview). Esc closes the window; a run that has started goes on.
func (a *App) toolOutput(title string, args, next []string) {
	tv := tview.NewTextView().SetDynamicColors(true).SetScrollable(true)
	tv.SetBorder(true).SetTitle(" " + title + " ").SetBackgroundColor(colPanel)
	foot := tview.NewTextView().SetDynamicColors(true)
	state, pending := "running", next
	setFoot := func(s string) { foot.SetText(" " + tag(colDim) + s) }
	run := func(args, next []string) {
		state, pending = "running", next
		setFoot("running ...  Esc closes this window; the work goes on")
		go func() {
			err := a.be.RunSetTool(args, func(l string) {
				a.app.QueueUpdateDraw(func() {
					_, _ = fmt.Fprintln(tv, tview.Escape(l))
					tv.ScrollToEnd()
				})
			})
			a.app.QueueUpdateDraw(func() {
				switch {
				case err != nil:
					state = "done"
					_, _ = fmt.Fprintf(tv, "\n%s%s[-]\n", tag(colErr), tview.Escape(describeErr(err)))
					setFoot("failed.  Esc closes")
				case pending != nil:
					state = "ask"
					setFoot("Press y to go ahead with this, Esc to cancel")
				default:
					state = "done"
					setFoot("finished.  Esc closes")
					go a.refresh(false)
				}
				tv.ScrollToEnd()
			})
		}()
	}
	tv.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		switch {
		case ev.Key() == tcell.KeyEscape:
			a.closeModal("tool")
			return nil
		case ev.Key() == tcell.KeyRune && (ev.Rune() == 'y' || ev.Rune() == 'Y') && state == "ask":
			_, _ = fmt.Fprintf(tv, "\n%s--- going ahead ---[-]\n", tag(colDim))
			run(pending, nil)
			return nil
		}
		return ev
	})
	flex := tview.NewFlex().SetDirection(tview.FlexRow).AddItem(tv, 0, 1, true).AddItem(foot, 1, 0, false)
	a.modal = true
	a.pages.AddPage("tool", flex, true, false)
	a.pages.SwitchToPage("tool")
	a.app.SetFocus(tv)
	run(args, next)
}
