package tui

import (
	"strings"
	"testing"
)

// A connection test that passes but warns (the account can restore but not
// back up) says so next to the result.
func TestServerTestShowsTheWarning(t *testing.T) {
	h := newHarness(t, 130, 44)
	demoBackendOf(t, h).testNote = "restore only: this account has Datastore.Read but not Datastore.Backup on /datastore/store"
	h.typ('5')
	h.waitFor("Main PBS")
	h.typ('t')
	h.waitFor("connection to Main PBS works, but restore only")
}

func TestServerTestWithoutAWarningStaysPlain(t *testing.T) {
	h := newHarness(t, 130, 44)
	h.typ('5')
	h.waitFor("Main PBS")
	h.typ('t')
	h.waitFor("connection to Main PBS works")
	if got := h.screen(); strings.Contains(got, "works, but") {
		t.Errorf("a warning with nothing to warn of:\n%s", got)
	}
}
