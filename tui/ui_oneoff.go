package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/salvanza/salvanza_client/gui/api"
)

// oneOff is the form for a one-off folder backup, the desktop app's "Back up
// now" without a Backup Set: folders, backup ID, server and a few options.
func (a *App) oneOff() {
	host, _ := os.Hostname()
	folders := tview.NewTextArea().SetLabel("Folders ").SetSize(4, 56)
	folders.SetBorder(false)
	backupID := tview.NewInputField().SetLabel("Backup ID ").SetText(host).SetFieldWidth(40)
	a.mu.Lock()
	options := []string{"(default server)"}
	ids := []string{""}
	for _, s := range a.servers {
		options = append(options, s.Name)
		ids = append(ids, s.ID)
	}
	a.mu.Unlock()
	server := tview.NewDropDown().SetLabel("PBS server ").SetOptions(options, nil).SetCurrentOption(0)
	compression := []string{"default", "fastest", "better", "best"}
	comp := tview.NewDropDown().SetLabel("Compression ").SetOptions(compression, nil).SetCurrentOption(0)
	vss := tview.NewCheckbox().SetLabel("Use VSS snapshot (Windows) ").SetChecked(true)
	exclude := tview.NewTextArea().SetLabel("Leave out ").SetSize(3, 56)
	exclude.SetBorder(false)
	comment := tview.NewInputField().SetLabel("Comment ").SetFieldWidth(50)

	form := styleForm(tview.NewForm()).AddFormItem(folders).AddFormItem(backupID).AddFormItem(server).
		AddFormItem(comp).AddFormItem(vss).AddFormItem(exclude).AddFormItem(comment)
	form.SetTitle(" Back up folders now ")
	closeForm := func() {
		a.pages.RemovePage("oneoff")
		a.modal = false
		a.switchTab(a.tab)
	}
	start := func() {
		var dirs []string
		for _, d := range splitLines(folders.GetText()) {
			dirs = append(dirs, fmt.Sprint(d))
		}
		if len(dirs) == 0 {
			a.errorBox("Add at least one folder to back up (one per line).")
			return
		}
		var skip []string
		for _, e := range splitLines(exclude.GetText()) {
			skip = append(skip, fmt.Sprint(e))
		}
		si, _ := server.GetCurrentOption()
		_, c := comp.GetCurrentOption()
		if c == "default" {
			c = ""
		}
		req := api.BackupRequest{
			BackupType: "directory", BackupID: strings.TrimSpace(backupID.GetText()), BackupDirs: dirs, ExcludeList: skip,
			UseVSS: vss.IsChecked(), Compression: c, PBSServerID: ids[si], Comment: strings.TrimSpace(comment.GetText()),
		}
		go func() {
			err := a.be.StartFolderBackup(req)
			a.app.QueueUpdateDraw(func() {
				if err != nil {
					a.errorBox(describeErr(err))
					return
				}
				closeForm()
				a.setFlash("started the one-off backup")
				go a.refresh(false)
			})
		}()
	}
	form.AddButton("Start", start).AddButton("Cancel", closeForm)
	form.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		switch ev.Key() {
		case tcell.KeyEscape:
			closeForm()
			return nil
		case tcell.KeyCtrlS:
			start()
			return nil
		}
		return ev
	})
	flex := tview.NewFlex().AddItem(nil, 0, 1, false).AddItem(form, 90, 0, true).AddItem(nil, 0, 1, false)
	a.modal = true
	a.pages.AddPage("oneoff", flex, true, false)
	a.pages.SwitchToPage("oneoff")
	a.app.SetFocus(form)
}
