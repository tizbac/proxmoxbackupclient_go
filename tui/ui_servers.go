package tui

import (
	"crypto/sha256"
	"crypto/tls"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// fetchFingerprint reads the SHA-256 fingerprint of the certificate a server
// presents (what PBS shows under Dashboard, Show Fingerprint), as AA:BB:...
// The connection is only used to read the certificate; nothing is sent.
func fetchFingerprint(address string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(address))
	if err != nil || u.Hostname() == "" {
		return "", fmt.Errorf("the address must look like https://pbs.example.com:8007")
	}
	port := u.Port()
	if port == "" {
		port = "8007"
	}
	d := &net.Dialer{Timeout: 10 * time.Second}
	conn, err := tls.DialWithDialer(d, "tcp", net.JoinHostPort(u.Hostname(), port), &tls.Config{InsecureSkipVerify: true}) // #nosec G402 -- only the certificate is read, to show its fingerprint
	if err != nil {
		return "", fmt.Errorf("cannot reach %s: %w", net.JoinHostPort(u.Hostname(), port), err)
	}
	defer func() { _ = conn.Close() }()
	certs := conn.ConnectionState().PeerCertificates
	if len(certs) == 0 {
		return "", fmt.Errorf("the server presented no certificate")
	}
	sum := sha256.Sum256(certs[0].Raw)
	parts := make([]string, len(sum))
	for i, b := range sum {
		parts[i] = fmt.Sprintf("%02X", b)
	}
	return strings.Join(parts, ":"), nil
}

func serverSlug(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_' || r == '.':
			if b.Len() > 0 && !strings.HasSuffix(b.String(), "-") {
				b.WriteByte('-')
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func (a *App) selectedServer() (Server, bool) {
	row, _ := a.srvTable.GetSelection()
	a.mu.Lock()
	defer a.mu.Unlock()
	if row < 1 || row > len(a.servers) {
		return Server{}, false
	}
	return a.servers[row-1], true
}

func (a *App) newServer() { a.openServerEditor(Server{Raw: map[string]interface{}{}}, true) }
func (a *App) editServer() {
	if s, ok := a.selectedServer(); ok {
		a.openServerEditor(s, false)
	}
}

func (a *App) deleteServer() {
	s, ok := a.selectedServer()
	if !ok {
		return
	}
	a.confirm(fmt.Sprintf("Delete the server %q?\nBackup Sets that use it need another server. Its backups stay on it.", s.Name), func() {
		go func() {
			err := a.be.DeleteServer(s.ID)
			msg := "deleted " + s.Name
			if err != nil {
				msg = describeErr(err)
			}
			a.app.QueueUpdateDraw(func() { a.setFlash(msg) })
			a.refresh(true)
		}()
	})
}

func (a *App) defaultServer() {
	s, ok := a.selectedServer()
	if !ok {
		return
	}
	go func() {
		err := a.be.SetDefaultServer(s.ID)
		msg := s.Name + " is the default server now"
		if err != nil {
			msg = describeErr(err)
		}
		a.app.QueueUpdateDraw(func() { a.setFlash(msg) })
		a.refresh(true)
	}()
}

func (a *App) testServer() {
	s, ok := a.selectedServer()
	if !ok {
		return
	}
	a.setFlash("testing the connection to " + s.Name + " ...")
	go func() {
		note, err := a.be.TestServer(s.ID)
		msg := "connection to " + s.Name + " works"
		if note != "" {
			msg += ", but " + note
		}
		if err != nil {
			msg = "connection to " + s.Name + " failed: " + describeErr(err)
		}
		a.app.QueueUpdateDraw(func() { a.setFlash(msg) })
	}()
}

// pinServer reads the certificate the server presents and, once you have
// compared the fingerprint with the one in the PBS web interface, stores it.
func (a *App) pinServer() {
	s, ok := a.selectedServer()
	if !ok {
		return
	}
	a.setFlash("reading the certificate of " + s.Name + " ...")
	go func() {
		fp, err := fetchFingerprint(s.URL)
		a.app.QueueUpdateDraw(func() {
			if err != nil {
				a.errorBox(describeErr(err))
				return
			}
			same := ""
			if strings.EqualFold(fp, s.Fingerprint) {
				same = "\n(It is the one stored already.)"
			}
			a.confirm(fmt.Sprintf("The certificate of %s has the fingerprint\n\n%s\n\nCompare it with the one in the Proxmox Backup Server\n(Dashboard, Show Fingerprint), then pin it?%s", s.Name, fp, same), func() {
				go func() {
					err := a.be.PinFingerprint(s.ID, fp)
					msg := "pinned the fingerprint of " + s.Name
					if err != nil {
						msg = describeErr(err)
					}
					a.app.QueueUpdateDraw(func() { a.setFlash(msg) })
					a.refresh(true)
				}()
			})
		})
	}()
}

// serverEditor is the server form: two steps, Server and Login.
type serverEditor struct {
	a     *App
	isNew bool
	orig  map[string]interface{}
	pages *tview.Pages
	title *tview.TextView

	name, address, fingerprint, datastore, namespace, description *tview.InputField
	login                                                         *tview.DropDown
	tokenID, secret, username, password                           *tview.InputField
	keyFile, keyPass                                              *tview.InputField
	forgetPass                                                    *tview.Checkbox
	serverID                                                      string
}

func (a *App) openServerEditor(s Server, isNew bool) {
	e := &serverEditor{a: a, isNew: isNew, orig: copyMap(s.Raw)}
	if e.orig == nil {
		e.orig = map[string]interface{}{}
	}
	in := func(label, val string, w int) *tview.InputField {
		return tview.NewInputField().SetLabel(label).SetText(val).SetFieldWidth(w)
	}
	e.name = in("Name ", str(e.orig, "name"), 40)
	e.address = in("Address ", str(e.orig, "baseurl"), 50)
	e.fingerprint = in("Fingerprint ", str(e.orig, "certfingerprint"), 64)
	e.datastore = in("Datastore ", str(e.orig, "datastore"), 30)
	e.namespace = in("Namespace ", str(e.orig, "namespace"), 30)
	e.description = in("Description ", str(e.orig, "description"), 50)
	e.login = tview.NewDropDown().SetLabel("Log in with ").SetOptions([]string{"API token", "User name and password"}, nil)
	if str(e.orig, "username") != "" {
		e.login.SetCurrentOption(1)
	}
	e.tokenID = in("Token ID ", str(e.orig, "authid"), 40)
	e.secret = in("Token secret ", "", 50).SetMaskCharacter('*')
	e.username = in("User name ", str(e.orig, "username"), 40)
	e.password = in("Password ", "", 50).SetMaskCharacter('*')
	e.keyFile = in("Key file ", str(e.orig, "encryption_key_file"), 70)
	e.keyPass = in("Key passphrase ", "", 40).SetMaskCharacter('*')
	e.forgetPass = tview.NewCheckbox().SetLabel("Forget the stored passphrase ")
	e.serverID = s.ID

	hint := tag(colDim) + "Leave a secret or password empty to keep the one stored."
	f1 := styleForm(tview.NewForm()).AddFormItem(e.name).AddFormItem(e.address).AddFormItem(e.fingerprint).
		AddFormItem(e.datastore).AddFormItem(e.namespace).AddFormItem(e.description)
	f1.SetTitle(" 1  Server ")
	f2 := styleForm(tview.NewForm()).AddFormItem(e.login).AddFormItem(e.tokenID).AddFormItem(e.secret).
		AddFormItem(e.username).AddFormItem(e.password)
	f2.AddTextView("", hint, 60, 1, true, false)
	f2.SetTitle(" 2  Login ")
	f3 := styleForm(tview.NewForm()).AddFormItem(e.keyFile).AddFormItem(e.keyPass).AddFormItem(e.forgetPass)
	f3.AddTextView("", tag(colDim)+"Backups to this server are encrypted with the key. Leave the key file empty for none.\n"+
		"A passphrase is stored so scheduled backups can run alone; leave it empty to keep the stored one.\n"+
		"Keep a copy of the key file somewhere safe: without it the backups cannot be restored.", 88, 4, true, false)
	f3.SetTitle(" 3  Encryption ")

	e.title = tview.NewTextView().SetDynamicColors(true)
	e.pages = tview.NewPages()
	forms := []*tview.Form{f1, f2, f3}
	for i, f := range forms {
		i := i
		if i == 0 {
			f.AddButton("Fetch fingerprint", e.fetch)
		} else {
			f.AddButton("Back", func() { e.show(i - 1) })
		}
		if i < len(forms)-1 {
			f.AddButton("Next", func() { e.show(i + 1) })
		}
		if i == 2 {
			f.AddButton("Generate key", e.generateKey)
			f.AddButton("Import key file", e.importKey)
			f.AddButton("Inspect", e.inspectKey)
			f.AddButton("Restrict access", e.restrictKey)
		}
		f.AddButton("Save", e.save)
		f.AddButton("Cancel", e.close)
		f.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
			switch ev.Key() {
			case tcell.KeyEscape:
				e.close()
				return nil
			case tcell.KeyPgDn:
				if i < len(forms)-1 {
					e.show(i + 1)
				}
				return nil
			case tcell.KeyPgUp:
				if i > 0 {
					e.show(i - 1)
				}
				return nil
			case tcell.KeyCtrlS:
				e.save()
				return nil
			}
			return ev
		})
		e.pages.AddPage(strconv.Itoa(i), f, true, i == 0)
	}
	body := tview.NewFlex().SetDirection(tview.FlexRow).AddItem(e.title, 1, 0, false).AddItem(e.pages, 0, 1, true)
	layout := tview.NewFlex().AddItem(nil, 0, 1, false).AddItem(body, 96, 0, true).AddItem(nil, 0, 1, false)
	a.modal = true
	a.pages.AddPage("srvedit", layout, true, false)
	a.pages.SwitchToPage("srvedit")
	e.show(0)
}

func (e *serverEditor) show(step int) {
	e.pages.SwitchToPage(strconv.Itoa(step))
	word := "Edit server"
	if e.isNew {
		word = "New server"
	}
	labels := []string{"Server", "Login", "Encryption"}
	var b strings.Builder
	b.WriteString(" ")
	for i, l := range labels {
		if i == step {
			fmt.Fprintf(&b, "[black:#%06x] %d %s [-:-] ", colHead.Hex(), i+1, l)
		} else {
			fmt.Fprintf(&b, "%s %d %s [-:-] ", tag(colDim), i+1, l)
		}
	}
	b.WriteString(tag(colDim) + "   " + word + "   PgDn next, PgUp back, Ctrl+S save, Esc cancel")
	e.title.SetText(b.String())
	_, front := e.pages.GetFrontPage()
	e.a.app.SetFocus(front)
}

func (e *serverEditor) close() {
	e.a.pages.RemovePage("srvedit")
	e.a.modal = false
	e.a.switchTab(e.a.tab)
}

func (e *serverEditor) fetch() {
	addr := e.address.GetText()
	e.a.setFlash("reading the certificate ...")
	go func() {
		fp, err := fetchFingerprint(addr)
		e.a.app.QueueUpdateDraw(func() {
			if err != nil {
				e.a.errorBox(describeErr(err))
				return
			}
			e.fingerprint.SetText(fp)
			e.a.setFlash("fingerprint filled in: compare it with the one in the Proxmox Backup Server before saving")
		})
	}()
}

func (e *serverEditor) document() (map[string]interface{}, error) {
	doc := copyMap(e.orig)
	name := strings.TrimSpace(e.name.GetText())
	if name == "" {
		return nil, fmt.Errorf("the server needs a name")
	}
	addr := strings.TrimSpace(e.address.GetText())
	if u, err := url.Parse(addr); err != nil || u.Scheme != "https" || u.Hostname() == "" {
		return nil, fmt.Errorf("the address must look like https://pbs.example.com:8007")
	}
	ds := strings.TrimSpace(e.datastore.GetText())
	if ds == "" {
		return nil, fmt.Errorf("the server needs a datastore")
	}
	doc["name"], doc["baseurl"], doc["datastore"] = name, addr, ds
	doc["certfingerprint"] = strings.TrimSpace(e.fingerprint.GetText())
	doc["namespace"] = strings.Trim(strings.TrimSpace(e.namespace.GetText()), "/")
	doc["description"] = strings.TrimSpace(e.description.GetText())
	if e.isNew {
		id := serverSlug(name)
		if id == "" {
			id = fmt.Sprint(time.Now().Unix())
		}
		e.a.mu.Lock()
		for _, s := range e.a.servers {
			if s.ID == id {
				id = fmt.Sprintf("%s-%d", id, time.Now().Unix()%10000)
			}
		}
		e.a.mu.Unlock()
		doc["id"] = id
	}
	if i, _ := e.login.GetCurrentOption(); i == 1 {
		if strings.TrimSpace(e.username.GetText()) == "" {
			return nil, fmt.Errorf("give the user name (user@realm, for example root@pam)")
		}
		doc["username"], doc["password"] = strings.TrimSpace(e.username.GetText()), e.password.GetText()
		doc["authid"], doc["secret"] = "", ""
		if e.isNew && e.password.GetText() == "" {
			return nil, fmt.Errorf("give the password")
		}
	} else {
		if strings.TrimSpace(e.tokenID.GetText()) == "" {
			return nil, fmt.Errorf("give the token ID (user@realm!tokenname)")
		}
		doc["authid"], doc["secret"] = strings.TrimSpace(e.tokenID.GetText()), e.secret.GetText()
		doc["username"], doc["password"] = "", ""
		if e.isNew && e.secret.GetText() == "" {
			return nil, fmt.Errorf("give the token secret")
		}
	}
	doc["encryption_key_file"] = strings.TrimSpace(e.keyFile.GetText())
	if p := e.keyPass.GetText(); p != "" {
		doc["encryption_key_passphrase"] = p
	}
	if e.forgetPass.IsChecked() {
		doc["clear_encryption_key_passphrase"] = true
	}
	// what the service sends as markers, never to be sent back
	delete(doc, "encryption_key_passphrase_set")
	delete(doc, "secret_set")
	delete(doc, "password_set")
	delete(doc, "is_online")
	return doc, nil
}

func (e *serverEditor) save() {
	doc, err := e.document()
	if err != nil {
		e.a.errorBox(err.Error())
		return
	}
	go func() {
		err := e.a.be.SaveServer(doc, e.isNew)
		e.a.app.QueueUpdateDraw(func() {
			if err != nil {
				e.a.errorBox(describeErr(err))
				return
			}
			e.close()
			e.a.setFlash("saved " + str(doc, "name"))
			go e.a.refresh(true)
		})
	}()
}
