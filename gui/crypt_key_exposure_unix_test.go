//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"testing"
)

// A key file others can read is flagged, and restricting it makes it the
// owner's only.
func TestKeyFileOpenToOthersUnix(t *testing.T) {
	p := filepath.Join(t.TempDir(), "key.json")
	if err := os.WriteFile(p, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(p, 0o644); err != nil { // past the umask
		t.Fatal(err)
	}
	if open, err := keyFileOpenToOthers(p); err != nil || !open {
		t.Fatalf("0644 key: open=%v err=%v, want open", open, err)
	}
	if err := restrictKeyAccess(p, false); err != nil {
		t.Fatal(err)
	}
	st, _ := os.Stat(p)
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("mode after restricting = %o, want 600", st.Mode().Perm())
	}
	if open, _ := keyFileOpenToOthers(p); open {
		t.Fatal("still open to others after restricting")
	}
}
