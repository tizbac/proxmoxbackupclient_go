package tui

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// Only an explicit false is off: a server that never had the switch set
// (every server before 2.1, and every new one) compresses.
func TestCompressEncryptedOffReadsOnlyAnExplicitFalse(t *testing.T) {
	for name, c := range map[string]struct {
		doc  map[string]interface{}
		want bool
	}{
		"unset":         {map[string]interface{}{"name": "a"}, false},
		"true":          {map[string]interface{}{"compress_encrypted": true}, false},
		"false":         {map[string]interface{}{"compress_encrypted": false}, true},
		"null":          {map[string]interface{}{"compress_encrypted": nil}, false},
		"text":          {map[string]interface{}{"compress_encrypted": "false"}, false},
		"no document":   {nil, false},
		"only the mark": {map[string]interface{}{"compress_encrypted_ask": true}, false},
	} {
		if got := compressEncryptedOff(c.doc); got != c.want {
			t.Errorf("%s: off = %v, want %v", name, got, c.want)
		}
	}
}

func askServer(id, key string, ask bool) Server {
	raw := map[string]interface{}{"id": id, "name": "Server " + id, "encryption_key_file": key}
	if ask {
		raw["compress_encrypted"] = false
		raw["compress_encrypted_ask"] = true
	}
	return Server{ID: id, Name: "Server " + id, KeyFile: key, CompressEncryptedOff: ask, Raw: raw}
}

// Each server with the mark and a key is asked about once a run, in order;
// one without a key is never asked about, whatever its mark says.
func TestNextCompressAskTakesEachMarkedServerOnce(t *testing.T) {
	noKey := askServer("nokey", "", true)
	servers := []Server{askServer("plain", "keys/a.json", false), noKey, askServer("one", "keys/b.json", true), askServer("two", "keys/c.json", true)}
	asked := map[string]bool{}
	var order []string
	for i := 0; i < 5; i++ {
		s, ok := nextCompressAsk(servers, asked)
		if !ok {
			break
		}
		order = append(order, s.ID)
	}
	if got := strings.Join(order, ","); got != "one,two" {
		t.Errorf("asked about %q, want one,two", got)
	}
	if _, ok := nextCompressAsk(servers, asked); ok {
		t.Error("asked again in the same run")
	}
	if asked["plain"] || asked["nokey"] {
		t.Errorf("a server with nothing to ask was marked as asked: %v", asked)
	}
	if _, ok := nextCompressAsk(nil, map[string]bool{}); ok {
		t.Error("asked with no servers")
	}
}

// The answer is saved as the server came, with the switch as answered and the
// mark off; what the service sends as markers does not go back, and the
// document it sent is left as it was.
func TestCompressAnswerDoc(t *testing.T) {
	raw := map[string]interface{}{
		"id": "main", "name": "Main PBS", "baseurl": "https://pbs.example.com:8007", "datastore": "backup",
		"encryption_key_file": "keys/key.json", "compress_encrypted": false, "compress_encrypted_ask": true,
		"encryption_key_passphrase_set": true, "secret_set": true, "password_set": false, "is_online": true,
	}
	for _, on := range []bool{true, false} {
		doc := compressAnswerDoc(raw, on)
		if v, ok := doc["compress_encrypted"].(bool); !ok || v != on {
			t.Errorf("answer %v: compress_encrypted = %v", on, doc["compress_encrypted"])
		}
		if v, ok := doc["compress_encrypted_ask"].(bool); !ok || v {
			t.Errorf("answer %v: the mark is still there: %v", on, doc["compress_encrypted_ask"])
		}
		for _, marker := range []string{"encryption_key_passphrase_set", "secret_set", "password_set", "is_online"} {
			if _, there := doc[marker]; there {
				t.Errorf("answer %v: %s is sent back", on, marker)
			}
		}
		for _, keep := range []string{"id", "name", "baseurl", "datastore", "encryption_key_file"} {
			if doc[keep] != raw[keep] {
				t.Errorf("answer %v: %s = %v, want %v", on, keep, doc[keep], raw[keep])
			}
		}
		for _, secret := range []string{"secret", "password", "encryption_key_passphrase"} {
			if _, there := doc[secret]; there {
				t.Errorf("answer %v: %s is in the document, so the stored one would be replaced", on, secret)
			}
		}
	}
	if raw["compress_encrypted_ask"] != true || raw["compress_encrypted"] != false || raw["secret_set"] != true {
		t.Errorf("the document the service sent was changed: %v", raw)
	}
}

// markDemoServers sets the demo servers up the way the service sends a server
// it started with the switch off: off, and marked to be asked about.
func markDemoServers(t *testing.T, h *harness, ids ...string) *demoBackend {
	t.Helper()
	d := demoBackendOf(t, h)
	if _, err := d.Servers(); err != nil {
		t.Fatal(err)
	}
	d.mu.Lock()
	for i := range d.servers {
		for _, id := range ids {
			if d.servers[i].ID != id {
				continue
			}
			raw := copyMap(d.servers[i].Raw)
			raw["compress_encrypted"] = false
			raw["compress_encrypted_ask"] = true
			d.servers[i].Raw = raw
			d.servers[i].CompressEncryptedOff = true
		}
	}
	d.mu.Unlock()
	return d
}

func demoServerRaw(t *testing.T, d *demoBackend, id string) map[string]interface{} {
	t.Helper()
	servers, err := d.Servers()
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range servers {
		if s.ID == id {
			return s.Raw
		}
	}
	t.Fatalf("no demo server %q", id)
	return nil
}

// Enter on the question keeps the switch off: the button with the focus is
// never the step older versions cannot follow. The answer is saved, so the
// question does not come back.
func TestCompressQuestionEnterKeepsItOff(t *testing.T) {
	h := newHarness(t, 140, 44)
	d := markDemoServers(t, h, "main")
	go h.a.refresh(true)
	h.waitFor("Compress encrypted backups?")
	screen := h.screen()
	// single words and the buttons: the box wraps the text to a third of the
	// window, so a phrase may be on two lines
	for _, want := range []string{"Main PBS", "compressed", "2.0.0", "recommended", "Keep off for now", "Turn on", "2.1", "ISO"} {
		if !strings.Contains(screen, want) {
			t.Errorf("the question lacks %q:\n%s", want, screen)
		}
	}
	// and the phrases, in the text the box is given
	text := compressAskText("Main PBS")
	for _, want := range []string{"Encrypted backups to Main PBS are not compressed", "version 2.0.0 and earlier can still restore", "need version 2.1 or later to restore", "restore ISO"} {
		if !strings.Contains(text, want) {
			t.Errorf("the question's text lacks %q: %q", want, text)
		}
	}
	h.key(tcell.KeyEnter, 0)
	h.waitFor("Compress encrypted backups stays off for Main PBS")
	raw := demoServerRaw(t, d, "main")
	if !compressEncryptedOff(raw) || boolean(raw, "compress_encrypted_ask") {
		t.Errorf("after Keep off: %v", raw)
	}
	// a full refresh later, nothing is asked again
	h.a.refresh(true)
	if strings.Contains(h.screen(), "Compress encrypted backups?") {
		t.Errorf("the question came back:\n%s", h.screen())
	}
	h.typ('5')
	h.waitFor("no (2.0.0 and earlier can restore)")
}

func TestCompressQuestionTurnOn(t *testing.T) {
	h := newHarness(t, 140, 44)
	d := markDemoServers(t, h, "main")
	go h.a.refresh(true)
	h.waitFor("Compress encrypted backups?")
	h.key(tcell.KeyTab, 0) // Turn on
	h.key(tcell.KeyEnter, 0)
	h.waitFor("Compress encrypted backups is on for Main PBS")
	raw := demoServerRaw(t, d, "main")
	if compressEncryptedOff(raw) || boolean(raw, "compress_encrypted_ask") {
		t.Errorf("after Turn on: %v", raw)
	}
	if v, ok := raw["compress_encrypted"].(bool); !ok || !v {
		t.Errorf("Turn on did not say so to the service: %v", raw["compress_encrypted"])
	}
	h.typ('5')
	h.waitFor("yes (restoring needs 2.1 or later)")
}

// Escape answers as "Keep off for now" does.
func TestCompressQuestionEscapeKeepsItOff(t *testing.T) {
	h := newHarness(t, 140, 44)
	d := markDemoServers(t, h, "main")
	go h.a.refresh(true)
	h.waitFor("Compress encrypted backups?")
	h.key(tcell.KeyEscape, 0)
	h.waitFor("Compress encrypted backups stays off for Main PBS")
	raw := demoServerRaw(t, d, "main")
	if !compressEncryptedOff(raw) || boolean(raw, "compress_encrypted_ask") {
		t.Errorf("after Escape: %v", raw)
	}
}

// Two servers to ask about are asked about one after the other.
func TestCompressQuestionAsksAboutEachServer(t *testing.T) {
	h := newHarness(t, 140, 44)
	d := markDemoServers(t, h, "main", "offsite")
	go h.a.refresh(true)
	h.waitFor("Encrypted backups to Main PBS")
	h.key(tcell.KeyEnter, 0)
	h.waitFor("Encrypted backups to Offsite PBS")
	h.key(tcell.KeyTab, 0)
	h.key(tcell.KeyEnter, 0)
	h.waitFor("Compress encrypted backups is on for Offsite PBS")
	if raw := demoServerRaw(t, d, "main"); !compressEncryptedOff(raw) || boolean(raw, "compress_encrypted_ask") {
		t.Errorf("Main PBS after Keep off: %v", raw)
	}
	if raw := demoServerRaw(t, d, "offsite"); compressEncryptedOff(raw) || boolean(raw, "compress_encrypted_ask") {
		t.Errorf("Offsite PBS after Turn on: %v", raw)
	}
}

// The question and both its buttons fit the smallest usual window, 80 by 24.
func TestCompressQuestionFitsASmallWindow(t *testing.T) {
	h := newHarness(t, 80, 24)
	markDemoServers(t, h, "offsite")
	go h.a.refresh(true)
	h.waitFor("Compress encrypted backups?")
	screen := h.screen()
	for _, want := range []string{"Offsite PBS", "Keep off for now", "Turn on", "Encryption."} {
		if !strings.Contains(screen, want) {
			t.Errorf("the question in an 80 by 24 window lacks %q:\n%s", want, screen)
		}
	}
}

// With nothing marked, nothing is asked: the window opens as it always did.
func TestNoCompressQuestionWithoutTheMark(t *testing.T) {
	h := newHarness(t, 140, 44)
	h.a.refresh(true)
	if strings.Contains(h.screen(), "Compress encrypted backups?") {
		t.Errorf("a question with no server marked:\n%s", h.screen())
	}
	h.typ('5')
	h.waitFor("yes (restoring needs 2.1 or later)")
}

// The server form: the box follows the server, unticking it saves off, and
// ticking it again is asked about before it is saved.
func TestServerFormCompressEncryptedBox(t *testing.T) {
	h := newHarness(t, 140, 44)
	d := demoBackendOf(t, h)
	openEncryption := func() {
		h.typ('5')
		h.waitFor("Main PBS")
		h.key(tcell.KeyEnter, 0)
		h.waitFor("Edit server")
		h.key(tcell.KeyPgDn, 0)
		h.key(tcell.KeyPgDn, 0)
		h.waitFor("Compress encrypted backups")
		// Key file, passphrase, forget, then the box
		for i := 0; i < 3; i++ {
			h.key(tcell.KeyTab, 0)
		}
	}

	// saved without touching the box, a server that never had the switch
	// still has none
	openEncryption()
	h.key(tcell.KeyCtrlS, 0)
	h.waitFor("saved Main PBS")
	if _, there := demoServerRaw(t, d, "main")["compress_encrypted"]; there {
		t.Errorf("an untouched box was saved: %v", demoServerRaw(t, d, "main"))
	}

	// unticked: saved as off, with no question (older versions restore it)
	openEncryption()
	h.typ(' ')
	h.key(tcell.KeyCtrlS, 0)
	h.waitFor("saved Main PBS")
	if !compressEncryptedOff(demoServerRaw(t, d, "main")) {
		t.Errorf("unticking the box did not save off: %v", demoServerRaw(t, d, "main"))
	}
	h.waitFor("no (2.0.0 and earlier can restore)")

	// ticked again: asked first, and No leaves the server as it was
	openEncryption()
	h.typ(' ')
	h.key(tcell.KeyCtrlS, 0)
	h.waitFor("Turn on Compress encrypted backups for")
	h.waitFor("2.1 or later")
	h.key(tcell.KeyTab, 0) // No
	h.key(tcell.KeyEnter, 0)
	h.waitFor("Edit server")
	if !compressEncryptedOff(demoServerRaw(t, d, "main")) {
		t.Errorf("No still turned it on: %v", demoServerRaw(t, d, "main"))
	}
	// and Yes saves it on
	h.key(tcell.KeyCtrlS, 0)
	h.waitFor("Turn on Compress encrypted backups for")
	h.key(tcell.KeyEnter, 0) // Yes
	h.waitFor("saved Main PBS")
	if raw := demoServerRaw(t, d, "main"); compressEncryptedOff(raw) {
		t.Errorf("Yes did not turn it on: %v", raw)
	}
	h.waitFor("yes (restoring needs 2.1 or later)")
}
