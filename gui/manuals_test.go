package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// The manuals built into the app are copies of the repo's docs/ files (see
// manuals.go). This fails when one was edited without the other.
func TestEmbeddedManualsMatchDocs(t *testing.T) {
	for id, m := range manuals {
		embedded, err := manualFS.ReadFile(m.path)
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		repo, err := os.ReadFile(filepath.Join("..", filepath.FromSlash(m.path)))
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		// A Windows checkout may give either copy CRLF line endings.
		norm := func(b []byte) []byte { return bytes.ReplaceAll(b, []byte("\r\n"), []byte("\n")) }
		if !bytes.Equal(norm(embedded), norm(repo)) {
			t.Errorf("gui/%s differs from %s: copy docs/ over gui/docs (make manuals) and rebuild", m.path, m.path)
		}
	}
}

func TestGetManual(t *testing.T) {
	a := &App{}
	for id := range manuals {
		s, err := a.GetManual(id)
		if err != nil || len(s) < 1000 {
			t.Errorf("GetManual(%q): %d bytes, err %v", id, len(s), err)
		}
	}
	if _, err := a.GetManual("nope"); err == nil {
		t.Error("an unknown manual must be an error")
	}
}
