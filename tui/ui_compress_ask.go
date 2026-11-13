package tui

import (
	"github.com/rivo/tview"
)

// The one-time question about "Compress encrypted backups", as the app asks
// it: a server whose switch the service started off, because the version
// before it wrote encrypted backups uncompressed, is marked
// (compress_encrypted_ask) until someone has answered. On a computer with no
// desktop this window is where that happens.

const (
	compressAskOn  = "Turn on"
	compressAskOff = "Keep off for now"
)

// compressAskText is the question for one server. The box that shows it is a
// third of the window wide and wraps the text itself, so the paragraphs have
// no line breaks of their own, and it is kept short enough for a window of 80
// by 24.
func compressAskText(server string) string {
	return "Compress encrypted backups?\n\n" +
		"Encrypted backups to " + server + " are not compressed, so version 2.0.0 and earlier can still restore them.\n\n" +
		"Turning it on is recommended and saves space. Backups made after that need version 2.1 or later to restore, or a restore ISO made from it.\n\n" +
		"You can change it later: Servers, edit, Encryption."
}

// compressAnswerDoc is the server document that saves an answer: the server as
// the service sent it, with the switch as answered and the question taken off.
// Secrets are not in what the service sends, and an empty one keeps the stored.
func compressAnswerDoc(raw map[string]interface{}, on bool) map[string]interface{} {
	doc := copyMap(raw)
	doc["compress_encrypted"] = on
	doc["compress_encrypted_ask"] = false
	delete(doc, "encryption_key_passphrase_set")
	delete(doc, "secret_set")
	delete(doc, "password_set")
	delete(doc, "is_online")
	return doc
}

// nextCompressAsk is the first server still to be asked about in this run, or
// false. asked is added to, so each server is asked once a run even if saving
// the answer fails (it is asked again the next time the window is opened).
func nextCompressAsk(servers []Server, asked map[string]bool) (Server, bool) {
	for _, s := range servers {
		if s.KeyFile == "" || !boolean(s.Raw, "compress_encrypted_ask") || asked[s.ID] {
			continue
		}
		asked[s.ID] = true
		return s, true
	}
	return Server{}, false
}

// askCompressEncrypted shows the question for the next server that has one,
// unless a form or another question is open (the next full refresh tries
// again). UI goroutine only.
func (a *App) askCompressEncrypted() {
	if a.modal {
		return
	}
	a.mu.Lock()
	servers := append([]Server(nil), a.servers...)
	a.mu.Unlock()
	if a.compressAsked == nil {
		a.compressAsked = map[string]bool{}
	}
	s, ok := nextCompressAsk(servers, a.compressAsked)
	if !ok {
		return
	}
	name := s.Name
	if name == "" {
		name = s.ID
	}
	// "Keep off for now" comes first, so it has the focus: Enter never does
	// the step older versions cannot follow. Escape answers the same.
	m := tview.NewModal().SetText(compressAskText(name)).AddButtons([]string{compressAskOff, compressAskOn})
	m.SetBackgroundColor(colPanel).SetTextColor(colText)
	m.SetButtonBackgroundColor(colAccent).SetButtonStyle(onColour(colAccent))
	m.SetDoneFunc(func(i int, label string) {
		a.pages.RemovePage("modal")
		a.modal = false
		a.switchTab(a.tab)
		on := label == compressAskOn
		go func() {
			err := a.be.SaveServer(compressAnswerDoc(s.Raw, on), false)
			a.app.QueueUpdateDraw(func() {
				switch {
				case err != nil:
					a.errorBox(describeErr(err))
				case on:
					a.setFlash("Compress encrypted backups is on for " + name)
				default:
					a.setFlash("Compress encrypted backups stays off for " + name)
				}
			})
			a.refresh(true)
		}()
	})
	a.modal = true
	a.pages.AddPage("modal", m, true, true)
	a.app.SetFocus(m)
}
