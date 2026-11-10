package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
)

// The buttons of the server form's widest step fit a 120 column window (the
// default Command Prompt): Cancel was cut off at the edge.
func TestServerFormButtonsFitA120ColumnWindow(t *testing.T) {
	h := newHarness(t, 120, 40)
	h.typ('5')
	h.waitFor("Main PBS")
	h.key(tcell.KeyEnter, 0)
	h.waitFor("Edit server")
	h.key(tcell.KeyPgDn, 0)
	h.key(tcell.KeyPgDn, 0)
	h.waitFor("Generate key")
	screen := h.screen()
	for _, b := range []string{"Back", "Generate key", "Import key file", "Inspect", "Restrict access", "Save", "Cancel"} {
		if !strings.Contains(screen, b) {
			t.Errorf("the %q button is cut off:\n%s", b, screen)
		}
	}
	if strings.Contains(screen, "Cance ") || strings.Contains(screen, "Cance\n") {
		t.Errorf("Cancel is cut off:\n%s", screen)
	}
}

// The title is the product's name; a narrow window keeps just Salvanza so the
// status beside it still fits.
func TestHeaderShowsTheProductName(t *testing.T) {
	wide := newHarness(t, 130, 40)
	wide.waitFor("Salvanza for Proxmox Backup Server")
	if strings.Contains(wide.screen(), "Salvanza Backup") {
		t.Errorf("the old title is still there:\n%s", wide.screen())
	}
	// shrinking the window changes the title at once, not at the next 2 second refresh
	wide.sim.SetSize(90, 30)
	wide.a.app.QueueUpdateDraw(func() {}) // what a real terminal's resize event makes the window do
	deadline := time.Now().Add(1500 * time.Millisecond)
	for strings.Contains(strings.SplitN(wide.screen(), "\n", 2)[0], "Proxmox") {
		if time.Now().After(deadline) {
			t.Fatalf("the title did not follow the window to 90 columns:\n%s", wide.screen())
		}
		time.Sleep(50 * time.Millisecond)
	}
	narrow := newHarness(t, 90, 30)
	narrow.waitFor("service running")
	first := strings.SplitN(narrow.screen(), "\n", 2)[0]
	if !strings.Contains(first, "Salvanza") || strings.Contains(first, "Proxmox") || !strings.Contains(first, "service running") {
		t.Errorf("the title of a narrow window: %q", first)
	}
}

// The panels of the forms are not numbered: the step bar above them names the steps.
func TestFormPanelsAreNotNumbered(t *testing.T) {
	h := newHarness(t, 130, 40)
	h.typ('5')
	h.waitFor("Main PBS")
	h.key(tcell.KeyEnter, 0)
	h.waitFor("Edit server")
	for _, bad := range []string{"1  Server", "2  Login", "3  Encryption"} {
		if strings.Contains(h.screen(), bad) {
			t.Errorf("the server form's panel still says %q", bad)
		}
	}
	h.key(tcell.KeyEscape, 0)
	h.typ('2')
	h.waitFor("Backup Sets")
	h.typ('n')
	h.waitFor("What should the new")
	h.key(tcell.KeyEnter, 0)
	h.waitFor("New Backup Set")
	for _, bad := range []string{"1  General", "2  Schedule", "3  Options"} {
		if strings.Contains(h.screen(), bad) {
			t.Errorf("the Backup Set form's panel still says %q", bad)
		}
	}
}
