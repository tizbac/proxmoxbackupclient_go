package main

import (
	"strings"
	"testing"
)

func TestSkippedSummary(t *testing.T) {
	if s := skippedSummary(0, 0); s != "" {
		t.Errorf("nothing skipped: got %q", s)
	}
	// Five sockets in a home folder: a note, no warning sign.
	s := skippedSummary(5, 0)
	if strings.Contains(s, "⚠️") || !strings.Contains(s, "5 items left out by design") || strings.Contains(s, "could not be read") {
		t.Errorf("by design only: got %q", s)
	}
	// Two unreadable among five: a warning for the two, a note for the three.
	s = skippedSummary(5, 2)
	if !strings.Contains(s, "⚠️  2 files/folders could not be read") || !strings.Contains(s, "3 items left out by design") {
		t.Errorf("mixed: got %q", s)
	}
	s = skippedSummary(2, 2)
	if strings.Contains(s, "by design") {
		t.Errorf("all unreadable: got %q", s)
	}
}
