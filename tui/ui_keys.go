package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"pbscommon"
)

// Encryption keys in the window. The key file stays in the service's keys
// folder and its contents are never shown: only fingerprints and where it is.
// Printing a paper key and reading one back (QR code, typed text) are left to
// the desktop app on purpose.

// loadKeyInfo asks the service about a key file and redraws the server's
// details with the answer.
func (a *App) loadKeyInfo(path string) {
	info, err := a.be.KeyInfo(path)
	if err != nil {
		info = KeyInfo{Path: path, Reason: describeErr(err)}
	}
	a.app.QueueUpdateDraw(func() {
		a.mu.Lock()
		if a.keyCache == nil {
			a.keyCache = map[string]KeyInfo{}
		}
		a.keyCache[path] = info
		a.mu.Unlock()
		a.renderSrvDetail()
	})
}

// forgetKeyInfo drops what is known of a key file after it changed.
func (a *App) forgetKeyInfo(path string) {
	a.mu.Lock()
	delete(a.keyCache, path)
	a.mu.Unlock()
}

// setEncryption says in words whether a Backup Set's backups are encrypted: by
// the key of the server it backs up to.
func (a *App) setEncryption(s Set) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, sv := range a.servers {
		if (s.ServerID != "" && sv.ID == s.ServerID) || (s.ServerID == "" && sv.Default) {
			if sv.KeyFile != "" || sv.Encrypted {
				return tag(colOK) + "encrypted, with the key of " + tview.Escape(sv.Name)
			}
			return tag(colWarn) + "not encrypted (" + tview.Escape(sv.Name) + " has no key)"
		}
	}
	return tag(colDim) + "unknown"
}

// infoBox shows a message above the window; OK closes it.
func (a *App) infoBox(text string) {
	m := tview.NewModal().SetText(text).AddButtons([]string{"OK"})
	m.SetBackgroundColor(colPanel).SetTextColor(colText).SetButtonBackgroundColor(colAccent).SetButtonStyle(onColour(colAccent))
	prev := a.modal
	m.SetDoneFunc(func(int, string) {
		a.pages.RemovePage("info")
		a.modal = prev
		if _, front := a.pages.GetFrontPage(); front != nil && prev {
			a.app.SetFocus(front)
		}
	})
	a.modal = true
	a.pages.AddPage("info", m, true, true)
	a.app.SetFocus(m)
}

func describeKey(k KeyInfo) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", k.Path)
	switch {
	case !k.Exists:
		b.WriteString("The file does not exist.")
	case !k.Usable:
		b.WriteString("Not usable: " + k.Reason)
	default:
		fmt.Fprintf(&b, "Fingerprint: %s\n", k.Fingerprint)
		if k.Created != "" {
			fmt.Fprintf(&b, "Created: %s\n", k.Created)
		}
		if k.PassphraseProtected {
			b.WriteString("Protected by a passphrase.\n")
		} else {
			b.WriteString("No passphrase.\n")
		}
		if k.OpenToOthers {
			b.WriteString("Other accounts on this computer can read the file: use Restrict access.")
		} else {
			b.WriteString("Only the service and administrators can read the file.")
		}
	}
	return b.String()
}

// uniqueKeyName is a file name for a new key that cannot clash with an old one.
func uniqueKeyName(base string) string {
	base = strings.TrimSuffix(base, filepath.Ext(base))
	if base == "" {
		base = "encryption-key"
	}
	return fmt.Sprintf("%s-%s.json", base, time.Now().Format("20060102-150405"))
}

func (e *serverEditor) keyPath() string { return strings.TrimSpace(e.keyFile.GetText()) }

// generateKey makes a new key (without a passphrase, as the desktop app does)
// in the keys folder and puts it in the key file field.
func (e *serverEditor) generateKey() {
	a := e.a
	slug := serverSlug(e.name.GetText())
	if slug == "" {
		slug = "server"
	}
	name := uniqueKeyName(slug + "-key")
	if e.keyPath() != "" {
		a.confirm("This server has a key already.\nA new key does not read the backups the old one made.\nMake a new key anyway?", func() { e.doGenerate(name) })
		return
	}
	e.doGenerate(name)
}

func (e *serverEditor) doGenerate(name string) {
	a := e.a
	go func() {
		info, err := a.be.GenerateKey(name)
		a.app.QueueUpdateDraw(func() {
			if err != nil {
				a.errorBox(describeErr(err))
				return
			}
			e.keyFile.SetText(info.Path)
			e.forgetPass.SetChecked(true) // the new key has no passphrase to remember
			a.forgetKeyInfo(info.Path)
			a.infoBox(fmt.Sprintf("A new key was made:\n%s\n\nFingerprint:\n%s\n\nWrite the fingerprint down and keep a copy of the key file somewhere safe:\nwithout it the backups cannot be restored. (Save the server to use the key.)", info.Path, info.Fingerprint))
		})
	}()
}

// importKey takes a key file from anywhere on this computer (a USB stick, a
// backup copy), checks it, shows its fingerprint to compare with your notes,
// and stores a copy in the keys folder.
func (e *serverEditor) importKey() {
	a := e.a
	path := tview.NewInputField().SetLabel("Key file ").SetFieldWidth(60)
	pass := tview.NewInputField().SetLabel("Its passphrase ").SetFieldWidth(40).SetMaskCharacter('*')
	form := styleForm(tview.NewForm()).AddFormItem(path).AddFormItem(pass)
	form.AddTextView("", tag(colDim)+"The passphrase, if the key has one, lets the key be checked and stored for scheduled backups.", 66, 2, true, false)
	form.SetTitle(" Import a key file ")
	closeForm := func() {
		a.pages.RemovePage("keyimport")
		a.modal = true // back to the server editor
		if _, front := e.pages.GetFrontPage(); front != nil {
			a.app.SetFocus(front)
		}
	}
	form.AddButton("Read", func() {
		p := strings.TrimSpace(path.GetText())
		ph := pass.GetText()
		closeForm()
		e.readKeyFile(p, ph)
	})
	form.AddButton("Cancel", closeForm)
	form.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		if ev.Key() == tcell.KeyEscape {
			closeForm()
			return nil
		}
		return ev
	})
	box := tview.NewFlex().AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).AddItem(nil, 0, 1, false).AddItem(form, 11, 0, true).AddItem(nil, 0, 1, false), 76, 0, true).
		AddItem(nil, 0, 1, false)
	a.pages.AddPage("keyimport", box, true, true)
	a.app.SetFocus(form)
}

// checkKeyFile reads a key file and checks it the way the desktop app does:
// without a passphrase it must compute to its own fingerprint; with a
// passphrase it must unlock. A protected key without a passphrase given is
// accepted unchecked (checked is false).
func checkKeyFile(path, passphrase string) (data []byte, cfg *pbscommon.KeyConfig, checked bool, err error) {
	if path == "" {
		return nil, nil, false, fmt.Errorf("give the path of the key file")
	}
	data, err = os.ReadFile(path) // #nosec G304 -- the key file the administrator names
	if err != nil {
		return nil, nil, false, fmt.Errorf("cannot read %s: %w", path, err)
	}
	cfg = &pbscommon.KeyConfig{}
	if err := json.Unmarshal(data, cfg); err != nil || cfg.Data == "" {
		return nil, nil, false, fmt.Errorf("%s is not a Proxmox Backup Server key file", path)
	}
	if cfg.KDF == nil {
		if _, err := cfg.CryptConfig(nil); err != nil {
			return nil, nil, false, fmt.Errorf("the key is damaged: %w", err)
		}
		return data, cfg, true, nil
	}
	if passphrase == "" {
		return data, cfg, false, nil
	}
	if _, err := cfg.CryptConfig([]byte(passphrase)); err != nil {
		return nil, nil, false, fmt.Errorf("that passphrase does not unlock the key: %w", err)
	}
	return data, cfg, true, nil
}

func (e *serverEditor) readKeyFile(path, passphrase string) {
	a := e.a
	data, cfg, checked, err := checkKeyFile(path, passphrase)
	if err != nil {
		a.errorBox(err.Error())
		return
	}
	note := "The key was checked: it unlocks and matches its fingerprint."
	if !checked {
		note = "The key has a passphrase and none was given, so it was not checked here."
	}
	kind := "no passphrase"
	if cfg.KDF != nil {
		kind = "protected by a passphrase"
	}
	a.confirm(fmt.Sprintf("Import this key?\n\nFingerprint: %s\n%s, created %s\n%s\n\nCompare the fingerprint with your notes or paper copy.", cfg.Fingerprint, kind, firstNonEmpty(cfg.Created, "unknown"), note), func() {
		go func() {
			name := filepath.Base(path)
			info, err := a.be.StoreKey(name, data)
			if err != nil && strings.Contains(err.Error(), "already exists") {
				info, err = a.be.StoreKey(uniqueKeyName(name), data)
			}
			a.app.QueueUpdateDraw(func() {
				if err != nil {
					a.errorBox(describeErr(err))
					return
				}
				e.keyFile.SetText(info.Path)
				if passphrase != "" {
					e.keyPass.SetText(passphrase)
				}
				a.forgetKeyInfo(info.Path)
				a.infoBox("The key was imported:\n" + info.Path + "\n\nSave the server to use it.")
			})
		}()
	})
}

func (e *serverEditor) inspectKey() {
	a := e.a
	path := e.keyPath()
	if path == "" {
		a.infoBox("No key file is set for this server.")
		return
	}
	go func() {
		info, err := a.be.KeyInfo(path)
		a.app.QueueUpdateDraw(func() {
			if err != nil {
				a.errorBox(describeErr(err))
				return
			}
			a.infoBox(describeKey(info))
		})
	}()
}

func (e *serverEditor) restrictKey() {
	a := e.a
	path := e.keyPath()
	if path == "" {
		a.infoBox("No key file is set for this server.")
		return
	}
	go func() {
		info, err := a.be.RestrictKey(path)
		a.app.QueueUpdateDraw(func() {
			if err != nil {
				a.errorBox(describeErr(err))
				return
			}
			a.forgetKeyInfo(path)
			a.infoBox("Access restricted.\n\n" + describeKey(info))
		})
	}()
}
