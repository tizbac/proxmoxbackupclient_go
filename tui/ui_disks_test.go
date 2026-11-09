package tui

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/salvanza/salvanza_client/gui/api"
)

func demoBackendOf(t *testing.T, h *harness) *demoBackend {
	t.Helper()
	d, ok := h.a.be.(*demoBackend)
	if !ok {
		t.Fatal("the harness does not run on the demo backend")
	}
	return d
}

func TestVMBackupIDsRoundTrip(t *testing.T) {
	for _, c := range []struct {
		n     string
		style int
		id    string
	}{{"123", 0, "9000123"}, {"7", 1, "0007"}, {" 4 5 ", 0, "9000045"}} {
		if got := vmBackupID(c.n, c.style); got != c.id {
			t.Errorf("vmBackupID(%q,%d) = %q, want %q", c.n, c.style, got, c.id)
		}
	}
	if vmBackupID("0", 0) != "" || vmBackupID("", 0) != "" || vmBackupID("1000000", 0) != "" {
		t.Error("an out of range number must give no ID")
	}
	if n, s := vmFromBackupID("9000123"); n != "123" || s != 0 {
		t.Errorf("range style: %q %d", n, s)
	}
	if n, s := vmFromBackupID("0007"); n != "7" || s != 1 {
		t.Errorf("zeros style: %q %d", n, s)
	}
	if n, _ := vmFromBackupID("my-laptop"); n != "" {
		t.Errorf("a name is not a vm ID: %q", n)
	}
}

func TestMachineSelectionRoundTripsThroughTheSetDocument(t *testing.T) {
	doc := map[string]interface{}{}
	in := machineSel{paths: []string{`\\.\PhysicalDrive1`}, ids: []string{"serial:X1"}}
	in.skips = append(in.skips, api.PartitionSkip{DiskID: "serial:X1", Device: `\\.\PhysicalDrive1`, Start: 1048576, Size: 4096, Number: 2, Label: "Partition 2"})
	in.apply(doc)
	out := selOf(doc)
	if len(out.paths) != 1 || out.ids[0] != "serial:X1" || len(out.skips) != 1 || out.skips[0].Start != 1048576 || out.skips[0].Number != 2 {
		t.Errorf("did not round trip: %+v", out)
	}
}

func TestNewDiskImageSetChoosesDisksAndLeavesAPartitionOut(t *testing.T) {
	h := newHarness(t, 130, 44)
	h.typ('2')
	h.waitFor("Backup Sets")
	h.typ('n')
	h.waitFor("What should the new")
	h.key(tcell.KeyTab, 0) // Disk image
	h.key(tcell.KeyEnter, 0)
	h.waitFor("Choose disks")
	h.waitFor("no disk chosen")
	h.typeText("Whole machine")
	h.key(tcell.KeyCtrlS, 0)
	h.waitFor("choose at least one disk")
	h.key(tcell.KeyEnter, 0) // OK

	h.key(tcell.KeyCtrlD, 0)
	h.waitFor("Samsung")
	h.waitFor("WDC")
	h.typ(' ') // tick disk 0
	h.waitFor("(*)")
	h.key(tcell.KeyDown, 0) // EFI
	h.key(tcell.KeyDown, 0) // Windows
	h.key(tcell.KeyDown, 0) // recovery
	h.typ(' ')
	h.waitFor("LEFT OUT")
	h.key(tcell.KeyEnter, 0)
	h.waitFor("1 partition")

	// the picker again: what was chosen is still ticked
	h.key(tcell.KeyCtrlD, 0)
	h.waitFor("1 disk(s) chosen")
	h.key(tcell.KeyEscape, 0)
	h.waitFor("1 partition")
	h.key(tcell.KeyCtrlS, 0)
	h.waitFor("saved Whole machine")

	var saved Set
	for _, s := range demoBackendOf(t, h).sets {
		if s.Name == "Whole machine" {
			saved = s
		}
	}
	if saved.Type != "machine" {
		t.Fatalf("not saved as a machine set: %+v", saved)
	}
	sel := selOf(saved.Raw)
	if len(sel.paths) != 1 || sel.paths[0] != `\\.\PhysicalDrive0` || sel.ids[0] != "serial:S64DNX0T" {
		t.Errorf("disks not saved: %+v", sel)
	}
	if len(sel.skips) != 1 || sel.skips[0].Number != 3 || sel.skips[0].DiskID != "serial:S64DNX0T" {
		t.Errorf("the left-out partition is not saved: %+v", sel.skips)
	}
	if saved.Raw["backupType"] != "machine" {
		t.Errorf("backupType %v", saved.Raw["backupType"])
	}
}

func TestEditingAMachineSetShowsItsDisks(t *testing.T) {
	h := newHarness(t, 130, 44)
	h.typ('2')
	h.waitFor("Machine image")
	h.key(tcell.KeyDown, 0)
	h.key(tcell.KeyEnter, 0)
	h.waitFor("Edit Backup Set")
	h.waitFor("PhysicalDrive0")
	h.key(tcell.KeyCtrlD, 0)
	h.waitFor("1 disk(s) chosen") // the set's disk is ticked
	h.key(tcell.KeyEscape, 0)
	h.waitFor("Edit Backup Set")
	h.key(tcell.KeyPgDn, 0)
	h.key(tcell.KeyPgDn, 0)
	h.waitFor("forensic")
	h.waitFor("Proxmox VM")
}

func TestOneOffDiskBackup(t *testing.T) {
	h := newHarness(t, 130, 44)
	h.typ('b')
	h.waitFor("What should be backed up")
	h.key(tcell.KeyTab, 0)
	h.key(tcell.KeyEnter, 0)
	h.waitFor("Back up disks now")
	h.key(tcell.KeyCtrlS, 0)
	h.waitFor("Choose at least one disk")
	h.key(tcell.KeyEnter, 0)

	h.key(tcell.KeyCtrlD, 0)
	h.waitFor("Samsung")
	if strings.Contains(h.screen(), "Partition 1") {
		t.Errorf("a one-off backup has no partitions to leave out:\n%s", h.screen())
	}
	h.key(tcell.KeyDown, 0) // WDC
	h.typ(' ')
	h.key(tcell.KeyEnter, 0)
	h.waitFor("PhysicalDrive1")
	h.key(tcell.KeyCtrlS, 0)
	h.waitFor("started the one-off disk backup")
}
