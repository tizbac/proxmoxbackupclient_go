package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"pbscommon"
)

// writeTestKey makes a real key file without a passphrase, as the Proxmox
// client's `key create --kdf none` does.
func writeTestKey(t *testing.T, dir, name string) (path, fingerprint string) {
	t.Helper()
	enc, err := pbscommon.GenerateEncryptionKey()
	if err != nil {
		t.Fatal(err)
	}
	crypt, err := pbscommon.NewCryptConfig(enc)
	if err != nil {
		t.Fatal(err)
	}
	path = filepath.Join(dir, name)
	if err := pbscommon.SaveKeyFile(path, crypt); err != nil {
		t.Fatal(err)
	}
	return path, crypt.Fingerprint()
}

func TestCheckKeyFile(t *testing.T) {
	dir := t.TempDir()
	good, fp := writeTestKey(t, dir, "good.json")
	data, cfg, checked, err := checkKeyFile(good, "")
	if err != nil || !checked || cfg.Fingerprint != fp || len(data) == 0 {
		t.Fatalf("a good key: %v checked=%v %+v", err, checked, cfg)
	}
	// not a key
	junk := filepath.Join(dir, "junk.json")
	_ = os.WriteFile(junk, []byte(`{"hello":"world"}`), 0o600)
	if _, _, _, err := checkKeyFile(junk, ""); err == nil || !strings.Contains(err.Error(), "not a Proxmox Backup Server key file") {
		t.Errorf("junk: %v", err)
	}
	// damaged: the key data no longer matches the fingerprint
	raw, _ := os.ReadFile(good)
	bad := filepath.Join(dir, "bad.json")
	damaged := strings.Replace(string(raw), `"data": "`, `"data": "AAAA`, 1)
	_ = os.WriteFile(bad, []byte(damaged), 0o600)
	if _, _, _, err := checkKeyFile(bad, ""); err == nil {
		t.Errorf("a damaged key was accepted")
	}
	if _, _, _, err := checkKeyFile(filepath.Join(dir, "missing.json"), ""); err == nil {
		t.Errorf("a missing file was accepted")
	}
	if _, _, _, err := checkKeyFile("", ""); err == nil {
		t.Errorf("an empty path was accepted")
	}
}

func TestUniqueKeyName(t *testing.T) {
	a := uniqueKeyName("main-pbs-key")
	if !strings.HasPrefix(a, "main-pbs-key-") || !strings.HasSuffix(a, ".json") {
		t.Errorf("name %q", a)
	}
	if b := uniqueKeyName("key.json"); !strings.HasPrefix(b, "key-") {
		t.Errorf("the extension is not doubled: %q", b)
	}
	if c := uniqueKeyName(""); !strings.HasPrefix(c, "encryption-key-") {
		t.Errorf("empty: %q", c)
	}
}

func TestServerDetailsShowTheKeyState(t *testing.T) {
	h := newHarness(t, 140, 44)
	h.typ('5')
	h.waitFor("Main PBS")
	h.waitFor("Key file")
	h.waitFor("fingerprint ab:12:cd:34")
	s := h.screen()
	for _, want := range []string{"keys/key.json", "the key is protected; stored: yes", "Encrypted"} {
		if !strings.Contains(s, want) {
			t.Errorf("the server details lack %q:\n%s", want, s)
		}
	}
	// the set details name the key of the server the set backs up to
	h.typ('2')
	h.waitFor("Encryption")
	h.waitFor("encrypted, with the key of Main PBS")
}

func TestGenerateAKeyForAServer(t *testing.T) {
	h := newHarness(t, 140, 44)
	h.typ('5')
	h.waitFor("Main PBS")
	h.key(tcell.KeyEnter, 0) // edit Main PBS
	h.waitFor("Edit server")
	h.key(tcell.KeyPgDn, 0)
	h.key(tcell.KeyPgDn, 0)
	h.waitFor("Key file")
	h.waitFor("keys/key.json")
	// Key file, passphrase, forget, then the buttons: Back, Generate key
	for i := 0; i < 4; i++ {
		h.key(tcell.KeyTab, 0)
	}
	h.key(tcell.KeyEnter, 0)
	h.waitFor("This server has a key already")
	h.key(tcell.KeyEnter, 0) // Yes
	h.waitFor("A new key was made")
	h.waitFor("Fingerprint:")
	h.key(tcell.KeyEnter, 0) // OK
	h.waitFor("main-pbs-key-")
	h.key(tcell.KeyCtrlS, 0)
	h.waitFor("saved Main PBS")
	// the details now describe the new key: no passphrase, and a warning that others can read the file
	h.waitFor("the key has none")
	h.waitFor("other accounts can read the key file")
}

func TestImportAKeyFileChecksItAndShowsTheFingerprint(t *testing.T) {
	dir := t.TempDir()
	path, fp := writeTestKey(t, dir, "usb-copy.json")
	h := newHarness(t, 140, 44)
	h.typ('5')
	h.waitFor("Main PBS")
	h.key(tcell.KeyEnter, 0)
	h.waitFor("Edit server")
	h.key(tcell.KeyPgDn, 0)
	h.key(tcell.KeyPgDn, 0)
	h.waitFor("Key file")
	// Back, Generate key, Import key file
	for i := 0; i < 5; i++ {
		h.key(tcell.KeyTab, 0)
	}
	h.key(tcell.KeyEnter, 0)
	h.waitFor("Import a key file")
	h.typeText(path)
	h.key(tcell.KeyTab, 0) // passphrase
	h.key(tcell.KeyTab, 0) // Read
	h.key(tcell.KeyEnter, 0)
	h.waitFor("Import this key?")
	h.waitFor(fp[:17]) // the dialog wraps the long fingerprint
	h.waitFor("The key was checked")
	h.key(tcell.KeyEnter, 0) // Yes
	h.waitFor("The key was imported")
	h.key(tcell.KeyEnter, 0)
	h.waitFor("usb-copy")
}

func TestImportRefusesWhatIsNotAKey(t *testing.T) {
	dir := t.TempDir()
	junk := filepath.Join(dir, "notes.txt")
	_ = os.WriteFile(junk, []byte("just some notes"), 0o600)
	h := newHarness(t, 140, 44)
	h.typ('5')
	h.waitFor("Main PBS")
	h.key(tcell.KeyEnter, 0)
	h.waitFor("Edit server")
	h.key(tcell.KeyPgDn, 0)
	h.key(tcell.KeyPgDn, 0)
	h.waitFor("Key file")
	for i := 0; i < 5; i++ {
		h.key(tcell.KeyTab, 0)
	}
	h.key(tcell.KeyEnter, 0)
	h.waitFor("Import a key file")
	h.typeText(junk)
	h.key(tcell.KeyTab, 0)
	h.key(tcell.KeyTab, 0)
	h.key(tcell.KeyEnter, 0)
	h.waitFor("not a Proxmox Backup Server key file")
	h.key(tcell.KeyEnter, 0)
	// the editor is still open, with the old key
	h.waitFor("Key file")
	h.waitFor("keys/key.json")
}

func TestRemovingTheKeyAndForgettingThePassphrase(t *testing.T) {
	h := newHarness(t, 140, 44)
	h.typ('5')
	h.waitFor("Main PBS")
	h.key(tcell.KeyEnter, 0)
	h.waitFor("Edit server")
	h.key(tcell.KeyPgDn, 0)
	h.key(tcell.KeyPgDn, 0)
	h.waitFor("keys/key.json")
	// clear the key file field: select all by deleting the text
	for i := 0; i < 20; i++ {
		h.key(tcell.KeyBackspace2, 0)
	}
	h.key(tcell.KeyCtrlS, 0)
	h.waitFor("saved Main PBS")
	h.waitFor("Encrypted    no")
}
