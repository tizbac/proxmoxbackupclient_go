package api

import (
	"os"
	"path/filepath"
	"testing"
)

// NewToken replaces whatever token file is already there (one planted before
// the service ran, or one copied earlier), and EnsureToken then reads it back.
func TestNewTokenReplacesExisting(t *testing.T) {
	p := filepath.Join(t.TempDir(), "api-token")
	if err := os.WriteFile(p, []byte("planted"), 0o600); err != nil {
		t.Fatal(err)
	}
	tok, err := NewToken(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(tok) != 64 || tok == "planted" {
		t.Fatalf("token %q", tok)
	}
	got, err := EnsureToken(p)
	if err != nil || got != tok {
		t.Fatalf("EnsureToken = %q, %v; want %q", got, err, tok)
	}
	again, _ := NewToken(p)
	if again == tok {
		t.Fatal("second NewToken returned the same token")
	}
	if _, err := os.Stat(p + ".tmp"); !os.IsNotExist(err) {
		t.Fatalf("temp file left behind: %v", err)
	}
}
