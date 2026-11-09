package tui

import (
	"os"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
)

// TestMain lets the test program stand in for salvanza-svc (SALVANZA_FAKE_SVC),
// so the real process handling of runTool is tested without the service.
func TestMain(m *testing.M) {
	if os.Getenv("SALVANZA_FAKE_SVC") == "1" {
		_, _ = os.Stdout.WriteString("first line\r\n\r\n 50%  halfway\n")
		_, _ = os.Stderr.WriteString("args: " + strings.Join(os.Args[1:], " ") + "\n")
		if os.Getenv("SALVANZA_FAKE_FAIL") == "1" {
			_, _ = os.Stderr.WriteString("Roll back: the safety folder is not writable\n")
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestRunToolStreamsBothOutputsAndReportsFailure(t *testing.T) {
	t.Setenv("SALVANZA_FAKE_SVC", "1")
	var got []string
	err := runTool(os.Args[0], []string{"-undelete", "Docs set", "-dry-run"}, func(s string) { got = append(got, s) })
	if err != nil {
		t.Fatal(err)
	}
	all := strings.Join(got, "|")
	for _, want := range []string{"first line", "50%  halfway", "args: -undelete Docs set -dry-run"} {
		if !strings.Contains(all, want) {
			t.Errorf("output lacks %q: %q", want, all)
		}
	}
	if strings.Contains(all, "\r") {
		t.Errorf("a carriage return was kept: %q", all)
	}

	t.Setenv("SALVANZA_FAKE_FAIL", "1")
	err = runTool(os.Args[0], []string{"-rollback", "x"}, func(string) {})
	if err == nil || !strings.Contains(err.Error(), "safety folder is not writable") {
		t.Errorf("a failure should carry the program's last line: %v", err)
	}
}

func TestFindServiceProgramHonoursTheVariable(t *testing.T) {
	t.Setenv("SALVANZA_SVC", os.Args[0])
	p, err := findServiceProgram()
	if err != nil || p != os.Args[0] {
		t.Errorf("got %q %v", p, err)
	}
}

func TestUndeleteFromTheBackupSets(t *testing.T) {
	h := newHarness(t, 130, 44)
	h.typ('2')
	h.waitFor("Backup Sets")
	h.typ('u')
	h.waitFor("Also look in the backups")
	// Look: the dry run lists the files and does not restore
	h.key(tcell.KeyTab, 0)
	h.key(tcell.KeyTab, 0)
	h.key(tcell.KeyTab, 0)
	h.key(tcell.KeyEnter, 0)
	h.waitFor("budget-2026.xlsx")
	h.waitFor("finished.")
	if strings.Contains(h.screen(), "Restored 2 files") {
		t.Errorf("the look must not restore:\n%s", h.screen())
	}
	h.key(tcell.KeyEscape, 0)
	h.waitFor("Backup Sets")

	// Restore
	h.typ('u')
	h.waitFor("Also look in the backups")
	for i := 0; i < 4; i++ {
		h.key(tcell.KeyTab, 0)
	}
	h.key(tcell.KeyEnter, 0)
	h.waitFor("Restored 2 files")
	h.key(tcell.KeyEscape, 0)
}

func TestRollBackPreviewsThenWaitsForY(t *testing.T) {
	h := newHarness(t, 130, 44)
	h.typ('2')
	h.waitFor("Backup Sets")
	h.typ('k')
	h.waitFor("Back to the backup at or before")
	// fields: at, policy, back up first, verify; buttons Preview, Roll back
	for i := 0; i < 5; i++ {
		h.key(tcell.KeyTab, 0)
	}
	h.key(tcell.KeyEnter, 0) // Roll back
	h.waitFor("changed since: 5 files")
	h.waitFor("Press y to go ahead")
	if strings.Contains(h.screen(), "Rolled back 7 files") {
		t.Fatalf("it must wait for y before it changes anything:\n%s", h.screen())
	}
	h.typ('y')
	h.waitFor("Rolled back 7 files")
	h.waitFor("finished.")
	h.key(tcell.KeyEscape, 0)
	h.waitFor("Backup Sets")

	// Esc at the question cancels
	h.typ('k')
	h.waitFor("Back to the backup at or before")
	for i := 0; i < 5; i++ {
		h.key(tcell.KeyTab, 0)
	}
	h.key(tcell.KeyEnter, 0)
	h.waitFor("Press y to go ahead")
	h.key(tcell.KeyEscape, 0)
	h.waitFor("Backup Sets")
}
