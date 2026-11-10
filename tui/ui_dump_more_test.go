package tui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
)

// TestDumpMoreScreens writes the screens that came after TestDumpScreens (disk
// images, Undelete and Roll back, encryption, share logins, the settings file,
// the server test warning) as text and as an HTML page, when TUI_DUMP names a
// folder.
func TestDumpMoreScreens(t *testing.T) {
	dir := os.Getenv("TUI_DUMP")
	if dir == "" {
		t.Skip("TUI_DUMP not set")
	}
	_ = os.MkdirAll(dir, 0o755)
	h := newHarness(t, 130, 38)
	demoBackendOf(t, h).testNote = "restore only: this account has Datastore.Read but not Datastore.Backup on /datastore/backup, so backups to it are refused"
	shot := func(name string) {
		time.Sleep(300 * time.Millisecond)
		_ = os.WriteFile(filepath.Join(dir, name+".txt"), []byte(h.screen()), 0o644)
		_ = os.WriteFile(filepath.Join(dir, name+".html"), []byte(h.htmlScreen()), 0o644)
	}
	esc := func() { h.key(tcell.KeyEscape, 0) }
	tabs := func(n int) {
		for i := 0; i < n; i++ {
			h.key(tcell.KeyTab, 0)
		}
	}

	h.typ('2')
	h.waitFor("Backup Sets")

	// a new disk image set
	h.typ('n')
	h.waitFor("What should the new")
	shot("22-new-set-type")
	tabs(1)
	h.key(tcell.KeyEnter, 0)
	h.waitFor("no disk chosen")
	shot("23-new-disk-set")
	h.key(tcell.KeyCtrlD, 0)
	h.waitFor("Samsung")
	h.typ(' ')
	h.waitFor("(*)")
	for i := 0; i < 3; i++ {
		h.key(tcell.KeyDown, 0)
	}
	h.typ(' ')
	h.waitFor("LEFT OUT")
	shot("24-disk-picker")
	h.key(tcell.KeyEnter, 0)
	h.waitFor("1 partition")
	shot("25-disk-set-chosen")
	h.key(tcell.KeyPgDn, 0)
	h.key(tcell.KeyPgDn, 0)
	h.waitFor("forensic")
	shot("26-disk-set-options")
	esc()
	h.waitFor("Backup Sets")

	// an existing machine set
	h.key(tcell.KeyDown, 0)
	h.key(tcell.KeyEnter, 0)
	h.waitFor("PhysicalDrive0")
	shot("27-edit-machine-set")
	esc()
	h.waitFor("Backup Sets")

	// one-off disk backup
	h.typ('b')
	h.waitFor("What should be backed up")
	shot("28-one-off-type")
	tabs(1)
	h.key(tcell.KeyEnter, 0)
	h.waitFor("Back up disks now")
	shot("29-one-off-disks")
	h.key(tcell.KeyCtrlD, 0)
	h.waitFor("Samsung")
	shot("30-one-off-disk-picker")
	esc()
	esc()
	h.waitFor("Backup Sets")

	// Undelete
	h.typ('u')
	h.waitFor("Also look in the backups")
	shot("31-undelete")
	tabs(3)
	h.key(tcell.KeyEnter, 0)
	h.waitFor("finished.")
	shot("32-undelete-look")
	esc()
	h.waitFor("Backup Sets")

	// Roll back, its preview and the earlier roll backs
	h.typ('k')
	h.waitFor("Back to the backup at or before")
	shot("33-rollback")
	tabs(5)
	h.key(tcell.KeyEnter, 0)
	h.waitFor("Press y to go ahead")
	shot("34-rollback-preview")
	esc()
	h.waitFor("Backup Sets")
	h.typ('k')
	h.waitFor("Back to the backup at or before")
	tabs(6)
	h.key(tcell.KeyEnter, 0)
	h.waitFor("Earlier roll backs of")
	h.waitFor("replace")
	shot("35-earlier-rollbacks")
	esc()
	h.waitFor("Backup Sets")

	// servers: the test warning and the encryption step
	h.typ('5')
	h.waitFor("Main PBS")
	h.typ('t')
	h.waitFor("works, but restore only")
	shot("36-server-test-warning")
	h.key(tcell.KeyEnter, 0)
	h.waitFor("Edit server")
	h.key(tcell.KeyPgDn, 0)
	h.key(tcell.KeyPgDn, 0)
	h.waitFor("Generate key")
	shot("37-server-encryption")
	esc()
	h.waitFor("Main PBS")

	// preferences: the pages after the first two
	h.sim.InjectKey(tcell.KeyF9, 0, tcell.ModNone)
	h.waitFor("Preferences")
	h.key(tcell.KeyPgDn, 0)
	h.key(tcell.KeyPgDn, 0)
	h.waitFor("Saved logins")
	shot("38-prefs-share-logins")
	h.key(tcell.KeyPgDn, 0)
	h.waitFor("Export")
	shot("39-prefs-settings-file")
}
