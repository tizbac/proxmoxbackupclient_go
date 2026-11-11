package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rivo/tview"
)

// The Network shares and Settings file pages of Preferences.

func (p *prefsView) loadShares() {
	go func() {
		list, err := p.a.be.ShareLogins()
		p.a.app.QueueUpdateDraw(func() {
			if err != nil {
				p.shareList.SetText(tag(colErr) + describeErr(err))
				return
			}
			var b strings.Builder
			b.WriteString(tag(colDim) + "Saved logins\n")
			if len(list) == 0 {
				b.WriteString("  none\n")
			}
			for _, l := range list {
				pw := "no password"
				if l.PasswordSet {
					pw = "password stored"
				}
				fmt.Fprintf(&b, "[-]  \\\\%s   %s   %s\n", l.Server, l.Username, tag(colDim)+pw)
			}
			p.shareList.SetText(b.String())
		})
	}()
}

func (p *prefsView) saveShare() {
	server := strings.TrimSpace(p.shareServer.GetText())
	user := strings.TrimSpace(p.shareUser.GetText())
	pass := p.sharePass.GetText()
	test := strings.TrimSpace(p.shareTest.GetText())
	p.a.setFlash("saving the login ...")
	go func() {
		err := p.a.be.SetShareLogin(server, user, pass, test)
		p.a.app.QueueUpdateDraw(func() {
			if err != nil {
				p.a.errorBox(describeErr(err))
				return
			}
			p.sharePass.SetText("")
			p.a.setFlash("login saved for " + server)
			p.loadShares()
		})
	}()
}

func (p *prefsView) removeShare() {
	server := strings.TrimSpace(p.shareServer.GetText())
	if server == "" {
		p.a.errorBox("Give the server whose login is to be removed.")
		return
	}
	p.a.confirm(fmt.Sprintf("Remove the saved login for %s? Backups of its shares then read them without a login.", server), func() {
		go func() {
			err := p.a.be.DeleteShareLogin(server)
			p.a.app.QueueUpdateDraw(func() {
				if err != nil {
					p.a.errorBox(describeErr(err))
					return
				}
				p.a.setFlash("login removed for " + server)
				p.loadShares()
			})
		}()
	})
}

func (p *prefsView) checkShare() {
	path := strings.TrimSpace(p.shareTest.GetText())
	if path == "" {
		p.a.errorBox(`Give a folder on the share to try, such as \\server\share\folder.`)
		return
	}
	p.a.setFlash("trying the folder ...")
	go func() {
		res, err := p.a.be.CheckShares([]string{path})
		p.a.app.QueueUpdateDraw(func() {
			switch {
			case err != nil:
				p.a.errorBox(describeErr(err))
			case len(res) == 0:
				p.a.errorBox(path + " is not a folder on a network share.")
			case res[0].OK && res[0].HasLogin:
				p.a.infoBox("The backups can read " + path + " with the saved login for " + res[0].Server + ".")
			case res[0].OK:
				p.a.infoBox("The backups can read " + path + " (no login is needed).")
			case res[0].NeedLogin:
				p.a.errorBox(path + " was refused for want of a login. Fill in the server, user name and password and choose Save login.")
			default:
				p.a.errorBox(path + ": " + res[0].Error)
			}
		})
	}()
}

func (p *prefsView) exportSettings() {
	path := strings.TrimSpace(p.setFile.GetText())
	secrets := p.setSecrets.IsChecked()
	if path == "" {
		p.a.errorBox("Give the file to write.")
		return
	}
	go func() {
		err := p.a.be.ExportSettings(path, secrets)
		p.a.app.QueueUpdateDraw(func() {
			if err != nil {
				p.a.errorBox(describeErr(err))
				return
			}
			note := "Settings written to " + path + "."
			if secrets {
				note += " The file holds passwords and secrets: keep it safe."
			}
			p.a.infoBox(note)
		})
	}()
}

// defaultLogsPath is a new file name for the logs, in the user's home folder.
func defaultLogsPath() string {
	name := "salvanza-logs-" + time.Now().Format("20060102-150405") + ".zip"
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return filepath.Join(home, name)
	}
	return name
}

// saveLogs asks for the file (a new name is filled in, in the home folder) and
// writes the service's logs of the last two weeks into it.
func (p *prefsView) saveLogs() {
	path := tview.NewInputField().SetLabel("File ").SetFieldWidth(60).SetText(defaultLogsPath())
	form := styleForm(tview.NewForm()).AddFormItem(path)
	form.SetTitle(" Save logs ")
	form.AddTextView("", tag(colDim)+"One zip with the service's logs of the last two weeks, to attach to a bug report. A file that already exists is never overwritten. The logs name your folders and servers: look through them before you post them.", 70, 4, true, false)
	cancel := func() { p.a.closeModal("savelogs") }
	form.AddButton("Save", func() {
		dest := strings.TrimSpace(path.GetText())
		if dest == "" {
			p.a.errorBox("Give the file to write.")
			return
		}
		p.a.closeModal("savelogs")
		go func() {
			err := p.a.be.SaveLogs(dest)
			p.a.app.QueueUpdateDraw(func() {
				if err != nil {
					p.a.errorBox(describeErr(err))
					return
				}
				p.a.infoBox("Logs written to " + dest + ". Look through them before you post them.")
			})
		}()
	})
	form.AddButton("Cancel", cancel)
	p.a.formModal("savelogs", form, 80, cancel)
}

func (p *prefsView) importSettings() {
	path := strings.TrimSpace(p.setFile.GetText())
	if path == "" {
		p.a.errorBox("Give the file to read.")
		return
	}
	p.a.confirm("Merge the settings in "+path+" into this computer's settings? Items with the same name are replaced.", func() {
		go func() {
			summary, err := p.a.be.ImportSettings(path)
			p.a.app.QueueUpdateDraw(func() {
				if err != nil {
					p.a.errorBox(describeErr(err))
					return
				}
				p.a.infoBox(summary)
				p.load()
				p.loadShares()
			})
		}()
	})
}
