package tui

import (
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

var compressionLevels = []string{"default", "fastest", "better", "best"}

// prefsView is the Preferences screen: Backup options and E-mail, as in the
// desktop app. Only what changed is sent, through the service's own setters.
type prefsView struct {
	a     *App
	pages *tview.Pages
	root  *tview.Flex
	title *tview.TextView
	step  int
	old   Prefs

	upload, download, disk, threads, chunk, workers, runs *tview.InputField
	compression                                           *tview.DropDown
	parRestore, parRead                                   *tview.Checkbox

	host, port, user, pass, from, testTo *tview.InputField
	insecure                             *tview.Checkbox

	shareList                                    *tview.TextView
	shareServer, shareUser, sharePass, shareTest *tview.InputField
	setFile                                      *tview.InputField
	setSecrets                                   *tview.Checkbox
}

func (a *App) buildPrefs() tview.Primitive {
	p := &prefsView{a: a}
	a.pf = p
	num := func(label string, w int) *tview.InputField {
		return tview.NewInputField().SetLabel(label).SetFieldWidth(w).SetAcceptanceFunc(tview.InputFieldInteger)
	}
	p.upload = num("Upload limit, MB/s (0 = none) ", 7)
	p.download = num("Download limit, MB/s (0 = none) ", 7)
	p.disk = num("Disk read limit, MB/s (0 = none) ", 7)
	p.threads = num("Upload threads (0 = automatic) ", 4)
	p.chunk = num("Chunk timeout, seconds (0 = 90) ", 6)
	p.runs = num("Backups at once (0 = default) ", 4)
	p.workers = num("Read-ahead workers (0 = default) ", 4)
	p.compression = tview.NewDropDown().SetLabel("Compression ").SetOptions(compressionLevels, nil)
	p.parRestore = tview.NewCheckbox().SetLabel("Restore files in parallel ")
	p.parRead = tview.NewCheckbox().SetLabel("Read folders ahead while backing up ")

	p.host = tview.NewInputField().SetLabel("SMTP server ").SetFieldWidth(40)
	p.port = tview.NewInputField().SetLabel("Port ").SetFieldWidth(6)
	p.user = tview.NewInputField().SetLabel("User name ").SetFieldWidth(40)
	p.pass = tview.NewInputField().SetLabel("Password ").SetFieldWidth(40).SetMaskCharacter('*')
	p.insecure = tview.NewCheckbox().SetLabel("Accept an untrusted mail certificate ")
	p.from = tview.NewInputField().SetLabel("Send from ").SetFieldWidth(40)
	p.testTo = tview.NewInputField().SetLabel("Test message to ").SetFieldWidth(40)

	f1 := styleForm(tview.NewForm()).AddFormItem(p.upload).AddFormItem(p.download).AddFormItem(p.disk).AddFormItem(p.threads).
		AddFormItem(p.chunk).AddFormItem(p.compression).AddFormItem(p.runs).AddFormItem(p.parRestore).
		AddFormItem(p.parRead).AddFormItem(p.workers)
	f1.SetTitle(" Backup options ")
	f2 := styleForm(tview.NewForm()).AddFormItem(p.host).AddFormItem(p.port).AddFormItem(p.user).AddFormItem(p.pass).
		AddFormItem(p.insecure).AddFormItem(p.from).AddFormItem(p.testTo)
	f2.AddTextView("", tag(colDim)+"Leave the password empty to keep the stored one. Every Backup Set's e-mail setting uses this account.", 70, 2, true, false)
	f2.SetTitle(" E-mail ")

	p.shareList = tview.NewTextView().SetDynamicColors(true)
	p.shareServer = tview.NewInputField().SetLabel(`Server (as in \\server\share) `).SetFieldWidth(30)
	p.shareUser = tview.NewInputField().SetLabel("User name ").SetFieldWidth(30)
	p.sharePass = tview.NewInputField().SetLabel("Password ").SetFieldWidth(30).SetMaskCharacter('*')
	p.shareTest = tview.NewInputField().SetLabel(`Try it on a folder (optional, \\server\share\folder) `).SetFieldWidth(40)
	f3 := styleForm(tview.NewForm()).AddFormItem(p.shareServer).AddFormItem(p.shareUser).AddFormItem(p.sharePass).AddFormItem(p.shareTest)
	f3.AddTextView("", tag(colDim)+"Backups that run in the service reach another computer's share with the login saved for its server. Leave the password empty to keep the stored one. To check a folder without saving, fill only the folder and choose Check folder.", 76, 3, true, false)
	f3.SetTitle(" Network shares ")

	p.setFile = tview.NewInputField().SetLabel("File ").SetFieldWidth(60)
	p.setSecrets = tview.NewCheckbox().SetLabel("Include passwords and secrets in the export ")
	f4 := styleForm(tview.NewForm()).AddFormItem(p.setFile).AddFormItem(p.setSecrets)
	f4.AddTextView("", tag(colDim)+"Export writes the Backup Sets, servers and preferences to a new file (never over an existing one; with secrets it is readable only by you). Import merges such a file into this computer's settings, in the service.", 76, 3, true, false)
	f4.SetTitle(" Settings file ")

	p.title = tview.NewTextView().SetDynamicColors(true)
	p.pages = tview.NewPages()
	forms := []*tview.Form{f1, f2, f3, f4}
	for i, f := range forms {
		i := i
		if i > 0 {
			f.AddButton("Previous page", func() { p.show(i - 1) })
		}
		if i < len(forms)-1 {
			f.AddButton("Next page", func() { p.show(i + 1) })
		}
		switch i {
		case 0, 1:
			f.AddButton("Save", p.save)
			if i == 1 {
				f.AddButton("Save and send test message", p.sendTest)
			}
			f.AddButton("Revert", p.load)
		case 2:
			f.AddButton("Save login", p.saveShare)
			f.AddButton("Remove login", p.removeShare)
			f.AddButton("Check folder", p.checkShare)
		case 3:
			f.AddButton("Export", p.exportSettings)
			f.AddButton("Import", p.importSettings)
		}
		f.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
			if d := altStep(ev); d != 0 {
				if j := i + d; j >= 0 && j < len(forms) {
					p.show(j)
				}
				return nil
			}
			switch ev.Key() {
			case tcell.KeyPgDn:
				if i < len(forms)-1 {
					p.show(i + 1)
				}
				return nil
			case tcell.KeyPgUp:
				if i > 0 {
					p.show(i - 1)
				}
				return nil
			case tcell.KeyCtrlS:
				switch i {
				case 0, 1:
					p.save()
				case 2:
					p.saveShare()
				}
				return nil
			}
			return ev
		})
		var page tview.Primitive = f
		if i == 2 {
			page = tview.NewFlex().SetDirection(tview.FlexRow).AddItem(p.shareList, 6, 0, false).AddItem(f, 0, 1, true)
		}
		p.pages.AddPage(strconv.Itoa(i), page, true, i == 0)
	}
	p.root = tview.NewFlex().SetDirection(tview.FlexRow).AddItem(p.title, 1, 0, false).AddItem(p.pages, 0, 1, true)
	p.renderTitle()
	return p.root
}

func (p *prefsView) renderTitle() {
	b := strings.Builder{}
	b.WriteString(stepBar([]string{"Backup options", "E-mail", "Network shares", "Settings file"}, p.step, false))
	b.WriteString(tag(colDim) + "   Tab next field, PgDn and PgUp switch page, Ctrl+S save, F1 to F9 other screens")
	p.title.SetText(b.String())
}

func (p *prefsView) show(step int) {
	p.step = step
	p.pages.SwitchToPage(strconv.Itoa(step))
	p.renderTitle()
	_, front := p.pages.GetFrontPage()
	p.a.app.SetFocus(front)
}

// enter is called when the screen is shown.
func (p *prefsView) enter() {
	p.load()
	p.loadShares()
	p.show(p.step)
}

func (p *prefsView) fill(v Prefs) {
	p.old = v
	set := func(f *tview.InputField, n int) { f.SetText(strconv.Itoa(n)) }
	set(p.upload, v.UploadMBps)
	set(p.download, v.DownloadMBps)
	set(p.disk, v.DiskReadMBps)
	set(p.threads, v.UploadThreads)
	set(p.chunk, v.ChunkTimeout)
	set(p.runs, v.MaxRuns)
	set(p.workers, v.ReadWorkers)
	idx := 0
	for i, c := range compressionLevels {
		if c == v.Compression {
			idx = i
		}
	}
	p.compression.SetCurrentOption(idx)
	p.parRestore.SetChecked(v.ParallelRestore)
	p.parRead.SetChecked(v.ParallelRead)
	p.host.SetText(v.SMTPHost)
	p.port.SetText(v.SMTPPort)
	p.user.SetText(v.SMTPUser)
	p.pass.SetText("")
	p.insecure.SetChecked(v.SMTPInsecure)
	p.from.SetText(v.MailFrom)
}

func (p *prefsView) load() {
	go func() {
		v, err := p.a.be.Prefs()
		p.a.app.QueueUpdateDraw(func() {
			if err != nil {
				p.a.errorBox(describeErr(err))
				return
			}
			p.fill(v)
		})
	}()
}

func atoi(f *tview.InputField) int {
	n, _ := strconv.Atoi(strings.TrimSpace(f.GetText()))
	return n
}

// values reads the form; the service checks the ranges.
func (p *prefsView) values() Prefs {
	_, comp := p.compression.GetCurrentOption()
	v := p.old
	v.UploadMBps, v.DownloadMBps, v.DiskReadMBps = atoi(p.upload), atoi(p.download), atoi(p.disk)
	v.UploadThreads, v.ChunkTimeout, v.MaxRuns, v.ReadWorkers = atoi(p.threads), atoi(p.chunk), atoi(p.runs), atoi(p.workers)
	v.Compression = comp
	v.ParallelRestore, v.ParallelRead = p.parRestore.IsChecked(), p.parRead.IsChecked()
	v.SMTPHost, v.SMTPPort, v.SMTPUser = strings.TrimSpace(p.host.GetText()), strings.TrimSpace(p.port.GetText()), strings.TrimSpace(p.user.GetText())
	v.SMTPPassword = p.pass.GetText()
	v.SMTPInsecure = p.insecure.IsChecked()
	v.MailFrom = strings.TrimSpace(p.from.GetText())
	return v
}

func (p *prefsView) save() { p.persist(nil) }

// persist saves, then runs after on the UI goroutine when it worked.
func (p *prefsView) persist(after func()) {
	now := p.values()
	old := p.old
	go func() {
		err := p.a.be.SavePrefs(old, now)
		p.a.app.QueueUpdateDraw(func() {
			if err != nil {
				p.a.errorBox(describeErr(err))
				return
			}
			p.a.setFlash("preferences saved")
			p.load()
			if after != nil {
				after()
			}
		})
	}()
}

func (p *prefsView) sendTest() {
	to := strings.TrimSpace(p.testTo.GetText())
	if to == "" {
		p.a.errorBox("Give the address to send the test message to.")
		return
	}
	p.persist(func() {
		p.a.setFlash("sending the test message ...")
		go func() {
			err := p.a.be.SendTestMail(to)
			msg := "test message sent to " + to
			if err != nil {
				msg = "the test message failed: " + describeErr(err)
			}
			p.a.app.QueueUpdateDraw(func() { p.a.setFlash(msg) })
		}()
	})
}
