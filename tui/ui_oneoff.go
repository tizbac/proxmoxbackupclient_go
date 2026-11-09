package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/salvanza/salvanza_client/gui/api"
)

// oneOff asks what to back up now, then opens that form.
func (a *App) oneOff() {
	m := tview.NewModal().SetText("What should be backed up now?").AddButtons([]string{"Folders", "Disk image", "Cancel"})
	m.SetBackgroundColor(colPanel).SetTextColor(colWhite).SetButtonBackgroundColor(colAccent).SetButtonStyle(onColour(colAccent))
	m.SetDoneFunc(func(_ int, label string) {
		a.pages.RemovePage("oneofftype")
		a.modal = false
		switch label {
		case "Folders":
			a.oneOffFolders()
		case "Disk image":
			a.oneOffDisks()
		default:
			a.switchTab(a.tab)
		}
	})
	a.modal = true
	a.pages.AddPage("oneofftype", m, true, true)
	a.app.SetFocus(m)
}

// oneOffDisks is the form for a one-off disk image backup, the desktop app's
// "Back up now" with Machine backup: the disks, backup ID, server and options.
func (a *App) oneOffDisks() {
	host, _ := os.Hostname()
	var sel machineSel
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
	comp := tview.NewDropDown().SetLabel("Compression ").SetOptions([]string{"default", "fastest", "better", "best"}, nil).SetCurrentOption(0)
	vss := tview.NewCheckbox().SetLabel("Use VSS snapshot (Windows) ").SetChecked(true)
	asVM := tview.NewCheckbox().SetLabel("Back up as a Proxmox VM (vm/ID) ")
	vmID := tview.NewInputField().SetLabel("  VM number (1 to 999999) ").SetFieldWidth(8)
	vmStyle := tview.NewDropDown().SetLabel("  Backup ID in PBS ").SetOptions(vmStyles, nil).SetCurrentOption(0)
	comment := tview.NewInputField().SetLabel("Comment ").SetFieldWidth(50)

	form := styleForm(tview.NewForm())
	form.AddTextView("Disks ", sel.summary(), 66, 2, true, false)
	form.AddFormItem(backupID).AddFormItem(server).AddFormItem(comp).AddFormItem(vss).
		AddFormItem(asVM).AddFormItem(vmID).AddFormItem(vmStyle).AddFormItem(comment)
	form.SetTitle(" Back up disks now ")
	closeForm := func() {
		a.pages.RemovePage("oneoffdisks")
		a.modal = false
		a.switchTab(a.tab)
	}
	choose := func() {
		back := func() {
			a.pages.SwitchToPage("oneoffdisks")
			a.app.SetFocus(form)
		}
		a.pickDisks(sel, false, func(s machineSel) {
			sel = s
			if tv, ok := form.GetFormItemByLabel("Disks ").(*tview.TextView); ok {
				tv.SetText(s.summary())
			}
			back()
		}, back)
	}
	start := func() {
		if len(sel.paths) == 0 {
			a.errorBox("Choose at least one disk to back up (Choose disks).")
			return
		}
		si, _ := server.GetCurrentOption()
		_, c := comp.GetCurrentOption()
		if c == "default" {
			c = ""
		}
		req := api.BackupRequest{
			BackupType: "machine", BackupID: strings.TrimSpace(backupID.GetText()), DriveLetters: sel.paths, DiskIDs: sel.ids,
			UseVSS: vss.IsChecked(), Compression: c, PBSServerID: ids[si], Comment: strings.TrimSpace(comment.GetText()),
		}
		if asVM.IsChecked() {
			sty, _ := vmStyle.GetCurrentOption()
			id := vmBackupID(vmID.GetText(), sty)
			if id == "" {
				a.errorBox("Enter a VM number between 1 and 999999.")
				return
			}
			req.BackupType, req.BackupID = "machine-vm", id
		}
		go func() {
			err := a.be.StartMachineBackup(req)
			a.app.QueueUpdateDraw(func() {
				if err != nil {
					a.errorBox(describeErr(err))
					return
				}
				closeForm()
				a.setFlash("started the one-off disk backup")
				go a.refresh(false)
			})
		}()
	}
	form.AddButton("Choose disks", choose).AddButton("Start", start).AddButton("Cancel", closeForm)
	form.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		switch ev.Key() {
		case tcell.KeyEscape:
			closeForm()
			return nil
		case tcell.KeyCtrlS:
			start()
			return nil
		case tcell.KeyCtrlD:
			choose()
			return nil
		}
		return ev
	})
	flex := tview.NewFlex().AddItem(nil, 0, 1, false).AddItem(form, 90, 0, true).AddItem(nil, 0, 1, false)
	a.modal = true
	a.pages.AddPage("oneoffdisks", flex, true, false)
	a.pages.SwitchToPage("oneoffdisks")
	a.app.SetFocus(form)
}

// oneOffFolders is the form for a one-off folder backup, the desktop app's "Back up
// now" without a Backup Set: folders, backup ID, server and a few options.
func (a *App) oneOffFolders() {
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
